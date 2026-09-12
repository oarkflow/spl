"use strict";

// HTML element tables plus the SPL directive catalogue the formatter needs in
// order to decide what is a block, what is inline, and what must stay verbatim.

const VOID_ELEMENTS = new Set([
  "area", "base", "basefont", "br", "col", "embed", "frame", "hr", "img", "input",
  "isindex", "keygen", "link", "meta", "param", "source", "track", "wbr"
]);

// Elements whose content is raw text: no tags or SPL nesting is parsed inside.
const RAW_TEXT_ELEMENTS = new Set(["script", "style"]);

// Elements whose content must survive formatting byte for byte.
const PRE_ELEMENTS = new Set(["pre", "textarea", "listing", "plaintext", "xmp"]);

// CSS `display: inline`-ish elements. Whitespace around these is significant,
// so the formatter only ever converts existing whitespace into line breaks.
const INLINE_ELEMENTS = new Set([
  "a", "abbr", "acronym", "b", "bdi", "bdo", "big", "br", "button", "cite", "code",
  "data", "datalist", "del", "dfn", "em", "embed", "font", "i", "img", "input",
  "ins", "kbd", "label", "map", "mark", "meter", "nobr", "noscript", "object",
  "output", "picture", "progress", "q", "rp", "rt", "ruby", "s", "samp", "select",
  "slot", "small", "span", "strike", "strong", "sub", "sup", "svg", "template",
  "textarea", "time", "tt", "u", "var", "wbr"
]);

// Tags that close a still-open sibling when they appear.
const IMPLICIT_CLOSE = {
  li: new Set(["li"]),
  dt: new Set(["dt", "dd"]),
  dd: new Set(["dt", "dd"]),
  p: new Set([
    "address", "article", "aside", "blockquote", "details", "div", "dl", "fieldset",
    "figcaption", "figure", "footer", "form", "h1", "h2", "h3", "h4", "h5", "h6",
    "header", "hgroup", "hr", "main", "menu", "nav", "ol", "p", "pre", "section",
    "table", "ul"
  ]),
  rt: new Set(["rt", "rp"]),
  rp: new Set(["rt", "rp"]),
  optgroup: new Set(["optgroup"]),
  option: new Set(["option", "optgroup"]),
  thead: new Set(["tbody", "tfoot"]),
  tbody: new Set(["tbody", "tfoot"]),
  tfoot: new Set(["tbody"]),
  tr: new Set(["tr"]),
  td: new Set(["td", "th", "tr"]),
  th: new Set(["td", "th", "tr"]),
  caption: new Set(["colgroup", "thead", "tbody", "tfoot", "tr"]),
  colgroup: new Set(["thead", "tbody", "tfoot", "tr"])
};

// SPL directives that accept a `{ ... }` body.
const BLOCK_DIRECTIVES = new Set([
  "if", "elseif", "else", "for", "empty", "switch", "match", "case", "default",
  "raw", "block", "define", "prepend", "append", "hasBlock", "component", "render",
  "fill", "watch", "effect", "reactive", "stream", "defer", "lazy", "fallback",
  "cache", "translate", "handler"
]);

// SPL directives that never take a body.
const INLINE_DIRECTIVES = new Set([
  "include", "import", "extends", "parent", "let", "computed", "computedClient",
  "signal", "local", "assets", "bind", "click", "slot",
  "schema_form", "schema_detail", "schema_table"
]);

// Directives that must be followed by `(` to be recognised as SPL. This keeps
// stray `@` characters in prose and CSS at-rules from being mis-parsed.
const REQUIRES_PARENS = new Set([
  "if", "elseif", "for", "switch", "match", "case", "include", "import", "extends",
  "block", "define", "prepend", "append", "hasBlock", "component", "render", "fill",
  "let", "computed", "computedClient", "watch", "signal", "local", "assets", "bind",
  "effect", "reactive", "click", "lazy", "cache", "translate", "handler",
  "schema_form", "schema_detail", "schema_table"
]);

// Directives whose body is literal output or raw JavaScript; never reformatted.
const VERBATIM_BODY_DIRECTIVES = new Set(["raw", "handler"]);

// Which continuation keywords may follow a given directive's closing brace.
const CONTINUATIONS = {
  if: new Set(["elseif", "else"]),
  elseif: new Set(["elseif", "else"]),
  for: new Set(["empty"]),
  hasBlock: new Set(["else"]),
  defer: new Set(["fallback"]),
  lazy: new Set(["fallback"])
};

// Directives that render inline content and should not force a line break.
const INLINE_LEVEL_DIRECTIVES = new Set(["slot", "bind", "parent"]);

// Structural directives always get their own indented block, even when their
// body would fit on one line. `@if` / `@for` / `@translate` are decided by what
// they contain, so short inline conditionals stay inline.
const ALWAYS_BLOCK_DIRECTIVES = new Set([
  "component", "define", "block", "prepend", "append", "hasBlock", "switch", "match",
  "case", "default", "reactive", "effect", "stream", "defer", "lazy", "cache", "fill",
  "render", "watch", "handler", "raw"
]);

const ALL_DIRECTIVES = new Set([...BLOCK_DIRECTIVES, ...INLINE_DIRECTIVES]);

function isVoidElement(name) {
  return VOID_ELEMENTS.has(name.toLowerCase());
}

function isRawTextElement(name) {
  return RAW_TEXT_ELEMENTS.has(name.toLowerCase());
}

function isPreElement(name) {
  return PRE_ELEMENTS.has(name.toLowerCase());
}

function isInlineElement(name) {
  return INLINE_ELEMENTS.has(name.toLowerCase());
}

function closesImplicitly(openName, currentName) {
  const set = IMPLICIT_CLOSE[openName.toLowerCase()];
  return !!set && set.has(currentName.toLowerCase());
}

module.exports = {
  BLOCK_DIRECTIVES,
  REQUIRES_PARENS,
  VERBATIM_BODY_DIRECTIVES,
  CONTINUATIONS,
  INLINE_LEVEL_DIRECTIVES,
  ALWAYS_BLOCK_DIRECTIVES,
  ALL_DIRECTIVES,
  isVoidElement,
  isRawTextElement,
  isPreElement,
  isInlineElement,
  closesImplicitly
};
