# Oarkflow Template VS Code Extension

Language tooling for Oarkflow SPL templates.

## Features

- Syntax highlighting for `${...}`, SPL directives, filters, and `data-spl-*` hydration attributes.
- Completions for directives, built-in filters, hydration attributes, local variables, signals, handlers, blocks, and components.
- Hover descriptions for directives, filters, attributes, and local definitions.
- Go to definition for local components, blocks, variables, signals, handlers, and referenced include/import/extends files.
- Document symbols for components, layout blocks, definitions, slots, signals, handlers, and renders.
- Lightweight diagnostics for unknown directives, unbalanced braces/interpolations, missing directive blocks, and unknown built-in filters.
- **Formatting** for `.spl`, `.spl.html`, `.tmpl`, and HTML files containing SPL — whole document, selection, and auto-indentation as you type.

The extension runs a small local Node.js language server process from `server/server.js` and does not require npm dependencies.

## Formatting

### How to format

Open the command palette (`cmd+shift+P` / `ctrl+shift+P`) and run:

| Command | Shortcut | What it does |
| --- | --- | --- |
| **SPL: Format This File** | `cmd+alt+L` / `ctrl+alt+L` | Formats the whole file. |
| **SPL: Format Selection** | — | Formats just the selected lines, keeping them at their current indentation. |

Both are also on the editor right-click menu. They format directly and do not depend on `editor.defaultFormatter`, so they work even when another extension owns HTML formatting.

The editor's built-in **Format Document** (`shift+alt+F`), **Format Selection**, and **Format On Save** work too, once this extension is selected as the formatter for the file type:

```json
"[spl-template]": { "editor.defaultFormatter": "oarkflow.oarkflow-template-vscode" },
"[html]": { "editor.defaultFormatter": "oarkflow.oarkflow-template-vscode" }
```

Pressing Enter inside a directive body or an HTML element indents the new line automatically, and typing `}` or `</` outdents it.

### If the commands do not appear

Run **SPL: Show Status** — it reports the extension version, the language VS Code resolved for the current file, and whether the tooling and formatter are active. Its full output also goes to the **Oarkflow Template** output channel.

Two things commonly get in the way:

- **Another extension owns `.spl`.** The older `oarkflow.spl-vscode` extension claims `.spl` for a different language (`spl`), so VS Code may pick that one. **SPL: Show Status** names the culprit when this happens. Either uninstall it (`make vscode-extension` from the repository root does this) or pin the association:

  ```json
  "files.associations": { "*.spl": "spl-template" }
  ```

  Formatting still works on `.spl` files either way — the commands match on the file path — but syntax highlighting and auto-indent need the right language.

- **Another formatter owns the file type.** If `editor.defaultFormatter` points elsewhere (Prettier, for example), `shift+alt+F` will use that instead. **SPL: Format This File** bypasses this entirely; to change what `shift+alt+F` does, set the per-language override shown above.

The formatter understands SPL and HTML together, so a template like this:

```
@component("Card", title) {
<article class="card">
@if(title) {
<h2>${title}</h2>
}
</article>
}
```

becomes:

```
@component("Card", title) {
  <article class="card">
    @if(title) {
      <h2>${title}</h2>
    }
  </article>
}
```

### What it will not do

The formatter only moves whitespace around. It never rewrites your markup, and every run is checked against the original before any edit is applied — if the output would not preserve the document's content character for character (whitespace aside), the edit is discarded and the reason is logged to the **Oarkflow Template** output channel.

Specifically:

- `@raw { ... }` bodies and `<pre>` / `<textarea>` content are preserved byte for byte.
- `<script>`, `<style>`, and `@handler` bodies are re-anchored to the surrounding indentation but their code is never reformatted. Scripts containing template literals are left completely alone.
- Whitespace around inline elements is preserved, because it is significant. `@if(i > 0) {, }${tag}` used mid-sentence stays exactly as written.
- A directive body is only split across lines when you already wrote it across lines.
- Tags are never rewritten: `<br>` does not become `<br />`.
- Unbalanced markup — including tags that open inside one directive branch and close in another — is left in place rather than restructured.

### Settings

| Setting | Default | Description |
| --- | --- | --- |
| `oarkflowTemplate.format.enable` | `true` | Enable the formatter. |
| `oarkflowTemplate.format.printWidth` | `120` | Column to wrap at. Lines that cannot be broken safely may still exceed it. |
| `oarkflowTemplate.format.whitespaceSensitivity` | `css` | `css` respects each element's default display; `strict` never adds or removes whitespace; `ignore` reflows freely. |
| `oarkflowTemplate.format.bracketSameLine` | `false` | Put the closing `>` of a multi-line start tag on the last attribute line. |
| `oarkflowTemplate.format.indentEmbeddedCode` | `true` | Re-indent `<script>`, `<style>`, and `@handler` bodies. |
| `oarkflowTemplate.enableHtmlFiles` | `true` | Enable SPL tooling and formatting in `.html` files. |

Indentation width follows the editor's own `tabSize` / `insertSpaces` settings.

## Development

```sh
npm test          # formatter unit, robustness, and idempotency tests
npm run check     # syntax-check the extension and language server
npm run verify:go # format testdata/templates and assert the Go engine renders them identically
```

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
