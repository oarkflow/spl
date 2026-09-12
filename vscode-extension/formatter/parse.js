"use strict";

const data = require("./language-data");

const WS = /\s/;

// parse turns a mixed HTML/SPL source string into a tree of nodes.
//
// The parser is deliberately forgiving: anything it cannot make sense of is
// kept as an opaque node so the printer can emit it untouched.
function parse(source, options) {
  const ctx = {
    src: source,
    pos: 0,
    options: options || {},
    delimLeft: (options && options.delimLeft) || "${",
    delimRight: (options && options.delimRight) || "}"
  };
  const children = parseNodes(ctx, false);
  return { type: "root", children };
}

function eof(ctx) {
  return ctx.pos >= ctx.src.length;
}

function peek(ctx, offset) {
  return ctx.src[ctx.pos + (offset || 0)];
}

function startsWith(ctx, text, at) {
  return ctx.src.startsWith(text, at == null ? ctx.pos : at);
}

function parseNodes(ctx, inBlock) {
  const root = { type: "container", children: [] };
  const stack = [root];
  let textBuf = "";
  let textBraceDepth = 0;

  const top = () => stack[stack.length - 1];
  const flushText = () => {
    if (textBuf) {
      top().children.push({ type: "text", value: textBuf });
      textBuf = "";
    }
  };
  const add = node => {
    flushText();
    top().children.push(node);
  };

  while (!eof(ctx)) {
    const ch = peek(ctx);

    // Closing brace of the enclosing directive body.
    if (inBlock && ch === "}") {
      if (textBraceDepth > 0) {
        textBraceDepth--;
        textBuf += ch;
        ctx.pos++;
        continue;
      }
      ctx.pos++;
      flushText();
      unwind(stack);
      ctx.blockClosed = true;
      return root.children;
    }

    // ${ ... } interpolation
    if (startsWith(ctx, ctx.delimLeft)) {
      const end = scanInterpolation(ctx, ctx.pos);
      if (end > ctx.pos) {
        add({ type: "interp", raw: ctx.src.slice(ctx.pos, end) });
        ctx.pos = end;
        continue;
      }
    }

    // JSX-style conditional fragment: {cond && <div/>} / {cond || <span/>}
    if (ch === "{") {
      const end = scanConditionalFragment(ctx.src, ctx.pos);
      if (end > 0) {
        add({ type: "fragment", raw: ctx.src.slice(ctx.pos, end) });
        ctx.pos = end;
        continue;
      }
    }

    if (ch === "@") {
      if (peek(ctx, 1) === "/" && peek(ctx, 2) === "/") {
        const nl = ctx.src.indexOf("\n", ctx.pos);
        const end = nl < 0 ? ctx.src.length : nl;
        add({ type: "splComment", raw: ctx.src.slice(ctx.pos, end).replace(/\s+$/, "") });
        ctx.pos = end;
        continue;
      }
      const directive = tryParseDirective(ctx, stack, flushText);
      if (directive) {
        add(directive);
        continue;
      }
    }

    if (ch === "<") {
      const handled = tryParseTag(ctx, stack, flushText);
      if (handled) continue;
    }

    if (inBlock && ch === "{") textBraceDepth++;
    textBuf += ch;
    ctx.pos++;
  }

  flushText();
  unwind(stack);
  ctx.blockClosed = false;
  return root.children;
}

// unwind replaces every element left open on the stack with an opaque orphan
// node followed by its children, so unbalanced markup still round-trips.
function unwind(stack) {
  while (stack.length > 1) {
    const el = stack.pop();
    const parent = stack[stack.length - 1];
    const idx = parent.children.lastIndexOf(el);
    if (idx < 0) continue;
    parent.children.splice(idx, 1, { type: "orphan", raw: el.rawOpen }, ...el.children);
  }
}

/* ------------------------------------------------------------------ tags */

