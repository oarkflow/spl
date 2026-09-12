"use strict";

// A compact Wadler/Prettier-style document printer.
//
// Docs are built from strings plus a handful of node kinds; `printDocToString`
// decides where the soft line breaks actually land based on the print width.

const CONCAT = "concat";
const GROUP = "group";
const FILL = "fill";
const INDENT = "indent";
const LINE = "line";
const BREAK_PARENT = "break-parent";

const MODE_BREAK = 1;
const MODE_FLAT = 2;

function concat(parts) {
  return { type: CONCAT, parts };
}

function group(contents, opts) {
  return { type: GROUP, contents, break: !!(opts && opts.shouldBreak) };
}

function fill(parts) {
  return { type: FILL, parts };
}

function indent(contents) {
  return { type: INDENT, contents };
}

const line = { type: LINE, soft: false, hard: false, literal: false };
const softline = { type: LINE, soft: true, hard: false, literal: false };
const breakParent = { type: BREAK_PARENT };
const hardline = concat([{ type: LINE, soft: false, hard: true, literal: false }, breakParent]);
const literalline = concat([{ type: LINE, soft: false, hard: true, literal: true }, breakParent]);

function join(separator, parts) {
  const out = [];
  parts.forEach((part, i) => {
    if (i > 0) out.push(separator);
    out.push(part);
  });
  return concat(out);
}

// propagateBreaks marks every group that (transitively) contains a hard line or
// an explicit break-parent so the printer never tries to flatten it.
function propagateBreaks(doc) {
  const seen = new Set();
  const walk = node => {
    if (typeof node === "string" || node == null) return false;
    if (seen.has(node)) return node.type === GROUP ? node.break : node.__breaks === true;
    seen.add(node);
    let breaks = false;
    switch (node.type) {
      case CONCAT:
      case FILL:
        for (const part of node.parts) {
          if (walk(part)) breaks = true;
        }
        break;
      case INDENT:
        breaks = walk(node.contents);
        break;
      case GROUP:
        if (walk(node.contents)) node.break = true;
        breaks = node.break;
        break;
      case LINE:
        breaks = !!node.hard;
        break;
      case BREAK_PARENT:
        breaks = true;
        break;
      default:
        break;
    }
    node.__breaks = breaks;
    return breaks;
  };
  walk(doc);
  return doc;
}

function trimTrailingWhitespace(out) {
  let trimmed = 0;
  while (out.length > 0) {
    const last = out[out.length - 1];
    // Empty chunks are common (unused lead/trail slots) and must not stop the scan.
    if (last === "") {
      out.pop();
      continue;
    }
    const stripped = last.replace(/[\t ]+$/, "");
    if (stripped === last) break;
    trimmed += last.length - stripped.length;
    if (stripped.length === 0) out.pop();
    else {
      out[out.length - 1] = stripped;
      break;
    }
  }
  return trimmed;
}

