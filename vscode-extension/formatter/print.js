"use strict";

const data = require("./language-data");
const {
  concat, group, fill, indent, join,
  line, softline, hardline, literalline
} = require("./doc");

// Known HTML tag names. Anything outside this list (custom elements, framework
// tags, SPL components) is treated as inline, which is the conservative choice:
// inline nodes never gain or lose surrounding whitespace.
const KNOWN_HTML = new Set([
  "a", "abbr", "acronym", "address", "area", "article", "aside", "audio", "b", "base",
  "basefont", "bdi", "bdo", "big", "blockquote", "body", "br", "button", "canvas",
  "caption", "center", "cite", "code", "col", "colgroup", "data", "datalist", "dd",
  "del", "details", "dfn", "dialog", "dir", "div", "dl", "dt", "em", "embed",
  "fieldset", "figcaption", "figure", "font", "footer", "form", "frame", "frameset",
  "h1", "h2", "h3", "h4", "h5", "h6", "head", "header", "hgroup", "hr", "html", "i",
  "iframe", "img", "input", "ins", "kbd", "label", "legend", "li", "link", "main",
  "map", "mark", "marquee", "menu", "meta", "meter", "nav", "nobr", "noframes",
  "noscript", "object", "ol", "optgroup", "option", "output", "p", "param", "picture",
  "pre", "progress", "q", "rp", "rt", "ruby", "s", "samp", "script", "search",
  "section", "select", "slot", "small", "source", "span", "strike", "strong", "style",
  "sub", "summary", "sup", "table", "tbody", "td", "template", "textarea", "tfoot",
  "th", "thead", "time", "title", "tr", "track", "tt", "u", "ul", "var", "video", "wbr"
]);

// Containers that always put their children on separate lines.
const FORCE_BREAK_ELEMENTS = new Set([
  "html", "head", "body", "ul", "ol", "dl", "select", "table", "thead", "tbody", "tfoot", "tr"
]);

function isInlineDisplay(name) {
  return data.isInlineElement(name) || !KNOWN_HTML.has(name);
}

function isBlockLevel(node) {
  switch (node.type) {
    case "element":
      return !isInlineDisplay(node.name);
    case "directive":
      return isBlockDirective(node);
    case "inlineDirective":
      return !data.INLINE_LEVEL_DIRECTIVES.has(node.kind);
    case "splComment":
    case "doctype":
    case "cdata":
    case "fragment":
      return true;
    case "comment":
      return node.raw.includes("\n");
    default:
      return false;
  }
}

function isWhitespaceSensitive(node, ctx) {
  if (ctx.whitespaceSensitivity === "ignore") return false;
  if (ctx.whitespaceSensitivity === "strict") return true;
  if (node.type === "element") return isInlineDisplay(node.name);
  return !isBlockLevel(node);
}

/* ------------------------------------------------------------- verbatim */

function verbatimDoc(text) {
  const lines = text.replace(/\r\n/g, "\n").split("\n");
  const parts = [];
  lines.forEach((l, i) => {
    if (i > 0) parts.push(literalline);
    parts.push(l);
  });
  return concat(parts);
}

// reindentDoc re-anchors a multi-line chunk at the current indentation while
// keeping its internal relative indentation intact.
function reindentDoc(text, { dropOuterBlanks = false } = {}) {
  let lines = text.replace(/\r\n/g, "\n").split("\n");
  if (dropOuterBlanks) {
    while (lines.length && lines[0].trim() === "") lines.shift();
    while (lines.length && lines[lines.length - 1].trim() === "") lines.pop();
  }
  if (lines.length <= 1) return lines[0] != null ? lines[0].replace(/\s+$/, "") : "";
  const measured = (dropOuterBlanks ? lines : lines.slice(1)).filter(l => l.trim() !== "");
  const min = measured.length
    ? Math.min(...measured.map(l => l.match(/^[ \t]*/)[0].length))
    : 0;
  const head = dropOuterBlanks ? lines[0].slice(min) : lines[0];
  const parts = [head.replace(/\s+$/, "")];
  for (const l of lines.slice(1)) {
    parts.push(hardline);
    parts.push(l.trim() === "" ? "" : l.slice(min).replace(/\s+$/, ""));
  }
  return concat(parts);
}

function reindentBlockDoc(text) {
  let lines = text.replace(/\r\n/g, "\n").split("\n");
  while (lines.length && lines[0].trim() === "") lines.shift();
  while (lines.length && lines[lines.length - 1].trim() === "") lines.pop();
  if (!lines.length) return null;
  const measured = lines.filter(l => l.trim() !== "");
  const min = measured.length
    ? Math.min(...measured.map(l => l.match(/^[ \t]*/)[0].length))
    : 0;
  const parts = [];
  lines.forEach((l, i) => {
    if (i > 0) parts.push(hardline);
    parts.push(l.trim() === "" ? "" : l.slice(min).replace(/\s+$/, ""));
  });
  return concat(parts);
}