function tryParseTag(ctx, stack, flushText) {
  const src = ctx.src;
  const start = ctx.pos;

  if (startsWith(ctx, "<!--")) {
    const close = src.indexOf("-->", start + 4);
    const end = close < 0 ? src.length : close + 3;
    flushText();
    stack[stack.length - 1].children.push({ type: "comment", raw: src.slice(start, end) });
    ctx.pos = end;
    return true;
  }

  if (startsWith(ctx, "<![CDATA[")) {
    const close = src.indexOf("]]>", start + 9);
    const end = close < 0 ? src.length : close + 3;
    flushText();
    stack[stack.length - 1].children.push({ type: "cdata", raw: src.slice(start, end) });
    ctx.pos = end;
    return true;
  }

  if (src[start + 1] === "!" || src[start + 1] === "?") {
    const close = src.indexOf(">", start);
    const end = close < 0 ? src.length : close + 1;
    flushText();
    stack[stack.length - 1].children.push({ type: "doctype", raw: src.slice(start, end) });
    ctx.pos = end;
    return true;
  }

  if (src[start + 1] === "/") {
    const m = /^<\/\s*([A-Za-z][-A-Za-z0-9_:.]*)\s*>/.exec(src.slice(start));
    if (!m) return false;
    flushText();
    closeElement(stack, m[1], src.slice(start, start + m[0].length));
    ctx.pos = start + m[0].length;
    return true;
  }

  const nameMatch = /^<([A-Za-z][-A-Za-z0-9_:.]*)/.exec(src.slice(start));
  if (!nameMatch) return false;
  const rawName = nameMatch[1];
  const parsed = readAttributes(ctx, start + nameMatch[0].length);
  if (!parsed) return false;

  const lower = rawName.toLowerCase();
  const rawOpen = src.slice(start, parsed.end);
  flushText();

  // Elements whose content never contains markup.
  if (!parsed.selfClosing && data.isRawTextElement(lower)) {
    const closeRe = new RegExp(`</\\s*${lower}\\s*>`, "i");
    const rest = src.slice(parsed.end);
    const m = closeRe.exec(rest);
    const contentEnd = m ? parsed.end + m.index : src.length;
    const node = {
      type: "element",
      kind: "rawtext",
      name: lower,
      rawName,
      attrs: parsed.attrs,
      rawOpen,
      content: src.slice(parsed.end, contentEnd),
      hasEndTag: !!m,
      rawClose: m ? m[0] : "",
      children: []
    };
    stack[stack.length - 1].children.push(node);
    ctx.pos = m ? contentEnd + m[0].length : src.length;
    return true;
  }

  // Elements whose content is whitespace-significant.
  if (!parsed.selfClosing && data.isPreElement(lower)) {
    const end = findMatchingCloseTag(src, parsed.end, lower);
    const node = {
      type: "element",
      kind: "pre",
      name: lower,
      rawName,
      attrs: parsed.attrs,
      rawOpen,
      content: src.slice(parsed.end, end.contentEnd),
      hasEndTag: end.found,
      rawClose: end.rawClose,
      children: []
    };
    stack[stack.length - 1].children.push(node);
    ctx.pos = end.next;
    return true;
  }

  const isVoid = data.isVoidElement(lower);
  const node = {
    type: "element",
    kind: "element",
    name: lower,
    rawName,
    attrs: parsed.attrs,
    rawOpen,
    selfClosing: parsed.selfClosing,
    isVoid,
    hasEndTag: false,
    rawClose: "",
    children: []
  };

  // Pop siblings that this tag implicitly closes (`<li>` after `<li>`, ...).
  while (stack.length > 1) {
    const openEl = stack[stack.length - 1];
    if (openEl.type === "element" && data.closesImplicitly(openEl.name, lower)) stack.pop();
    else break;
  }

  stack[stack.length - 1].children.push(node);
  ctx.pos = parsed.end;
  if (!isVoid && !parsed.selfClosing) stack.push(node);
  return true;
}

function closeElement(stack, name, raw) {
  const lower = name.toLowerCase();
  let idx = -1;
  for (let i = stack.length - 1; i >= 1; i--) {
    if (stack[i].type === "element" && stack[i].name === lower) {
      idx = i;
      break;
    }
  }
  if (idx < 0) {
    // Stray close tag: keep it as opaque text rather than restructuring.
    stack[stack.length - 1].children.push({ type: "orphan", raw });
    return;
  }
  while (stack.length - 1 > idx) stack.pop();
  const el = stack.pop();
  el.hasEndTag = true;
  el.rawClose = raw;
}

