"use strict";

const test = require("node:test");
const assert = require("node:assert");
const { format } = require("..");

const fmt = (src, opts) => format(src, opts);

function assertFormat(input, expected, opts) {
  assert.strictEqual(fmt(input, opts), expected);
}

test("indents an SPL block directive and its HTML body", () => {
  assertFormat(
    `<div class="card">\n<h3>\${title}</h3>\n@if(badge) {\n<span>\${badge}</span>\n}\n</div>\n`,
    '<div class="card">\n  <h3>${title}</h3>\n  @if(badge) {\n    <span>${badge}</span>\n  }\n</div>\n'
  );
});

test("indents an unindented component definition", () => {
  assertFormat(
    `@component("Card", title) {\n<article>\n<h2>\${title}</h2>\n</article>\n}\n`,
    '@component("Card", title) {\n  <article>\n    <h2>${title}</h2>\n  </article>\n}\n'
  );
});

test("keeps @elseif / @else chains attached to the closing brace", () => {
  assertFormat(
    `@if(a) {\n<p>a</p>\n} @elseif(b) {\n<p>b</p>\n} @else {\n<p>c</p>\n}\n`,
    "@if(a) {\n  <p>a</p>\n} @elseif(b) {\n  <p>b</p>\n} @else {\n  <p>c</p>\n}\n"
  );
});

test("keeps @for / @empty together", () => {
  assertFormat(
    `@for(item in items) {\n<li>\${item}</li>\n} @empty {\n<li>none</li>\n}\n`,
    "@for(item in items) {\n  <li>${item}</li>\n} @empty {\n  <li>none</li>\n}\n"
  );
});

test("keeps @defer / @fallback together", () => {
  assertFormat(
    "@defer {\n<p>loaded</p>\n} @fallback {\n<p>loading</p>\n}\n",
    "@defer {\n  <p>loaded</p>\n} @fallback {\n  <p>loading</p>\n}\n"
  );
});

test("indents @switch branches", () => {
  assertFormat(
    `@switch(status) {\n@case("a") {\n<span>A</span>\n}\n@default {\n<span>?</span>\n}\n}\n`,
    '@switch(status) {\n  @case("a") {\n    <span>A</span>\n  }\n  @default {\n    <span>?</span>\n  }\n}\n'
  );
});

test("does not insert whitespace inside a directive used mid-text", () => {
  const src = "<p>@for(i, t in tags) {@if(i > 0) {, }${t}}</p>\n";
  assert.strictEqual(fmt(src), src);
});

test("preserves @raw bodies byte for byte", () => {
  const src = "@raw {\n  \\${not} @parsed\n     keep    this\n}\n";
  assert.strictEqual(fmt(src), src);
});

test("preserves <pre> content byte for byte", () => {
  const src = "<div>\n  <pre>\n  a\n     b\n  </pre>\n</div>\n";
  assert.strictEqual(fmt(src), "<div>\n  <pre>\n  a\n     b\n  </pre>\n</div>\n");
});

test("preserves whitespace around inline elements", () => {
  const src = "<p>Hello <b>there</b> friend</p>\n";
  assert.strictEqual(fmt(src), src);
  assert.strictEqual(fmt("<p>Hello<b>there</b>friend</p>\n"), "<p>Hello<b>there</b>friend</p>\n");
});

test("breaks long attribute lists one per line", () => {
  const out = fmt(
    `<input type="text" name="title" data-spl-model="todoDraft" placeholder="A very long placeholder value here" value="" />\n`,
    { printWidth: 80 }
  );
  assert.strictEqual(
    out,
    '<input\n  type="text"\n  name="title"\n  data-spl-model="todoDraft"\n  placeholder="A very long placeholder value here"\n  value=""\n/>\n'
  );
});

test("keeps a single long attribute on one line", () => {
  const src = `<div class="a-very-long-class-list that-keeps-going and-going and-going and-going and-going and-going">x</div>\n`;
  assert.strictEqual(fmt(src), src);
});

