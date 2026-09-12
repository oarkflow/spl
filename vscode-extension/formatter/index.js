"use strict";

const { parse } = require("./parse");
const { printRoot } = require("./print");
const { printDocToString } = require("./doc");

const DEFAULTS = {
  printWidth: 120,
  tabWidth: 2,
  useTabs: false,
  whitespaceSensitivity: "css", // "css" | "strict" | "ignore"
  bracketSameLine: false,
  indentEmbeddedCode: true,
  baseIndent: "",
  endOfLine: "auto",
  delimLeft: "${",
  delimRight: "}"
};

class FormatError extends Error {}

function resolveOptions(options) {
  const opts = Object.assign({}, DEFAULTS, options || {});
  opts.tabWidth = Math.max(1, Number(opts.tabWidth) || DEFAULTS.tabWidth);
  opts.printWidth = Math.max(20, Number(opts.printWidth) || DEFAULTS.printWidth);
  return opts;
}

function detectEol(text) {
  const crlf = (text.match(/\r\n/g) || []).length;
  const lf = (text.match(/\n/g) || []).length - crlf;
  return crlf > lf ? "\r\n" : "\n";
}

// Strips every whitespace character so two renderings can be compared for
// content equality. The formatter only ever moves whitespace around, so any
// difference here means a bug and the original text is returned untouched.
function contentFingerprint(text) {
  return text.replace(/\s+/g, "");
}

/**
 * Format an SPL template (or an HTML file containing SPL).
 *
 * @param {string} source
 * @param {object} [options]
 * @returns {string} the formatted source
 */
function format(source, options) {
  const opts = resolveOptions(options);
  const eol = opts.endOfLine === "crlf" ? "\r\n" : opts.endOfLine === "lf" ? "\n" : detectEol(source);
  const normalized = source.replace(/\r\n/g, "\n").replace(/\r/g, "\n");

  if (!normalized.trim()) return "";

  const tree = parse(normalized, opts);
  const doc = printRoot(tree, opts);
  let output = opts.baseIndent + printDocToString(doc, opts);
  // Exactly one trailing newline, so repeated runs cannot accumulate blank lines.
  output = output.replace(/[\t ]*\n\s*$/, "") + "\n";

  if (contentFingerprint(output) !== contentFingerprint(normalized)) {
    throw new FormatError(
      "SPL formatter aborted: the formatted output would not preserve the document's content."
    );
  }

  return eol === "\r\n" ? output.replace(/\n/g, "\r\n") : output;
}

module.exports = { format, FormatError, DEFAULTS };