function findMatchingCloseTag(src, from, lower) {
  const re = new RegExp(`<\\s*(/?)\\s*${lower}\\b[^>]*>`, "gi");
  re.lastIndex = from;
  let depth = 1;
  let m;
  while ((m = re.exec(src))) {
    depth += m[1] ? -1 : 1;
    if (depth === 0) {
      return { contentEnd: m.index, next: m.index + m[0].length, found: true, rawClose: m[0] };
    }
  }
  return { contentEnd: src.length, next: src.length, found: false, rawClose: "" };
}

// readAttributes scans from just after the tag name up to `>` or `/>`.
function readAttributes(ctx, from) {
  const src = ctx.src;
  const attrs = [];
  let i = from;
  while (i < src.length) {
    while (i < src.length && WS.test(src[i])) i++;
    if (i >= src.length) return null;
    if (src[i] === ">") return { attrs, selfClosing: false, end: i + 1 };
    if (src[i] === "/" && src[i + 1] === ">") return { attrs, selfClosing: true, end: i + 2 };

    const attrStart = i;
    if (src.startsWith(ctx.delimLeft, i)) {
      i = skipInterpolation(ctx, i);
      attrs.push({ raw: src.slice(attrStart, i) });
      continue;
    }
    if (src[i] === "@") {
      const end = scanDirectiveAtom(ctx, i);
      if (end > i) {
        i = end;
        attrs.push({ raw: src.slice(attrStart, i) });
        continue;
      }
    }

    while (i < src.length && !WS.test(src[i]) && src[i] !== "=" && src[i] !== ">" && !(src[i] === "/" && src[i + 1] === ">")) {
      if (src.startsWith(ctx.delimLeft, i)) {
        i = skipInterpolation(ctx, i);
        continue;
      }
      // A directive glued to the previous attribute -- `novalidate@if(x) { y }` --
      // starts its own atom rather than being swallowed by the attribute name.
      if (i > attrStart && src[i] === "@" && scanDirectiveAtom(ctx, i) > i) break;
      i++;
    }
    if (i === attrStart) i++; // never stall on an unexpected character

    let j = i;
    while (j < src.length && WS.test(src[j])) j++;
    if (src[j] !== "=") {
      attrs.push({ raw: src.slice(attrStart, i) });
      continue;
    }
    j++;
    while (j < src.length && WS.test(src[j])) j++;
    const valueEnd = scanAttributeValue(ctx, j);
    attrs.push({ raw: `${src.slice(attrStart, i)}=${src.slice(j, valueEnd)}` });
    i = valueEnd;
  }
  return null;
}

function scanAttributeValue(ctx, from) {
  const src = ctx.src;
  let i = from;
  const quote = src[i];
  if (quote === '"' || quote === "'") {
    i++;
    while (i < src.length) {
      if (src.startsWith(ctx.delimLeft, i)) {
        i = skipInterpolation(ctx, i);
        continue;
      }
      if (src[i] === "@") {
        const end = scanDirectiveAtom(ctx, i);
        if (end > i) {
          i = end;
          continue;
        }
      }
      if (src[i] === quote) return i + 1;
      i++;
    }
    return src.length;
  }
  while (i < src.length && !WS.test(src[i]) && src[i] !== ">") {
    if (src.startsWith(ctx.delimLeft, i)) {
      i = skipInterpolation(ctx, i);
      continue;
    }
    i++;
  }
  return i;
}

/* ------------------------------------------------------------ directives */

function readKeyword(src, at) {
  const m = /^[A-Za-z_][A-Za-z0-9_]*/.exec(src.slice(at));
  return m ? m[0] : "";
}

// scanDirectiveAtom measures a directive used in attribute position, e.g.
// `@if(loop.first) { active }`. Returns -1 when this is not a directive.
function scanDirectiveAtom(ctx, at) {
  const src = ctx.src;
  const keyword = readKeyword(src, at + 1);
  if (!keyword || !data.ALL_DIRECTIVES.has(keyword)) return -1;
  let i = at + 1 + keyword.length;
  let j = i;
  while (j < src.length && (src[j] === " " || src[j] === "\t")) j++;
  if (src[j] === "(") {
    const close = scanBalanced(src, j, "(", ")");
    if (close < 0) return -1;
    i = close;
  } else if (data.REQUIRES_PARENS.has(keyword)) {
    return -1;
  }
  if (!data.BLOCK_DIRECTIVES.has(keyword)) return i;
  let k = i;
  while (k < src.length && WS.test(src[k])) k++;
  if (src[k] !== "{") return i;
  const close = scanBalanced(src, k, "{", "}");
  return close < 0 ? i : close;
}