function printDocToString(doc, options) {
  const tab = options.useTabs ? "\t" : " ".repeat(options.tabWidth || 2);
  const width = options.printWidth || 120;
  const tabWidth = options.tabWidth || 2;

  const widthOf = text => {
    // Tabs render as `tabWidth` columns; everything else counts as one column.
    let n = 0;
    for (const ch of text) n += ch === "\t" ? tabWidth : 1;
    return n;
  };

  propagateBreaks(doc);

  const fits = (next, restCommands, remainingWidth, mustBeFlat) => {
    let remaining = remainingWidth;
    const cmds = [next];
    let restIdx = restCommands.length;
    while (remaining >= 0) {
      if (cmds.length === 0) {
        if (restIdx === 0) return true;
        cmds.push(restCommands[--restIdx]);
        continue;
      }
      const [ind, mode, d] = cmds.pop();
      if (typeof d === "string") {
        remaining -= widthOf(d);
        continue;
      }
      if (d == null) continue;
      switch (d.type) {
        case CONCAT:
        case FILL:
          for (let i = d.parts.length - 1; i >= 0; i--) cmds.push([ind, mode, d.parts[i]]);
          break;
        case INDENT:
          cmds.push([ind + tab, mode, d.contents]);
          break;
        case GROUP:
          if (mustBeFlat && d.break) return false;
          cmds.push([ind, d.break ? MODE_BREAK : mode, d.contents]);
          break;
        case LINE:
          if (mode === MODE_BREAK || d.hard) return true;
          if (!d.soft) remaining -= 1;
          break;
        case BREAK_PARENT:
          break;
        default:
          break;
      }
    }
    return false;
  };

  const out = [];
  const baseIndent = options.baseIndent || "";
  let pos = 0;
  let shouldRemeasure = false;
  const cmds = [[baseIndent, MODE_BREAK, doc]];

  while (cmds.length > 0) {
    const [ind, mode, d] = cmds.pop();
    if (typeof d === "string") {
      out.push(d);
      pos += widthOf(d);
      continue;
    }
    if (d == null) continue;
    switch (d.type) {
      case BREAK_PARENT:
        break;
      case CONCAT:
        for (let i = d.parts.length - 1; i >= 0; i--) cmds.push([ind, mode, d.parts[i]]);
        break;
      case INDENT:
        cmds.push([ind + tab, mode, d.contents]);
        break;
      case GROUP: {
        if (mode === MODE_FLAT && !shouldRemeasure) {
          cmds.push([ind, d.break ? MODE_BREAK : MODE_FLAT, d.contents]);
          break;
        }
        shouldRemeasure = false;
        if (d.break) {
          cmds.push([ind, MODE_BREAK, d.contents]);
          break;
        }
        const next = [ind, MODE_FLAT, d.contents];
        cmds.push(fits(next, cmds, width - pos, false) ? next : [ind, MODE_BREAK, d.contents]);
        break;
      }
      case FILL: {
        const parts = d.parts;
        if (parts.length === 0) break;
        const content = parts[0];
        const contentFlatCmd = [ind, MODE_FLAT, content];
        const contentBreakCmd = [ind, MODE_BREAK, content];
        const contentFits = fits(contentFlatCmd, [], width - pos, true);
        if (parts.length === 1) {
          cmds.push(contentFits ? contentFlatCmd : contentBreakCmd);
          break;
        }
        const whitespace = parts[1];
        const whitespaceFlatCmd = [ind, MODE_FLAT, whitespace];
        const whitespaceBreakCmd = [ind, MODE_BREAK, whitespace];
        if (parts.length === 2) {
          if (contentFits) cmds.push(whitespaceFlatCmd, contentFlatCmd);
          else cmds.push(whitespaceBreakCmd, contentBreakCmd);
          break;
        }
        const remainingCmd = [ind, mode, fill(parts.slice(2))];
        const pairCmd = [ind, MODE_FLAT, concat([content, whitespace, parts[2]])];
        if (fits(pairCmd, [], width - pos, true)) {
          cmds.push(remainingCmd, whitespaceFlatCmd, contentFlatCmd);
        } else if (contentFits) {
          cmds.push(remainingCmd, whitespaceBreakCmd, contentFlatCmd);
        } else {
          cmds.push(remainingCmd, whitespaceBreakCmd, contentBreakCmd);
        }
        break;
      }
      case LINE: {
        if (mode === MODE_FLAT && !d.hard) {
          if (!d.soft) {
            out.push(" ");
            pos += 1;
          }
          break;
        }
        if (mode === MODE_FLAT) shouldRemeasure = true;
        if (d.literal) {
          // Verbatim content (<pre>, @raw) keeps its trailing whitespace.
          out.push("\n");
          pos = 0;
        } else {
          pos -= trimTrailingWhitespace(out);
          out.push("\n" + ind);
          pos = widthOf(ind);
        }
        break;
      }
      default:
        break;
    }
  }

  return out.join("");
}

module.exports = {
  concat,
  group,
  fill,
  indent,
  join,
  line,
  softline,
  hardline,
  literalline,
  printDocToString
};
