"use strict";

const test = require("node:test");
const assert = require("node:assert");
const fs = require("node:fs");
const path = require("node:path");
const { format } = require("..");

const templatesDir = path.resolve(__dirname, "..", "..", "..", "testdata", "templates");
const viewsDir = path.resolve(__dirname, "..", "..", "..", "examples", "fh", "views");

function collect(dir, out = []) {
  if (!fs.existsSync(dir)) return out;
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) collect(full, out);
    else if (/\.(html|spl|tmpl)$/.test(entry.name)) out.push(full);
  }
  return out;
}

const corpus = [...collect(templatesDir), ...collect(viewsDir)];

test("every repository template formats and is idempotent", () => {
  assert.ok(corpus.length > 0, "expected to find templates to format");
  for (const file of corpus) {
    const source = fs.readFileSync(file, "utf8");
    const once = format(source, {});
    assert.strictEqual(format(once, {}), once, `not idempotent: ${file}`);
  }
});

// Truncating and splicing real templates produces the kind of half-written
// markup an editor sees on every keystroke. None of it may crash the formatter,
// and formatting must settle within one extra pass.
test("survives truncated and spliced templates", () => {
  let seed = 12345;
  const rand = () => {
    seed = (seed * 1103515245 + 12345) & 0x7fffffff;
    return seed / 0x7fffffff;
  };
  const cases = [];
  for (const file of corpus) {
    const source = fs.readFileSync(file, "utf8");
    if (source.length < 200) continue;
    for (let i = 0; i < 25; i++) {
      const a = Math.floor(rand() * source.length);
      cases.push(source.slice(a, a + Math.floor(rand() * 2000)));
      const b = Math.floor(rand() * source.length);
      cases.push(source.slice(0, b) + source.slice(b + Math.floor(rand() * 150)));
    }
  }
  cases.push("", "{", "}", "<", "</", "<div", "${", "@if(", "@if(x) {", "@raw {", "<!--",
    "<div>${a", "@for(x in y) {<li>", "{a && (<div>)}", "<script>", "<style>{", "@@@", "@//");

  for (const input of cases) {
    let first;
    assert.doesNotThrow(() => {
      first = format(input, {});
    }, `threw on: ${JSON.stringify(input.slice(0, 120))}`);
    const second = format(first, {});
    assert.strictEqual(format(second, {}), second, `never settles: ${JSON.stringify(input.slice(0, 120))}`);
  }
});

test("formats a large template quickly", () => {
  const big = fs.readFileSync(path.join(templatesDir, "complete_reactive_showcase.html"), "utf8").repeat(40);
  const started = Date.now();
  const out = format(big, {});
  const elapsed = Date.now() - started;
  assert.ok(out.length > 0);
  assert.ok(elapsed < 5000, `formatting ${(big.length / 1024).toFixed(0)}KB took ${elapsed}ms`);
});
