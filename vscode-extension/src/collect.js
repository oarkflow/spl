"use strict";

// Finds the template files under a folder so the whole tree can be formatted in
// one go. Kept free of the `vscode` module so it can be unit tested directly.

const fs = require("fs");
const path = require("path");

const DEFAULT_EXTENSIONS = [".spl", ".spl.html", ".tmpl"];

// Folders that never hold hand-written templates. Anything starting with a dot
// is skipped as well, so `.git`, `.vscode`, and friends need no entry here.
const DEFAULT_EXCLUDED_FOLDERS = ["node_modules", "vendor", "dist", "build", "out", "coverage", "target"];

const DEFAULT_MAX_FILES = 2000;

function matchesExtension(name, extensions) {
  const lower = name.toLowerCase();
  return extensions.some(ext => lower.endsWith(ext.toLowerCase()));
}

/**
 * Collect every template file under `root`, depth first and in a stable order.
 *
 * Symbolic links are never followed: a link back up the tree would otherwise
 * make the walk run forever, and the file it points at is formatted anyway when
 * it lives inside the folder.
 *
 * @param {string} root
 * @param {object} [options]
 * @returns {{files: string[], truncated: boolean, errors: Array<{path: string, message: string}>}}
 */
function collectTemplateFiles(root, options) {
  const opts = options || {};
  const extensions = opts.extensions && opts.extensions.length ? opts.extensions : DEFAULT_EXTENSIONS;
  const excluded = new Set((opts.excludedFolders || DEFAULT_EXCLUDED_FOLDERS).map(name => name.toLowerCase()));
  const includeHidden = !!opts.includeHidden;
  const maxFiles = Number(opts.maxFiles) > 0 ? Number(opts.maxFiles) : DEFAULT_MAX_FILES;
  const isCancelled = typeof opts.isCancelled === "function" ? opts.isCancelled : () => false;

  const files = [];
  const errors = [];
  let truncated = false;

  let rootStat;
  try {
    rootStat = fs.statSync(root);
  } catch (err) {
    errors.push({ path: root, message: err.message });
    return { files, truncated, errors };
  }

  if (rootStat.isFile()) {
    if (matchesExtension(path.basename(root), extensions)) files.push(root);
    return { files, truncated, errors };
  }

  const queue = [root];
  while (queue.length) {
    if (isCancelled()) break;
    const dir = queue.shift();
    let entries;
    try {
      entries = fs.readdirSync(dir, { withFileTypes: true });
    } catch (err) {
      errors.push({ path: dir, message: err.message });
      continue;
    }
    entries.sort((a, b) => (a.name < b.name ? -1 : a.name > b.name ? 1 : 0));

    const subdirectories = [];
    for (const entry of entries) {
      if (entry.isSymbolicLink()) continue;
      const full = path.join(dir, entry.name);
      if (entry.isDirectory()) {
        if (!includeHidden && entry.name.startsWith(".")) continue;
        if (excluded.has(entry.name.toLowerCase())) continue;
        subdirectories.push(full);
        continue;
      }
      if (!entry.isFile()) continue;
      if (!includeHidden && entry.name.startsWith(".")) continue;
      if (!matchesExtension(entry.name, extensions)) continue;
      if (files.length >= maxFiles) {
        truncated = true;
        continue;
      }
      files.push(full);
    }
    // Depth first, so a folder's own files are listed before its children's.
    queue.unshift(...subdirectories);
  }

  return { files, truncated, errors };
}

module.exports = {
  collectTemplateFiles,
  matchesExtension,
  DEFAULT_EXTENSIONS,
  DEFAULT_EXCLUDED_FOLDERS,
  DEFAULT_MAX_FILES
};
