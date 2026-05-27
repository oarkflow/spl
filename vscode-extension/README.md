# Oarkflow Template VS Code Extension

Language tooling for Oarkflow SPL templates.

## Features

- Syntax highlighting for `${...}`, SPL directives, filters, and `data-spl-*` hydration attributes.
- Completions for directives, built-in filters, hydration attributes, local variables, signals, handlers, blocks, and components.
- Hover descriptions for directives, filters, attributes, and local definitions.
- Go to definition for local components, blocks, variables, signals, handlers, and referenced include/import/extends files.
- Document symbols for components, layout blocks, definitions, slots, signals, handlers, and renders.
- Lightweight diagnostics for unknown directives, unbalanced braces/interpolations, missing directive blocks, and unknown built-in filters.

The extension runs a small local Node.js language server process from `server/server.js` and does not require npm dependencies.

## Install From This Repository

Run from the repository root:

```sh
make vscode-extension
```

That installs the extension into `~/.vscode/extensions/oarkflow.oarkflow-template-vscode` and asks VS Code to reload the current window.

If your editor CLI is `codium` or another compatible command:

```sh
make vscode-extension VSCODE_CLI=codium
```
