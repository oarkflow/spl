#!/usr/bin/env node
"use strict";

// Formats every template under ../testdata/templates into a temporary directory
// and runs the Go engine's equivalence tests against it, proving the formatter
// does not change what a template renders.

const fs = require("fs");
const os = require("os");
const path = require("path");
const cp = require("child_process");
const { format } = require("../formatter");

const repoRoot = path.resolve(__dirname, "..", "..");
const sourceDir = path.join(repoRoot, "testdata", "templates");

if (!fs.existsSync(sourceDir)) {
  console.error(`no templates found at ${sourceDir}`);
  process.exit(1);
}

const outDir = fs.mkdtempSync(path.join(os.tmpdir(), "spl-fmt-"));
let failures = 0;

for (const name of fs.readdirSync(sourceDir)) {
  if (!name.endsWith(".html")) continue;
  const source = fs.readFileSync(path.join(sourceDir, name), "utf8");
  let formatted;
  try {
    formatted = format(source, {});
  } catch (err) {
    console.error(`FAIL ${name}: ${err.message}`);
    failures++;
    continue;
  }
  if (format(formatted, {}) !== formatted) {
    console.error(`FAIL ${name}: formatting is not idempotent`);
    failures++;
  }
  fs.writeFileSync(path.join(outDir, name), formatted);
}

if (failures) process.exit(1);

const result = cp.spawnSync("go", ["test", "-run", "TestFormatted", "."], {
  cwd: repoRoot,
  stdio: "inherit",
  env: Object.assign({}, process.env, { SPL_FMT_DIR: outDir })
});

fs.rmSync(outDir, { recursive: true, force: true });
process.exit(result.status == null ? 1 : result.status);
