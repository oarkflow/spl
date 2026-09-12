"use strict";

// Directive argument lists -- `@render("Panel", { "id": x })`, `@component("Card", title)` --
// are the one place in a template where the author writes structured data rather
// than markup. When such a list is written across several lines the printer has
// to lay it out itself, otherwise the argument lines keep whatever column they
// happened to start in while the directive around them moves.
//
// The rules mirror the rest of the formatter: a list is only ever broken across
// lines when the author already wrote it that way, and nothing but whitespace
// is ever changed.

const { concat, group, indent, join, line, softline, hardline, literalline } = require("./doc");

const CLOSERS = { "{": "}", "[": "]", "(": ")" };

// Argument lists holding code rather than data are re-anchored but never
// restructured: splitting them at commas could move a comment, a statement, or
// part of a template literal onto a line where it means something else.
const CODE_LIKE = /[`;]|\/\/|\/\*|=>|\bfunction\b/;

/**
 * Build the doc for a directive head (`@name` plus its argument list).
 *
 * @param {string} keyword   directive name, without the `@`
 * @param {string|null} args collapsed argument text, or null when there are no parens
 * @param {string} raw       the head exactly as the parser collapsed it
 */
function printDirectiveHead(keyword, args, raw) {
  if (args == null) return raw;
  const text = args.trim();
  // Single-line lists are already where the author put them, and reflowing a
  // long condition would break lines the author never asked to break.
  if (!text || !text.includes("\n")) return raw;
  // A template literal carries its own line breaks and indentation as data, so
  // the whole list is emitted exactly as written.
  if (text.includes("`")) return concat([`@${keyword}(`, verbatimDoc(text), ")"]);
  if (CODE_LIKE.test(text)) return printCodeArgs(keyword, text);

  const parts = splitTopLevel(text);
  const trailingComma = parts.length > 1 && parts[parts.length - 1].trim() === "";
  if (trailingComma) parts.pop();

  const hug = hugDoc(keyword, parts, trailingComma);
  if (hug) return hug;

  if (parts.length > 1) {
    return group(
      concat([
        `@${keyword}(`,
        indent(concat([softline, join(concat([",", line]), parts.map(printValue)), trailingComma ? "," : ""])),
        softline,
        ")"
      ]),
      { shouldBreak: true }
    );
  }
  return concat([`@${keyword}(`, reindentValue(text), ")"]);
}

// Code arguments -- an inline `@handler(save = { ... })` body, say -- keep every
// line exactly as written; only the block's anchor moves.
function printCodeArgs(keyword, text) {
  const bracketAt = trailingBracketStart(text);
  if (bracketAt >= 0) {
    const prefix = text.slice(0, bracketAt);
    const open = text[bracketAt];
    const inner = text.slice(bracketAt + 1, -1);
    if (!prefix.includes("\n") && inner.includes("\n")) {
      const body = dedentBlock(inner);
      if (body) {
        return concat([`@${keyword}(`, prefix, open, indent(concat([hardline, body])), hardline, CLOSERS[open], ")"]);
      }
    }
  }
  return concat([`@${keyword}(`, reindentValue(text), ")"]);
}

// `@render("Panel", { ... })` keeps its leading arguments on the directive's own
// line and expands only the trailing object or array, the way the author wrote it.
function hugDoc(keyword, parts, trailingComma) {
  if (trailingComma || !parts.length) return null;
  const last = parts[parts.length - 1].trim();
  const bracketAt = trailingBracketStart(last);
  if (bracketAt < 0) return null;
  const prefix = last.slice(0, bracketAt);
  if (prefix.includes("\n")) return null;
  const lead = parts.slice(0, -1);
  if (lead.some(part => part.includes("\n"))) return null;
  return concat([
    `@${keyword}(`,
    ...lead.map(part => concat([part.trim(), ", "])),
    prefix,
    printBracketed(last.slice(bracketAt)),
    ")"
  ]);
}

// One argument, or one entry of an object or array literal.
function printValue(text) {
  const value = text.trim();
  if (!value) return "";
  const bracketAt = trailingBracketStart(value);
  if (bracketAt >= 0) {
    const prefix = value.slice(0, bracketAt);
    if (!prefix.includes("\n")) return concat([prefix, printBracketed(value.slice(bracketAt))]);
  }
  return value.includes("\n") ? reindentValue(value) : value;
}

