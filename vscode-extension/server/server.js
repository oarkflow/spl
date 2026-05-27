"use strict";

const fs = require("fs");
const path = require("path");

const directives = {
  if: "Conditional block. Syntax: @if(condition) { ... } @elseif(other) { ... } @else { ... }",
  elseif: "Alternative branch for @if.",
  else: "Fallback branch for @if.",
  for: "Loop over arrays or hashes. Syntax: @for(item in items) { ... } or @for(key, value in map) { ... } @empty { ... }",
  empty: "Fallback branch rendered when a @for iterable is empty.",
  switch: "Switch on an expression with @case and @default branches.",
  case: "Branch inside @switch or @match.",
  default: "Fallback branch inside @switch or @match.",
  match: "Pattern matching block. Supports optional guards: @case(pattern if guard) { ... }.",
  raw: "Literal block. SPL syntax inside @raw { ... } is not parsed.",
  include: "Render another template. Syntax: @include(\"path.html\") or @include(\"path.html\", dataExpr).",
  import: "Import component definitions from another template.",
  handler: "Register a client-side hydration handler. Syntax: @handler(name = expression) or multiline body.",
  extends: "Use a layout template. Syntax: @extends(\"layouts/main.html\").",
  block: "Layout block placeholder.",
  define: "Define content for a layout block.",
  component: "Define a reusable component with optional props.",
  render: "Render a component by name with optional props and children.",
  slot: "Slot placeholder inside a component.",
  fill: "Named slot content inside @render.",
  let: "Assign a computed value for the current render scope.",
  computed: "Define a derived value. Same render-time behavior as @let, with clearer intent.",
  watch: "Render a block only when the watched expression value changes.",
  signal: "Declare a reactive signal for SSR hydration.",
  bind: "Bind a signal to textContent or another attribute.",
  effect: "Hydration effect block with signal dependencies.",
  reactive: "Reactive view block re-rendered when dependencies change.",
  click: "Create a button that mutates a signal.",
  stream: "Streaming block.",
  defer: "Deferred block with optional @fallback.",
  lazy: "Lazy block gated by an expression with optional @fallback.",
  fallback: "Fallback branch for @defer or @lazy."
};

const filters = {
  upper: "Uppercase a string.",
  lower: "Lowercase a string.",
  trim: "Trim leading and trailing whitespace.",
  title: "Title-case words.",
  capitalize: "Uppercase the first rune.",
  escape: "HTML-escape output.",
  json: "JSON-encode a value.",
  format: "Format with a Go fmt-style format string.",
  default: "Use an argument when the value is empty.",
  join: "Join list-like values.",
  first: "First character.",
  last: "Last character.",
  length: "String length in runes.",
  reverse: "Reverse a string.",
  truncate: "Truncate text. Example: ${text | truncate 20 \"...\"}.",
  nl2br: "Replace newlines with <br>.",
  urlencode: "URL query-escape a string.",
  striptags: "Remove HTML tags.",
  slug: "Create a lowercase URL slug.",
  replace: "Replace all occurrences. Example: ${name | replace \"a\" \"b\"}.",
  split: "Split a string.",
  repeat: "Repeat a string.",
  padstart: "Left-pad to a length.",
  padend: "Right-pad to a length.",
  wrap: "Wrap output with prefix and suffix."
};

const attrs = {
  "data-spl-model": "Two-way bind an input to a signal or dot path.",
  "data-spl-bind": "Bind an element property to a signal.",
  "data-spl-bind-value": "Bind value to a signal.",
  "data-spl-bind-checked": "Bind checked state to a signal.",
  "data-spl-bind-textContent": "Bind textContent to a signal.",
  "data-spl-bind-innerHTML": "Bind innerHTML to a signal.",
  "data-spl-on-click": "Run a hydration expression on click.",
  "data-spl-api-url": "Fetch URL for declarative API integration.",
  "data-spl-api-method": "HTTP method for declarative API integration.",
  "data-spl-api-target": "Selector updated with API response.",
  "data-spl-api-event": "DOM event that triggers the API request.",
  "data-spl-ref": "Register an element reference by name.",
  "data-spl-if": "Show element when a signal is truthy.",
  "data-spl-else": "Show element when a signal is falsy."
};