/* ------------------------------------------------------------- children */

function buildSequence(children) {
  const items = [];
  const pushWs = raw => {
    if (items.length && items[items.length - 1].ws) items[items.length - 1].raw += raw;
    else items.push({ ws: true, raw });
  };
  for (const child of children) {
    if (child.type === "text") {
      if (!child.value) continue;
      for (const part of child.value.split(/(\s+)/)) {
        if (!part) continue;
        if (/^\s+$/.test(part)) pushWs(part);
        else items.push({ ws: false, node: { type: "word", value: part } });
      }
    } else {
      items.push({ ws: false, node: child });
    }
  }
  return items;
}

function separator(prev, next, ws, ctx) {
  if (ws) {
    const newlines = (ws.match(/\n/g) || []).length;
    if (newlines >= 2) return concat([hardline, hardline]);
    if (isBlockLevel(prev) || isBlockLevel(next)) return hardline;
    return line;
  }
  // No whitespace in the source. Two block-level neighbours may still be split
  // onto separate lines because whitespace between them is insignificant --
  // unless the user asked for strict whitespace handling.
  if (ctx.whitespaceSensitivity === "strict") return "";
  if (isBlockLevel(prev) && isBlockLevel(next)) return hardline;
  return "";
}

// printBody lays out a container's children and reports how its leading and
// trailing edges may break.
function printBody(children, sensitive, ctx) {
  const items = buildSequence(children);
  let leadWs = null;
  let trailWs = null;
  if (items.length && items[0].ws) leadWs = items.shift().raw;
  if (items.length && items[items.length - 1].ws) trailWs = items.pop().raw;

  if (!items.length) {
    const hadWs = !!(leadWs || trailWs);
    return {
      lead: "",
      content: sensitive && hadWs ? line : "",
      trail: "",
      empty: true,
      hasLeadWs: !!leadWs,
      hasTrailWs: !!trailWs,
      hasLeadNewline: false,
      hasTrailNewline: false,
      firstIsText: false
    };
  }

  const seq = [];
  for (const item of items) {
    if (item.ws) {
      if (seq.length) seq[seq.length - 1].wsAfter = item.raw;
    } else {
      seq.push({ node: item.node, wsAfter: null });
    }
  }

  const parts = [];
  seq.forEach((entry, i) => {
    parts.push(printNode(entry.node, ctx));
    if (i < seq.length - 1) parts.push(separator(entry.node, seq[i + 1].node, entry.wsAfter, ctx));
  });

  const lead = sensitive ? (leadWs ? line : "") : softline;
  const trail = sensitive ? (trailWs ? line : "") : softline;
  const firstType = seq[0].node.type;
  return {
    lead,
    content: parts.length === 1 ? parts[0] : fill(parts),
    trail,
    empty: false,
    hasLeadWs: !!leadWs,
    hasTrailWs: !!trailWs,
    hasLeadNewline: !!leadWs && leadWs.includes("\n"),
    hasTrailNewline: !!trailWs && trailWs.includes("\n"),
    firstIsText: firstType === "word" || firstType === "interp"
  };
}

/* ------------------------------------------------------------- elements */

function printAttributes(node, ctx) {
  const open = `<${node.rawName}`;
  const bracket = node.selfClosing ? "/>" : ">";
  if (!node.attrs.length) return node.selfClosing ? `${open} />` : `${open}>`;

  const isMultiline = attr => /[\n\r]/.test(attr.raw);
  // A multi-line attribute (a wrapped value, or an `@if` used in attribute
  // position) is re-anchored so the printer keeps tracking the column correctly.
  const attrDocs = node.attrs.map(a => (isMultiline(a) ? reindentDoc(a.raw) : a.raw));
  if (node.attrs.length === 1 && !isMultiline(node.attrs[0])) {
    const only = node.attrs[0].raw;
    return node.selfClosing ? `${open} ${only} />` : `${open} ${only}>`;
  }
  const shouldBreak = node.attrs.some(isMultiline);
  return group(
    concat([
      open,
      indent(concat([line, join(line, attrDocs)])),
      node.selfClosing ? concat([line, bracket]) : ctx.bracketSameLine ? bracket : concat([softline, bracket])
    ]),
    { shouldBreak }
  );
}

