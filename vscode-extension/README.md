# Oarkflow Template VS Code Extension

Language tooling for Oarkflow SPL templates.

## Features

- Syntax highlighting for `${...}`, SPL directives, filters, and `data-spl-*` hydration attributes.
- Completions for directives, built-in filters, hydration attributes, local variables, signals, handlers, blocks, and components.
- Hover descriptions for directives, filters, attributes, and local definitions.
- Go to definition for local components, blocks, variables, signals, handlers, and referenced include/import/extends files.
- Document symbols for components, layout blocks, definitions, slots, signals, handlers, and renders.
- Lightweight diagnostics for unknown directives, unbalanced braces/interpolations, missing directive blocks, and unknown built-in filters.
- **Formatting** for `.spl`, `.spl.html`, `.tmpl`, and HTML files containing SPL — whole document, selection, whole folder (recursively), and auto-indentation as you type.

The extension runs a small local Node.js language server process from `server/server.js` and does not require npm dependencies.

## Formatting

### How to format

Open the command palette (`cmd+shift+P` / `ctrl+shift+P`) and run:

| Command | Shortcut | What it does |
| --- | --- | --- |
| **SPL: Format This File** | `cmd+alt+L` / `ctrl+alt+L` | Formats the whole file. |
| **SPL: Format Selection** | — | Formats just the selected lines, keeping them at their current indentation. |
| **SPL: Format Folder** | — | Formats every template in a folder and all of its subfolders. |

The first two are also on the editor right-click menu. They format directly and do not depend on `editor.defaultFormatter`, so they work even when another extension owns HTML formatting.

The editor's built-in **Format Document** (`shift+alt+F`), **Format Selection**, and **Format On Save** work too, once this extension is selected as the formatter for the file type:

```json
"[spl-template]": { "editor.defaultFormatter": "oarkflow.oarkflow-template-vscode" },
"[html]": { "editor.defaultFormatter": "oarkflow.oarkflow-template-vscode" }
```

Pressing Enter inside a directive body or an HTML element indents the new line automatically, and typing `}` or `</` outdents it.

### Formatting a whole folder

Right-click a folder in the Explorer and choose **SPL: Format Folder**, or run the command from the palette and pick a folder. Several folders selected in the Explorer are formatted in one run.

The walk is recursive and picks up `.spl`, `.spl.html`, and `.tmpl` files, plus `.html` files while `oarkflowTemplate.enableHtmlFiles` is on. Skipped along the way:

- hidden folders — anything whose name starts with a dot, including `.git`;
- the folders listed in `oarkflowTemplate.format.excludeFolders`;
- symbolic links, so a link pointing back up the tree cannot loop.

Before anything is written, the command asks for confirmation and says how many files it found. Progress is reported in a notification and the run can be cancelled part-way; files already rewritten stay rewritten. Every file that changes is listed in the **Oarkflow Template** output channel, along with anything that could not be read or formatted.

Files open in an editor are changed through a workspace edit, so a single **Undo** in that editor reverts them. A file with unsaved changes is formatted but left unsaved — saving it stays your decision. Everything else is written straight to disk.

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
- The same goes for an argument list. `@render("Panel", { "id": x })` written on one line stays on one line; written across lines it is laid out one entry per line and indented under the directive, with nested objects and arrays following the same rule. Arguments holding code rather than data — an inline `@handler(save = { ... })` body, a template literal — keep their own line structure and are only re-anchored.
- Two tags written with nothing between them — `<input a><input b>` — stay glued together, because a line break there would render as a space. Set `whitespaceSensitivity` to `ignore` to let the formatter put each tag on its own line.
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
| `oarkflowTemplate.format.excludeFolders` | `node_modules`, `vendor`, `dist`, `build`, `out`, `coverage`, `target` | Folder names **SPL: Format Folder** skips. |
| `oarkflowTemplate.format.maxFilesPerRun` | `2000` | Most files **SPL: Format Folder** will rewrite in one run. |
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