const cssAtRules = new Set([
  "charset", "color-profile", "container", "counter-style", "document", "font-face", "font-feature-values",
  "font-palette-values", "import", "keyframes", "layer", "media", "namespace", "page", "property", "scope",
  "starting-style", "supports", "viewport"
]);

let input = Buffer.alloc(0);
process.stdin.on("data", data => {
  input = Buffer.concat([input, data]);
  read();
});

function read() {
  while (input.length) {
    const headerEnd = input.indexOf("\r\n\r\n");
    if (headerEnd < 0) return;
    const match = /Content-Length:\s*(\d+)/i.exec(input.slice(0, headerEnd).toString("utf8"));
    if (!match) return;
    const length = Number(match[1]);
    const start = headerEnd + 4;
    if (input.length < start + length) return;
    const msg = JSON.parse(input.slice(start, start + length).toString("utf8"));
    input = input.slice(start + length);
    handle(msg);
  }
}

function handle(msg) {
  try {
    const p = msg.params || {};
    const result = ({
      "textDocument/completion": () => completions(p),
      "textDocument/hover": () => hover(p),
      "textDocument/definition": () => definition(p),
      "textDocument/documentSymbol": () => symbols(p),
      "textDocument/diagnostic": () => diagnostics(p)
    }[msg.method] || (() => null))();
    send({ jsonrpc: "2.0", id: msg.id, result });
  } catch (err) {
    send({ jsonrpc: "2.0", id: msg.id, error: { code: -32603, message: err.message } });
  }
}

function send(msg) {
  const body = JSON.stringify(msg);
  process.stdout.write(`Content-Length: ${Buffer.byteLength(body)}\r\n\r\n${body}`);
}