function printElement(node, ctx) {
  if (node.kind === "pre") {
    const parts = [printAttributes(node, ctx)];
    if (node.content) parts.push(verbatimDoc(node.content));
    if (node.hasEndTag) parts.push(node.rawClose);
    return concat(parts);
  }

  if (node.kind === "rawtext") {
    const openTag = printAttributes(node, ctx);
    const closeTag = node.hasEndTag ? node.rawClose : "";
    if (!node.content || !node.content.trim()) return concat([openTag, closeTag]);
    // Template literals and SPL output make leading whitespace significant.
    const reindentable = ctx.indentEmbeddedCode && !node.content.includes("`");
    const body = reindentable ? reindentBlockDoc(node.content) : null;
    if (!body) return concat([openTag, verbatimDoc(node.content), closeTag]);
    return concat([openTag, indent(concat([hardline, body])), hardline, closeTag]);
  }

  const openTag = printAttributes(node, ctx);
  if (node.isVoid || node.selfClosing) return openTag;

  const closeTag = node.hasEndTag ? node.rawClose : "";
  if (!node.children.length) return concat([openTag, closeTag]);

  const sensitive = isWhitespaceSensitive(node, ctx);
  const body = printBody(node.children, sensitive, ctx);
  // Break when the author already put the content on its own lines, or when the
  // element is one that always lists its children vertically.
  const shouldBreak =
    (!sensitive && FORCE_BREAK_ELEMENTS.has(node.name)) ||
    (body.hasLeadNewline && !body.firstIsText && (body.hasTrailNewline || !sensitive));
  return group(concat([openTag, indent(concat([body.lead, body.content])), body.trail, closeTag]), {
    shouldBreak
  });
}

/* ----------------------------------------------------------- directives */

// A directive body may be broken onto its own lines only when the author
// already placed a line break on both sides of it. `@if(i > 0) {, }` used
// mid-sentence has no line breaks, so breaking it would change the output.
function directiveIsBreakable(node) {
  if (node.breakable != null) return node.breakable;
  node.breakable = node.branches.every(branch => {
    if (branch.verbatim != null) return true;
    const kids = branch.children;
    if (!kids.length) return false;
    const first = kids[0];
    if (!(first.type === "text" && /^[^\S\n]*\n/.test(first.value))) return false;
    // An unterminated body has no closing brace, so only its opening edge counts.
    if (branch.closed === false) return true;
    const last = kids[kids.length - 1];
    return last.type === "text" && /\n[^\S\n]*$/.test(last.value);
  });
  return node.breakable;
}

// A directive is block-level when it is structural, has several branches, wraps
// block-level markup, or is written across several lines.
function isBlockDirective(node) {
  if (node.blockLevel != null) return node.blockLevel;
  node.blockLevel = true; // guard against cycles while descending
  node.blockLevel =
    data.ALWAYS_BLOCK_DIRECTIVES.has(node.kind) ||
    node.branches.length > 1 ||
    directiveIsBreakable(node) ||
    node.branches.some(branch => branch.children.some(child => isBlockLevel(child)));
  return node.blockLevel;
}

function printDirective(node, ctx) {
  // Directive bodies are always whitespace-sensitive: `@if(i > 0) {, }` used
  // mid-sentence must never gain or lose the spaces around its content.
  const sensitive = ctx.whitespaceSensitivity !== "ignore";
  const breakable = directiveIsBreakable(node);
  const parts = [];

  node.branches.forEach((branch, i) => {
    parts.push(i === 0 ? `${branch.head} {` : `} ${branch.head} {`);
    if (branch.verbatim != null) {
      // @raw keeps its body byte for byte; @handler holds JavaScript we may re-anchor.
      if (branch.kind === "handler" && ctx.indentEmbeddedCode) {
        const body = reindentBlockDoc(branch.verbatim);
        if (body) parts.push(indent(concat([hardline, body])), hardline);
      } else if (branch.verbatim) {
        parts.push(verbatimDoc(branch.verbatim));
      }
      return;
    }
    const body = printBody(branch.children, sensitive, ctx);
    parts.push(indent(concat([body.lead, body.content])), body.trail);
  });

  // Only emit a closing brace when the source actually had one.
  if (node.branches[node.branches.length - 1].closed !== false) parts.push("}");
  return group(concat(parts), { shouldBreak: breakable });
}

/* ---------------------------------------------------------------- nodes */

function printInterp(node) {
  const raw = node.raw;
  if (raw.startsWith("${") && raw.endsWith("}")) {
    const inner = raw.slice(2, -1);
    if (!/[\n\r]/.test(inner)) return `\${${inner.trim()}}`;
  }
  return reindentDoc(raw);
}

function printNode(node, ctx) {
  switch (node.type) {
    case "word":
      return node.value;
    case "interp":
      return printInterp(node);
    case "orphan":
      return node.raw;
    case "doctype":
      return node.raw.replace(/\s*\n\s*/g, " ");
    case "cdata":
      return verbatimDoc(node.raw);
    case "comment":
      return node.raw.includes("\n") ? reindentDoc(node.raw) : node.raw;
    case "fragment":
      return reindentDoc(node.raw);
    case "splComment":
      return node.raw;
    case "inlineDirective":
      return node.raw;
    case "directive":
      return printDirective(node, ctx);
    case "element":
      return printElement(node, ctx);
    default:
      return "";
  }
}

function printRoot(root, ctx) {
  const { content } = printBody(root.children, false, ctx);
  if (!content || content === "") return "";
  return concat([content, hardline]);
}

module.exports = { printRoot, isBlockLevel, isInlineDisplay };