test("handles SPL inside attribute values", () => {
  const src = `<li class="@if(loop.first) {first} @if(loop.last) {last}">\${color}</li>\n`;
  assert.strictEqual(fmt(src), src);
});

test("handles interpolation and quotes inside attribute values", () => {
  const src = `<button onclick="openSnippet('\${action.unique_id}', this)" data-v="\${a ? a : 'primary'}">go</button>\n`;
  assert.strictEqual(fmt(src), src);
});

test("normalizes spacing inside interpolations", () => {
  assertFormat("<p>${  name | upper  }</p>\n", "<p>${name | upper}</p>\n");
});

test("keeps SPL comments on their own line", () => {
  assertFormat(
    "@// section header\n<div>\n<p>x</p>\n</div>\n",
    "@// section header\n<div>\n  <p>x</p>\n</div>\n"
  );
});

test("preserves a single blank line between siblings", () => {
  assertFormat(
    "<div>\n<p>a</p>\n\n\n\n<p>b</p>\n</div>\n",
    "<div>\n  <p>a</p>\n\n  <p>b</p>\n</div>\n"
  );
});

test("keeps void elements exactly as the author wrote them", () => {
  // `<br>` is never rewritten to `<br />`: the formatter only moves whitespace.
  assertFormat(
    "<div>\n<br>\n<img src=\"a.png\" alt=\"a\">\n<input value=\"x\" />\n</div>\n",
    '<div>\n  <br> <img src="a.png" alt="a"> <input value="x" />\n</div>\n'
  );
});

test("handles implicitly closed list items", () => {
  assertFormat(
    "<ul>\n<li>one\n<li>two\n</ul>\n",
    "<ul>\n  <li>one\n  <li>two\n</ul>\n"
  );
});

test("re-anchors <script> bodies without reformatting them", () => {
  assertFormat(
    "<div>\n<script>\nfunction a() {\n    return 1;\n}\n</script>\n</div>\n",
    "<div>\n  <script>\n    function a() {\n        return 1;\n    }\n  </script>\n</div>\n"
  );
});

test("leaves <script> bodies containing template literals alone", () => {
  const src = "<script>\nconst a = `line1\n   line2`;\n</script>\n";
  assert.strictEqual(fmt(src), src);
});

test("re-anchors @handler bodies", () => {
  assertFormat(
    "@handler(save) {\nconst x = 1;\n  return x;\n}\n",
    "@handler(save) {\n  const x = 1;\n    return x;\n}\n"
  );
});

test("leaves a stray close tag in place instead of restructuring", () => {
  const src = "<div>\n  <span>x</span>\n</em>\n</div>\n";
  assert.doesNotThrow(() => fmt(src));
  assert.match(fmt(src), /<\/em>/);
});

test("tolerates unbalanced markup across directive boundaries", () => {
  const src = `@if(a) {<div class="x">} @else {<div class="y">}\n<p>body</p>\n</div>\n`;
  const out = fmt(src);
  assert.match(out, /class="x"/);
  assert.match(out, /class="y"/);
  assert.match(out, /<\/div>/);
});

test("preserves conditional fragments verbatim", () => {
  const src = "<div>\n  {note != '' && (\n    <div class=\"n\">${note}</div>\n  )}\n</div>\n";
  assert.strictEqual(fmt(src), src);
});

test("keeps doctype and formats a whole document", () => {
  assertFormat(
    "<!DOCTYPE html>\n<html>\n<head>\n<title>t</title>\n</head>\n<body>\n<p>x</p>\n</body>\n</html>\n",
    "<!DOCTYPE html>\n<html>\n  <head>\n    <title>t</title>\n  </head>\n  <body>\n    <p>x</p>\n  </body>\n</html>\n"
  );
});