function tryParseDirective(ctx, stack, flushText) {
  const src = ctx.src;
  const start = ctx.pos;
  const keyword = readKeyword(src, start + 1);
  if (!keyword || !data.ALL_DIRECTIVES.has(keyword)) return null;

  const head = readDirectiveHead(ctx, start);
  if (!head) return null;

  // Directive with a `{ ... }` body?
  let bodyAt = head.end;
  while (bodyAt < src.length && WS.test(src[bodyAt])) bodyAt++;
  const hasBody = data.BLOCK_DIRECTIVES.has(keyword) && src[bodyAt] === "{" && !head.inlineHandler;
  if (!hasBody) {
    if (data.REQUIRES_PARENS.has(keyword) && !head.hasParens) return null;
    ctx.pos = head.end;
    return { type: "inlineDirective", kind: keyword, raw: head.text };
  }

  const node = { type: "directive", kind: keyword, branches: [], startOffset: start };
  let currentKind = keyword;
  let currentHead = head;
  for (;;) {
    let at = currentHead.end;
    while (at < src.length && WS.test(src[at])) at++;
    ctx.pos = at + 1; // consume '{'

    if (data.VERBATIM_BODY_DIRECTIVES.has(currentKind)) {
      const close = scanBalanced(src, at, "{", "}");
      const closed = close >= 0;
      const end = closed ? close - 1 : src.length;
      node.branches.push({
        head: currentHead.text,
        verbatim: src.slice(at + 1, Math.max(end, at + 1)),
        kind: currentKind,
        closed,
        children: []
      });
      ctx.pos = closed ? close : src.length;
    } else {
      const children = parseNodes(ctx, true);
      node.branches.push({
        head: currentHead.text,
        kind: currentKind,
        children,
        closed: ctx.blockClosed
      });
    }

    // An unterminated body has no closing brace to print or to continue from.
    if (!node.branches[node.branches.length - 1].closed) break;

    const allowed = data.CONTINUATIONS[currentKind];
    if (!allowed) break;
    let probe = ctx.pos;
    while (probe < src.length && WS.test(src[probe])) probe++;
    if (src[probe] !== "@") break;
    const nextKeyword = readKeyword(src, probe + 1);
    if (!allowed.has(nextKeyword)) break;
    const nextHead = readDirectiveHead(ctx, probe);
    if (!nextHead) break;
    let braceAt = nextHead.end;
    while (braceAt < src.length && WS.test(src[braceAt])) braceAt++;
    if (src[braceAt] !== "{") break;
    currentKind = nextKeyword;
    currentHead = nextHead;
  }

  node.endOffset = ctx.pos;
  node.spansLines = src.slice(start, ctx.pos).includes("\n");
  return node;
}

// readDirectiveHead reads `@name` plus its optional `( ... )` argument list.
function readDirectiveHead(ctx, at) {
  const src = ctx.src;
  const keyword = readKeyword(src, at + 1);
  if (!keyword) return null;
  let i = at + 1 + keyword.length;
  let j = i;
  while (j < src.length && (src[j] === " " || src[j] === "\t")) j++;
  let args = null;
  if (src[j] === "(") {
    const close = scanBalanced(src, j, "(", ")");
    if (close < 0) return null;
    args = src.slice(j + 1, close - 1);
    i = close;
  } else if (data.REQUIRES_PARENS.has(keyword)) {
    return null;
  }
  const text = args === null ? `@${keyword}` : `@${keyword}(${collapseArgs(args)})`;
  // `@handler(name = expr)` has no block body; `@handler(name) { ... }` does.
  const inlineHandler = keyword === "handler" && args !== null && hasTopLevelAssign(args);
  return { keyword, args, text, end: i, hasParens: args !== null, inlineHandler };
}