function analyze(text, uri) {
  const defs = [];
  const vars = new Map();
  const addDef = (kind, name, index, detail, scopeStart = null, scopeEnd = null) => defs.push({ kind, name, index, detail, scopeStart, scopeEnd, range: wordRangeAt(text, index, name.length) });
  for (const m of text.matchAll(/@(component|define|block|fill|render)\s*\(\s*["']([^"']+)["']/g)) addDef(m[1], m[2], m.index + m[0].lastIndexOf(m[2]), `${m[1]} "${m[2]}"`);
  for (const comp of componentDeclarations(text)) {
    for (const prop of comp.props) {
      vars.set(prop.localName, { index: prop.index, detail: `prop ${prop.name}${prop.alias ? ` as ${prop.alias}` : ""}${prop.defaultValue ? ` = ${prop.defaultValue}` : ""}`, scopeStart: comp.bodyStart, scopeEnd: comp.bodyEnd });
      addDef("prop", prop.localName, prop.index, `@component "${comp.name}" prop`, comp.bodyStart, comp.bodyEnd);
    }
  }
  for (const m of text.matchAll(/@(let|computed|signal|handler)\s*\(\s*([A-Za-z_][\w]*)/g)) {
    vars.set(m[2], { index: m.index + m[0].lastIndexOf(m[2]), detail: `@${m[1]} ${m[2]}` });
    addDef(m[1], m[2], m.index + m[0].lastIndexOf(m[2]), `@${m[1]} ${m[2]}`);
  }
  for (const m of text.matchAll(/@for\s*\(\s*(?:([A-Za-z_][\w]*)\s*,\s*)?([A-Za-z_][\w]*)\s+in\s+([^)]+)\)/g)) {
    if (m[1]) vars.set(m[1], { index: m.index + m[0].indexOf(m[1]), detail: "loop key" });
    vars.set(m[2], { index: m.index + m[0].lastIndexOf(m[2]), detail: "loop value" });
  }
  return { text, uri, defs, vars };
}

function completions(p) {
  const ctx = analyze(p.text, p.uri);
  const line = linePrefix(p.text, p.position);
  if (/\|\s*[\w-]*$/.test(line)) return Object.entries(filters).map(([label, documentation]) => item(label, "Function", documentation));
  if (/data-spl-[\w-]*$/.test(line)) return Object.entries(attrs).map(([label, documentation]) => item(label, "Property", documentation));
  const base = Object.entries(directives).map(([label, documentation]) => item(`@${label}`, "Keyword", documentation, directiveSnippet(label)));
  const names = [...ctx.vars.keys()].map(label => item(label, "Variable", ctx.vars.get(label).detail));
  const componentNames = ctx.defs.filter(d => d.kind === "component").map(d => item(d.name, "Class", d.detail));
  return base.concat(Object.entries(filters).map(([label, documentation]) => item(label, "Function", documentation))).concat(names, componentNames);
}

function hover(p) {
  const ctx = analyze(p.text, p.uri);
  const w = wordAt(p.text, offsetAt(p.text, p.position));
  if (!w) return null;
  const plain = w.replace(/^@/, "");
  if (directives[plain]) return { contents: `**${w.startsWith("@") ? w : "@" + plain}**\n\n${directives[plain]}` };
  if (filters[plain]) return { contents: `**filter: ${plain}**\n\n${filters[plain]}` };
  if (attrs[w]) return { contents: `**${w}**\n\n${attrs[w]}` };
  const found = ctx.defs.find(d => d.name === plain) || (ctx.vars.has(plain) && { detail: ctx.vars.get(plain).detail });
  return found ? { contents: `**${plain}**\n\n${found.detail}` } : null;
}

function definition(p) {
  const ctx = analyze(p.text, p.uri);
  const offset = offsetAt(p.text, p.position);
  const w = wordAt(p.text, offset)?.replace(/^@/, "");
  if (!w) return null;
  const candidates = [w, w.split(".")[0]];
  const defName = candidates.find(name => ctx.defs.find(d => d.name === name) || ctx.vars.has(name));
  const matchingDefs = defName ? ctx.defs.filter(d => d.name === defName) : [];
  const scopedDef = chooseScopedDefinition(matchingDefs, offset);
  const def = scopedDef || (defName && ctx.vars.has(defName) && { index: ctx.vars.get(defName).index, range: wordRangeAt(p.text, ctx.vars.get(defName).index, defName.length) });
  if (!def) return includeDefinition(p, w);
  return { uri: p.uri, range: def.range };
}

function chooseScopedDefinition(defs, offset) {
  if (!defs.length) return null;
  return defs
    .filter(d => d.scopeStart != null && d.scopeStart <= offset && offset <= d.scopeEnd)
    .sort((a, b) => (a.scopeEnd - a.scopeStart) - (b.scopeEnd - b.scopeStart))[0] || defs.find(d => d.scopeStart == null) || defs[0];
}

function includeDefinition(p, word) {
  const off = offsetAt(p.text, p.position);
  const before = p.text.lastIndexOf("@", off);
  const after = p.text.indexOf(")", off);
  if (before < 0 || after < 0) return null;
  const call = p.text.slice(before, after + 1);
  const m = /@(include|import|extends)\s*\(\s*["']([^"']+)["']/.exec(call);
  if (!m) return null;
  const base = uriToPath(p.uri);
  const target = path.resolve(path.dirname(base), m[2]);
  if (!fs.existsSync(target)) return null;
  return { uri: pathToUri(target), range: rangeFromOffsets("", 0, 0) };
}

function symbols(p) {
  return analyze(p.text, p.uri).defs.map(d => ({
    name: d.name,
    detail: d.detail,
    kind: d.kind === "component" ? "Class" : d.kind === "render" ? "Method" : "Field",
    range: d.range,
    selectionRange: d.range
  }));
}

function diagnostics(p) {
  const out = [];
  const max = p.maxDiagnostics || 200;
  const stack = [];
  const pairs = { if: "}", for: "}", switch: "}", match: "}", raw: "}", block: "}", define: "}", component: "}", render: "}", fill: "}", watch: "}", effect: "}", reactive: "}", stream: "}", defer: "}", lazy: "}" };
  for (const m of p.text.matchAll(/@\w+|\$\{|[{}]/g)) {
    if (out.length >= max) break;
    const token = m[0];
    const name = token.slice(1);
    if (token === "${") stack.push({ token, index: m.index });
    else if (token === "{") stack.push({ token, index: m.index });
    else if (token === "}") {
      const open = stack.pop();
      if (!open) out.push(diag(p.text, m.index, 1, "Unmatched closing brace.", "Error"));
    } else if (token.startsWith("@") && !directives[name] && !cssAtRules.has(name)) {
      out.push(diag(p.text, m.index, token.length, `Unknown SPL directive ${token}.`, "Warning"));
    } else if (pairs[name]) {
      const next = p.text.slice(m.index, m.index + 300);
      if (!/\{/.test(next)) out.push(diag(p.text, m.index, token.length, `${token} expects a block with { ... }.`, "Warning"));
    }
  }
  for (const open of stack) out.push(diag(p.text, open.index, open.token.length, `Unclosed ${open.token}.`, "Error"));
  for (const m of p.text.matchAll(/\|\s*([A-Za-z_][\w]*)/g)) {
    if (!filters[m[1]]) out.push(diag(p.text, m.index + m[0].indexOf(m[1]), m[1].length, `Unknown built-in filter "${m[1]}". Custom filters may still be registered in Go.`, "Information"));
  }
  return out.slice(0, max);
}

function item(label, kind, documentation, insertText) {
  return { label, kind, documentation, insertText: insertText || label };
}

function directiveSnippet(label) {
  const snippets = {
    "@if": "@if(${1:condition}) {\n  $0\n}",
    "@for": "@for(${1:item} in ${2:items}) {\n  $0\n} @empty {\n  ${3:No items.}\n}",
    "@component": "@component(\"${1:Name}\", ${2:prop}) {\n  $0\n}",
    "@render": "@render(\"${1:Name}\", {${2:key}: ${3:value}}) {\n  $0\n}",
    "@signal": "@signal(${1:name} = ${2:value})"
  };
  return snippets[`@${label}`] || `@${label}`;
}

function componentDeclarations(text) {
  const out = [];
  const re = /@component\s*\(/g;
  for (const m of text.matchAll(re)) {
    const open = m.index + m[0].length - 1;
    const close = findMatching(text, open, "(", ")");
    if (close < 0) continue;
    const inner = text.slice(open + 1, close);
    const parts = splitTopLevel(inner, ",");
    if (!parts.length) continue;
    const name = unquote(parts[0].text.trim());
    if (!name) continue;
    const bodyOpen = findNextNonWhitespace(text, close + 1);
    const bodyClose = bodyOpen >= 0 && text[bodyOpen] === "{" ? findMatching(text, bodyOpen, "{", "}") : -1;
    const props = [];
    for (const part of parts.slice(1)) {
      const parsed = parseProp(part.text);
      if (!parsed) continue;
      const search = parsed.alias || parsed.name;
      const relative = part.text.indexOf(search);
      props.push({
        ...parsed,
        localName: parsed.alias || parsed.name,
        index: open + 1 + part.start + Math.max(relative, 0)
      });
    }
    out.push({ name, index: m.index, props, bodyStart: bodyOpen >= 0 ? bodyOpen : close, bodyEnd: bodyClose >= 0 ? bodyClose : text.length });
  }
  return out;
}

function findNextNonWhitespace(text, start) {
  for (let i = start; i < text.length; i++) {
    if (!/\s/.test(text[i])) return i;
  }
  return -1;
}

function parseProp(raw) {
  const eq = findTopLevelAssign(raw);
  const left = (eq >= 0 ? raw.slice(0, eq) : raw).trim();
  const defaultValue = eq >= 0 ? raw.slice(eq + 1).trim() : "";
  if (!left) return null;
  const asMatch = /^([A-Za-z_][\w-]*)\s+as\s+([A-Za-z_][\w-]*)$/.exec(left);
  if (asMatch) return { name: asMatch[1], alias: asMatch[2], defaultValue };
  const nameMatch = /^([A-Za-z_][\w-]*)$/.exec(left);
  return nameMatch ? { name: nameMatch[1], alias: "", defaultValue } : null;
}

function splitTopLevel(input, sep) {
  const out = [];
  let start = 0, depth = 0, quote = "";
  for (let i = 0; i < input.length; i++) {
    const ch = input[i];
    if (quote) {
      if (ch === "\\") i++;
      else if (ch === quote) quote = "";
      continue;
    }
    if (ch === "\"" || ch === "'" || ch === "`") quote = ch;
    else if (ch === "(" || ch === "{" || ch === "[") depth++;
    else if (ch === ")" || ch === "}" || ch === "]") depth--;
    else if (ch === sep && depth === 0) {
      out.push({ text: input.slice(start, i), start });
      start = i + 1;
    }
  }
  out.push({ text: input.slice(start), start });
  return out;
}

function findTopLevelAssign(input) {
  let depth = 0, quote = "";
  for (let i = 0; i < input.length; i++) {
    const ch = input[i];
    if (quote) {
      if (ch === "\\") i++;
      else if (ch === quote) quote = "";
      continue;
    }
    if (ch === "\"" || ch === "'" || ch === "`") quote = ch;
    else if (ch === "(" || ch === "{" || ch === "[") depth++;
    else if (ch === ")" || ch === "}" || ch === "]") depth--;
    else if (ch === "=" && depth === 0 && input[i - 1] !== "!" && input[i - 1] !== "<" && input[i - 1] !== ">" && input[i + 1] !== "=") return i;
  }
  return -1;
}

function findMatching(text, open, left, right) {
  let depth = 0, quote = "";
  for (let i = open; i < text.length; i++) {
    const ch = text[i];
    if (quote) {
      if (ch === "\\") i++;
      else if (ch === quote) quote = "";
      continue;
    }
    if (ch === "\"" || ch === "'" || ch === "`") quote = ch;
    else if (ch === left) depth++;
    else if (ch === right) {
      depth--;
      if (depth === 0) return i;
    }
  }
  return -1;
}

function unquote(value) {
  const s = value.trim();
  if ((s.startsWith("\"") && s.endsWith("\"")) || (s.startsWith("'") && s.endsWith("'"))) return s.slice(1, -1);
  return s;
}

function diag(text, index, len, message, severity) {
  return { range: wordRangeAt(text, index, len), message, severity };
}

function offsetAt(text, pos) {
  let line = 0, col = 0;
  for (let i = 0; i < text.length; i++) {
    if (line === pos.line && col === pos.character) return i;
    if (text[i] === "\n") { line++; col = 0; } else col++;
  }
  return text.length;
}

function positionAt(text, offset) {
  let line = 0, character = 0;
  for (let i = 0; i < Math.min(offset, text.length); i++) {
    if (text[i] === "\n") { line++; character = 0; } else character++;
  }
  return { line, character };
}

function rangeFromOffsets(text, start, end) {
  return { start: positionAt(text, start), end: positionAt(text, end) };
}

function wordRangeAt(text, index, len) {
  return rangeFromOffsets(text, index, index + len);
}

function wordAt(text, offset) {
  const left = text.slice(0, offset).match(/[@A-Za-z0-9_.-]+$/)?.[0] || "";
  const right = text.slice(offset).match(/^[@A-Za-z0-9_.-]+/)?.[0] || "";
  return (left + right).match(/^@?[A-Za-z_][\w.-]*$/)?.[0] || null;
}

function linePrefix(text, pos) {
  const off = offsetAt(text, pos);
  return text.slice(text.lastIndexOf("\n", off - 1) + 1, off);
}

function uriToPath(uri) {
  return decodeURIComponent(uri.replace(/^file:\/\//, ""));
}

function pathToUri(file) {
  return `file://${file.split(path.sep).map(encodeURIComponent).join("/")}`;
}
