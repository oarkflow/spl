"use strict";

const test = require("node:test");
const assert = require("node:assert");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const { collectTemplateFiles, DEFAULT_EXTENSIONS } = require("../collect");

// Builds a throwaway tree from a { "relative/path": "contents" } map.
function tree(files) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "spl-collect-"));
  for (const [relative, contents] of Object.entries(files)) {
    const full = path.join(root, relative);
    fs.mkdirSync(path.dirname(full), { recursive: true });
    fs.writeFileSync(full, contents);
  }
  return root;
}

const relative = (root, files) => files.map(f => path.relative(root, f).split(path.sep).join("/")).sort();

test("finds templates in subdirectories", () => {
  const root = tree({
    "a.spl": "",
    "views/b.tmpl": "",
    "views/deep/nested/c.spl.html": "",
    "views/notes.txt": "",
    "README.md": ""
  });
  const { files, truncated, errors } = collectTemplateFiles(root);
  assert.deepStrictEqual(relative(root, files), ["a.spl", "views/b.tmpl", "views/deep/nested/c.spl.html"]);
  assert.strictEqual(truncated, false);
  assert.deepStrictEqual(errors, []);
});

test("includes .html only when asked to", () => {
  const root = tree({ "page.html": "", "card.spl": "" });
  assert.deepStrictEqual(relative(root, collectTemplateFiles(root).files), ["card.spl"]);
  const withHtml = collectTemplateFiles(root, { extensions: [...DEFAULT_EXTENSIONS, ".html"] });
  assert.deepStrictEqual(relative(root, withHtml.files), ["card.spl", "page.html"]);
});

test("skips excluded and hidden folders", () => {
  const root = tree({
    "keep.spl": "",
    "node_modules/pkg/x.spl": "",
    ".git/hooks/y.spl": "",
    "dist/z.spl": ""
  });
  assert.deepStrictEqual(relative(root, collectTemplateFiles(root).files), ["keep.spl"]);
});

test("honours a custom exclude list", () => {
  const root = tree({ "keep.spl": "", "legacy/old.spl": "", "node_modules/x.spl": "" });
  const { files } = collectTemplateFiles(root, { excludedFolders: ["legacy"] });
  assert.deepStrictEqual(relative(root, files), ["keep.spl", "node_modules/x.spl"]);
});

test("stops at the file limit and says so", () => {
  const root = tree({ "a.spl": "", "b.spl": "", "c.spl": "" });
  const { files, truncated } = collectTemplateFiles(root, { maxFiles: 2 });
  assert.strictEqual(files.length, 2);
  assert.strictEqual(truncated, true);
});

test("does not follow symlinked folders", () => {
  const root = tree({ "a.spl": "", "views/b.spl": "" });
  try {
    fs.symlinkSync(root, path.join(root, "views", "loop"), "dir");
  } catch {
    return; // symlinks are not available (Windows without developer mode)
  }
  const { files } = collectTemplateFiles(root);
  assert.deepStrictEqual(relative(root, files), ["a.spl", "views/b.spl"]);
});

test("accepts a single file as the root", () => {
  const root = tree({ "a.spl": "", "b.txt": "" });
  assert.deepStrictEqual(relative(root, collectTemplateFiles(path.join(root, "a.spl")).files), ["a.spl"]);
  assert.deepStrictEqual(collectTemplateFiles(path.join(root, "b.txt")).files, []);
});

test("reports an unreadable root instead of throwing", () => {
  const { files, errors } = collectTemplateFiles(path.join(os.tmpdir(), "spl-collect-does-not-exist"));
  assert.deepStrictEqual(files, []);
  assert.strictEqual(errors.length, 1);
});

test("stops early when cancelled", () => {
  const root = tree({ "a/x.spl": "", "b/y.spl": "", "c/z.spl": "" });
  let calls = 0;
  const { files } = collectTemplateFiles(root, { isCancelled: () => ++calls > 1 });
  assert.ok(files.length < 3, `expected an early stop, got ${files.length} files`);
});