// A balanced `{ ... }`, `[ ... ]`, or `( ... )` group, expanded one entry per
// line when the author wrote it across lines.
function printBracketed(text) {
  const open = text[0];
  const close = CLOSERS[open];
  if (!close || text[text.length - 1] !== close) return reindentValue(text);

  const inner = text.slice(1, -1);
  if (!inner.trim()) return open + close;

  const parts = splitTopLevel(inner);
  const trailingComma = parts.length > 1 && parts[parts.length - 1].trim() === "";
  if (trailingComma) parts.pop();

  // Objects read as `{ a: 1 }` when flat; arrays and parens as `[1, 2]`.
  const pad = open === "{" ? line : softline;
  return group(
    concat([
      open,
      indent(concat([pad, join(concat([",", line]), parts.map(printValue)), trailingComma ? "," : ""])),
      pad,
      close
    ]),
    { shouldBreak: text.includes("\n") }
  );
}

// Keeps a chunk's own line structure but re-anchors it one level in from the
// directive, so continuation lines follow the directive when it moves.
function reindentValue(text) {
  const lines = text.replace(/\r\n/g, "\n").split("\n").map(l => l.replace(/\s+$/, ""));
  if (lines.length <= 1) return lines[0] || "";
  const measured = lines.slice(1).filter(l => l.trim() !== "");
  const min = measured.length ? Math.min(...measured.map(l => /^[ \t]*/.exec(l)[0].length)) : 0;
  const parts = [lines[0]];
  for (const l of lines.slice(1)) {
    parts.push(hardline);
    parts.push(l.trim() === "" ? "" : l.slice(min));
  }
  return indent(concat(parts));
}

// Emits every line at the exact column it was written in.
function verbatimDoc(text) {
  const lines = text.replace(/\r\n/g, "\n").split("\n");
  const parts = [];
  lines.forEach((l, i) => {
    if (i > 0) parts.push(literalline);
    parts.push(l);
  });
  return concat(parts);
}

// Drops the common indentation from a block of lines, keeping their relative
// shape, so the caller can re-anchor it with indent().
function dedentBlock(text) {
  const lines = text.replace(/\r\n/g, "\n").split("\n").map(l => l.replace(/\s+$/, ""));
  while (lines.length && lines[0].trim() === "") lines.shift();
  while (lines.length && lines[lines.length - 1].trim() === "") lines.pop();
  if (!lines.length) return null;
  const measured = lines.filter(l => l.trim() !== "");
  const min = measured.length ? Math.min(...measured.map(l => /^[ \t]*/.exec(l)[0].length)) : 0;
  const parts = [];
  lines.forEach((l, i) => {
    if (i > 0) parts.push(hardline);
    parts.push(l.trim() === "" ? "" : l.slice(min));
  });
  return concat(parts);
}

// Splits on commas that sit outside every quote and bracket. Returns the raw
// slices, including an empty trailing one when the list ends with a comma.
function splitTopLevel(text) {
  const out = [];
  let start = 0;
  let depth = 0;
  let quote = "";
  for (let i = 0; i < text.length; i++) {
    const ch = text[i];
    if (quote) {
      if (ch === "\\") i++;
      else if (ch === quote) quote = "";
      continue;
    }
    if (ch === '"' || ch === "'" || ch === "`") quote = ch;
    else if (ch === "(" || ch === "[" || ch === "{") depth++;
    else if (ch === ")" || ch === "]" || ch === "}") depth--;
    else if (ch === "," && depth === 0) {
      out.push(text.slice(start, i));
      start = i + 1;
    }
  }
  out.push(text.slice(start));
  return out;
}

// Offset of the opening bracket whose matching close is the final character,
// or -1 when the text does not end in a balanced group.
function trailingBracketStart(text) {
  let depth = 0;
  let quote = "";
  let openAt = -1;
  let result = -1;
  for (let i = 0; i < text.length; i++) {
    const ch = text[i];
    if (quote) {
      if (ch === "\\") i++;
      else if (ch === quote) quote = "";
      continue;
    }
    if (ch === '"' || ch === "'" || ch === "`") quote = ch;
    else if (ch === "(" || ch === "[" || ch === "{") {
      if (depth === 0) openAt = i;
      depth++;
    } else if (ch === ")" || ch === "]" || ch === "}") {
      depth--;
      if (depth < 0) return -1;
      if (depth === 0) result = i === text.length - 1 ? openAt : -1;
    }
  }
  return depth === 0 ? result : -1;
}

module.exports = { printDirectiveHead };