test("respects tabWidth and useTabs", () => {
  assertFormat("<div>\n<p>x</p>\n</div>\n", "<div>\n    <p>x</p>\n</div>\n", { tabWidth: 4 });
  assertFormat("<div>\n<p>x</p>\n</div>\n", "<div>\n\t<p>x</p>\n</div>\n", { useTabs: true });
});

test("respects printWidth", () => {
  const src = "<p>alpha beta gamma delta epsilon zeta eta theta</p>\n";
  assert.strictEqual(fmt(src, { printWidth: 200 }), src);
  // eslint-disable-next-line no-unused-expressions
  assert.strictEqual(
    fmt(src, { printWidth: 30 }),
    "<p>\n  alpha beta gamma delta\n  epsilon zeta eta theta\n</p>\n"
  );
});

test("preserves CRLF line endings", () => {
  const out = fmt("<div>\r\n<p>x</p>\r\n</div>\r\n");
  assert.strictEqual(out, "<div>\r\n  <p>x</p>\r\n</div>\r\n");
});

test("returns an empty string for blank input", () => {
  assert.strictEqual(fmt("   \n\n  "), "");
});

test("does not treat email addresses or unknown @words as directives", () => {
  const src = "<p>Contact me@example.com or @someone about @media queries.</p>\n";
  assert.strictEqual(fmt(src), src);
});

test("re-anchors <style> content without rewriting the CSS", () => {
  assertFormat(
    "<style>\n@media (min-width: 10px) {\n  .a { color: red; }\n}\n</style>\n",
    "<style>\n  @media (min-width: 10px) {\n    .a { color: red; }\n  }\n</style>\n"
  );
});

test("formats inline directives onto their own lines", () => {
  assertFormat(
    `@extends("layout.html")\n@import("ui.html")\n@signal(count = 0)\n<p>x</p>\n`,
    '@extends("layout.html")\n@import("ui.html")\n@signal(count = 0)\n<p>x</p>\n'
  );
});

test("keeps @slot inline within its element", () => {
  const src = '<header>@slot("header")</header>\n';
  assert.strictEqual(fmt(src), src);
});

test("is idempotent", () => {
  const samples = [
    `@component("C", a) {\n<div>\n@if(a) {<p>\${a}</p>}\n</div>\n}\n`,
    `<ul>@for(x in xs) {<li>\${x}</li>}</ul>\n`,
    "<div>\n<pre>  keep  </pre>\n<script>var a=1;</script>\n</div>\n",
    `@switch(s) {@case("a") {<b>a</b>} @default {<b>?</b>}}\n`
  ];
  for (const s of samples) {
    const once = fmt(s);
    assert.strictEqual(fmt(once), once, `not idempotent for: ${JSON.stringify(s)}`);
  }
});

test("preserves trailing whitespace inside verbatim blocks", () => {
  const src = "<div>\n<pre>a   \n  b\t\n</pre>\n</div>\n";
  assert.strictEqual(fmt(src), "<div>\n  <pre>a   \n  b\t\n</pre>\n</div>\n");
});

test("baseIndent indents the output without shifting verbatim content", () => {
  const out = fmt("<section>\n<pre>keep\n  me\n</pre>\n</section>\n", { baseIndent: "    " });
  assert.strictEqual(out, "    <section>\n      <pre>keep\n  me\n</pre>\n    </section>\n");
});

test("baseIndent is idempotent", () => {
  const src = "<div>\n<p>a</p>\n<p>b</p>\n</div>\n";
  const once = fmt(src, { baseIndent: "  " });
  assert.strictEqual(fmt(once.replace(/^ {2}/, ""), { baseIndent: "  " }), once);
});

test("keeps a directive glued to an attribute name as one unit", () => {
  assertFormat(
    `<form action="/x" novalidate@if(req) { data-required="true"}>\n<p>a</p>\n</form>\n`,
    '<form action="/x" novalidate @if(req) { data-required="true"}>\n  <p>a</p>\n</form>\n'
  );
});