function collapseArgs(args) {
  if (!/[\n\r]/.test(args)) return args.trim();
  // Keep multi-line argument lists readable but strip trailing padding.
  return args
    .split("\n")
    .map(l => l.replace(/\s+$/, ""))
    .join("\n")
    .trim();
}

function hasTopLevelAssign(input) {
  let depth = 0;
  let quote = "";
  for (let i = 0; i < input.length; i++) {
    const ch = input[i];
    if (quote) {
      if (ch === "\\") i++;
      else if (ch === quote) quote = "";
      continue;
    }
    if (ch === '"' || ch === "'" || ch === "`") quote = ch;
    else if (ch === "(" || ch === "{" || ch === "[") depth++;
    else if (ch === ")" || ch === "}" || ch === "]") depth--;
    else if (ch === "=" && depth === 0 && input[i - 1] !== "!" && input[i - 1] !== "<" && input[i - 1] !== ">" && input[i + 1] !== "=") return true;
  }
  return false;
}

/* ---------------------------------------------------------------- scanners */

// scanBalanced returns the offset just past the matching close character.
function scanBalanced(src, at, open, close) {
  let depth = 0;
  let quote = "";
  let i = at;
  for (; i < src.length; i++) {
    const ch = src[i];
    if (quote) {
      if (ch === "\\") i++;
      else if (ch === quote) quote = "";
      continue;
    }
    if (ch === "/" && src[i + 1] === "/" && open === "{") {
      const nl = src.indexOf("\n", i);
      if (nl < 0) return -1;
      i = nl;
      continue;
    }
    if (ch === '"' || ch === "'" || ch === "`") {
      quote = ch;
      continue;
    }
    if (ch === open) depth++;
    else if (ch === close) {
      depth--;
      if (depth === 0) return i + 1;
    }
  }
  return -1;
}

// skipInterpolation is scanInterpolation for callers inside a loop: an
// unterminated `${` still advances the cursor so the scan cannot spin forever.
function skipInterpolation(ctx, at) {
  const end = scanInterpolation(ctx, at);
  return end > at ? end : at + ctx.delimLeft.length;
}

function scanInterpolation(ctx, at) {
  const src = ctx.src;
  const left = ctx.delimLeft;
  const right = ctx.delimRight;
  let i = at + left.length;
  if (right.length === 1) {
    let depth = 1;
    for (; i < src.length; i++) {
      const ch = src[i];
      if (ch === '"' || ch === "'" || ch === "`") {
        i = skipString(src, i);
        continue;
      }
      if (ch === "{") depth++;
      else if (ch === right) {
        depth--;
        if (depth === 0) return i + 1;
      }
    }
    return at;
  }
  const close = src.indexOf(right, i);
  return close < 0 ? at : close + right.length;
}

function skipString(src, at) {
  const quote = src[at];
  for (let i = at + 1; i < src.length; i++) {
    if (src[i] === "\\") {
      i++;
      continue;
    }
    if (src[i] === quote) return i;
  }
  return src.length - 1;
}

const FRAGMENT_SCAN_LIMIT = 20000;

// scanConditionalFragment recognises `{cond && <div/>}` / `{cond || <span/>}`.
function scanConditionalFragment(src, at) {
  let depth = 0;
  let quote = "";
  let matched = false;
  const limit = Math.min(src.length, at + FRAGMENT_SCAN_LIMIT);
  for (let i = at; i < limit; i++) {
    const ch = src[i];
    if (quote) {
      if (ch === "\\") i++;
      else if (ch === quote) quote = "";
      continue;
    }
    if (ch === '"' || ch === "'" || ch === "`") {
      quote = ch;
      continue;
    }
    if (ch === "{" || ch === "(" || ch === "[") {
      depth++;
      continue;
    }
    if (ch === "}" || ch === ")" || ch === "]") {
      depth--;
      if (depth === 0) return matched ? i + 1 : -1;
      continue;
    }
    if (depth === 1 && (ch === "&" || ch === "|") && src[i + 1] === ch) {
      let n = i + 2;
      while (n < src.length && WS.test(src[n])) n++;
      if (src[n] === "<" || src[n] === "(") matched = true;
    }
  }
  return -1;
}

module.exports = { parse };
