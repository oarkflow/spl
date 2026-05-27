const cp = require("child_process");
const path = require("path");
const vscode = require("vscode");

let server;
let extensionPath;
let workspaceRoot;
let nextId = 1;
const pending = new Map();
const buffers = new Map();
const diagnostics = vscode.languages.createDiagnosticCollection("oarkflow-template");
const semanticLegend = new vscode.SemanticTokensLegend(
  ["keyword", "function", "variable", "property", "operator", "string", "number", "comment"],
  []
);

function activate(context) {
  extensionPath = context.extensionPath;
  workspaceRoot = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath || context.extensionPath;
  startServer();

  context.subscriptions.push(diagnostics, { dispose: () => server?.kill() });
  context.subscriptions.push(vscode.workspace.onDidOpenTextDocument(syncDocument));
  context.subscriptions.push(vscode.workspace.onDidChangeTextDocument(e => syncDocument(e.document)));
  context.subscriptions.push(vscode.workspace.onDidCloseTextDocument(doc => diagnostics.delete(doc.uri)));
  context.subscriptions.push(vscode.commands.registerCommand("oarkflowTemplate.showStatus", () => {
    const active = vscode.window.activeTextEditor?.document;
    vscode.window.showInformationMessage(`Oarkflow Template extension active. Document language: ${active?.languageId || "none"}. Server: ${server?.stdin?.writable ? "running" : "not running"}.`);
  }));

  for (const doc of vscode.workspace.textDocuments) syncDocument(doc);

  const selector = [
    { language: "spl-template", scheme: "file" },
    { language: "html", scheme: "file" },
    { scheme: "file", pattern: "**/*.spl" },
    { scheme: "file", pattern: "**/*.spl.html" },
    { scheme: "file", pattern: "**/*.tmpl" }
  ];

  context.subscriptions.push(vscode.languages.registerCompletionItemProvider(selector, {
    async provideCompletionItems(document, position) {
      if (!isEnabled(document)) return;
      const items = await safeRequest("textDocument/completion", params(document, position), []);
      return items.map(toCompletion);
    }
  }, "@", "|", "\"", "'", " "));

  context.subscriptions.push(vscode.languages.registerHoverProvider(selector, {
    async provideHover(document, position) {
      if (!isEnabled(document)) return;
      const local = await localHover(document, position);
      if (local) return local;
      const result = await safeRequest("textDocument/hover", params(document, position), null);
      if (!result) return;
      return new vscode.Hover(new vscode.MarkdownString(result.contents));
    }
  }));

  context.subscriptions.push(vscode.languages.registerDefinitionProvider(selector, {
    async provideDefinition(document, position) {
      if (!isEnabled(document)) return;
      const local = await localDefinition(document, position);
      if (local) return local;
      const result = await safeRequest("textDocument/definition", params(document, position), null);
      if (!result) return;
      return new vscode.Location(vscode.Uri.parse(result.uri), toRange(result.range));
    }
  }));

  context.subscriptions.push(vscode.languages.registerDocumentSymbolProvider(selector, {
    async provideDocumentSymbols(document) {
      if (!isEnabled(document)) return;
      const symbols = await safeRequest("textDocument/documentSymbol", { uri: document.uri.toString(), text: document.getText() }, []);
      return symbols.map(s => new vscode.DocumentSymbol(s.name, s.detail, vscode.SymbolKind[s.kind] || vscode.SymbolKind.Object, toRange(s.range), toRange(s.selectionRange)));
    }
  }));

  context.subscriptions.push(vscode.languages.registerDocumentLinkProvider(selector, {
    provideDocumentLinks(document) {
      if (!isEnabled(document)) return;
      return templateLinks(document);
    }
  }));

  context.subscriptions.push(vscode.languages.registerReferenceProvider(selector, {
    async provideReferences(document, position) {
      if (!isEnabled(document)) return;
      return localReferences(document, position);
    }
  }));

  context.subscriptions.push(vscode.languages.registerDocumentSemanticTokensProvider(selector, {
    provideDocumentSemanticTokens(document) {
      if (!isEnabled(document)) return;
      return buildSemanticTokens(document);
    }
  }, semanticLegend));
}

function deactivate() {
  server?.kill();
}

function startServer() {
  if (server && !server.killed && server.stdin?.writable) return server;
  buffers.delete("server");
  server = cp.spawn(process.execPath, [path.join(extensionPath, "server", "server.js")], {
    cwd: workspaceRoot,
    stdio: ["pipe", "pipe", "pipe"]
  });
  server.stdout.on("data", readMessages);
  server.stderr.on("data", data => console.warn(`[oarkflow-template] ${data}`));
  server.on("exit", () => {
    server = undefined;
    for (const { reject } of pending.values()) reject(new Error("Oarkflow template server stopped"));
    pending.clear();
  });
  server.on("error", err => console.warn(`[oarkflow-template] server error: ${err.message}`));
  return server;
}

function isEnabled(document) {
  if (/\.(spl|spl\.html|tmpl)$/.test(document.uri.fsPath || "")) return true;
  if (document.languageId === "spl-template") return true;
  if (document.languageId !== "html") return false;
  return vscode.workspace.getConfiguration("oarkflowTemplate").get("enableHtmlFiles", true);
}

function params(document, position) {
  return { uri: document.uri.toString(), text: document.getText(), position: { line: position.line, character: position.character } };
}

function syncDocument(document) {
  if (!isEnabled(document)) return;
  request("textDocument/diagnostic", {
    uri: document.uri.toString(),
    text: document.getText(),
    maxDiagnostics: vscode.workspace.getConfiguration("oarkflowTemplate").get("maxDiagnostics", 200)
  }).then(items => {
    diagnostics.set(document.uri, items.map(d => new vscode.Diagnostic(toRange(d.range), d.message, vscode.DiagnosticSeverity[d.severity] || vscode.DiagnosticSeverity.Warning)));
  }).catch(() => {});
}

function toCompletion(item) {
  const out = new vscode.CompletionItem(item.label, vscode.CompletionItemKind[item.kind] || vscode.CompletionItemKind.Text);
  out.detail = item.detail;
  out.documentation = item.documentation ? new vscode.MarkdownString(item.documentation) : undefined;
  out.insertText = item.insertText ? new vscode.SnippetString(item.insertText) : item.label;
  return out;
}

function toRange(range) {
  return new vscode.Range(range.start.line, range.start.character, range.end.line, range.end.character);
}

function request(method, params) {
  const child = startServer();
  if (!child.stdin?.writable) {
    return Promise.reject(new Error("Oarkflow template server is not writable"));
  }
  const id = nextId++;
  const payload = JSON.stringify({ jsonrpc: "2.0", id, method, params });
  return new Promise((resolve, reject) => {
    pending.set(id, { resolve, reject });
    child.stdin.write(`Content-Length: ${Buffer.byteLength(payload, "utf8")}\r\n\r\n${payload}`, err => {
      if (err) {
        pending.delete(id);
        reject(err);
      }
    });
  });
}

async function safeRequest(method, params, fallback) {
  try {
    return await request(method, params);
  } catch (err) {
    console.warn(`[oarkflow-template] ${method} failed: ${err.message}`);
    return fallback;
  }
}

async function localHover(document, position) {
  const link = templateLinkAt(document, position);
  if (link) {
    const md = new vscode.MarkdownString(`**${link.kind}**\n\nOpen [${link.path}](${link.target.toString()}).`);
    md.isTrusted = true;
    return new vscode.Hover(md, link.range);
  }
  const symbol = templateSymbolAt(document, position);
  if (!symbol) return null;
  const def = await findSymbolDefinition(document, symbol.name, position);
  if (!def) return null;
  const md = new vscode.MarkdownString(`**${symbol.name}**\n\n${def.detail}\n\nGo to definition is available with Ctrl+Click or F12.`);
  return new vscode.Hover(md, symbol.range);
}

async function localDefinition(document, position) {
  const link = templateLinkAt(document, position);
  if (link) return new vscode.Location(link.target, new vscode.Position(0, 0));
  const symbol = templateSymbolAt(document, position);
  if (!symbol) return null;
  const def = await findSymbolDefinition(document, symbol.name, position);
  return def ? new vscode.Location(def.uri, def.range) : null;
}

function templateLinks(document) {
  const text = document.getText();
  const links = [];
  const re = /@(include|import|extends)\s*\(\s*(["'])([^"']+)\2/g;
  for (const m of text.matchAll(re)) {
    const pathStart = m.index + m[0].indexOf(m[3]);
    const range = new vscode.Range(document.positionAt(pathStart), document.positionAt(pathStart + m[3].length));
    const target = resolveTemplateUri(document, m[3]);
    if (!target) continue;
    const link = new vscode.DocumentLink(range, target);
    link.tooltip = `Open ${m[1]} template ${m[3]}`;
    links.push(link);
  }
  return links;
}

function templateLinkAt(document, position) {
  return templateLinks(document).map(link => ({
    range: link.range,
    target: link.target,
    path: document.getText(link.range),
    kind: "template file"
  })).find(link => link.range.contains(position));
}

function resolveTemplateUri(document, relativePath) {
  if (document.uri.scheme !== "file") return null;
  const target = vscode.Uri.file(path.resolve(path.dirname(document.uri.fsPath), relativePath));
  return fsExists(target.fsPath) ? target : null;
}

function templateSymbolAt(document, position) {
  const text = document.getText();
  const offset = document.offsetAt(position);
  const found = symbolAtOffset(text, offset);
  if (!found) return null;
  const range = new vscode.Range(document.positionAt(found.start), document.positionAt(found.end));
  const name = found.name;
  if (!name || /^data-spl-/.test(name)) return null;
  return { name, range };
}

function symbolAtOffset(text, offset) {
  const isWord = ch => /[A-Za-z0-9_.-]/.test(ch || "");
  let start = offset;
  let end = offset;
  if (!isWord(text[start]) && isWord(text[start - 1])) start--;
  while (start > 0 && isWord(text[start - 1])) start--;
  while (end < text.length && isWord(text[end])) end++;
  const name = text.slice(start, end);
  if (!/^[A-Za-z_][A-Za-z0-9_.-]*$/.test(name)) return null;
  return { name, start, end };
}

async function findSymbolDefinition(document, name, position) {
  const candidates = [name, name.split(".")[0]];
  const currentOffset = position ? document.offsetAt(position) : -1;
  const docs = await relatedTemplateDocuments(document);
  for (const doc of docs) {
    const defs = collectTemplateDefinitions(doc);
    for (const candidate of candidates) {
      const matches = defs.filter(d => d.name === candidate);
      const def = chooseScopedDefinition(document, doc, currentOffset, matches);
      if (def) return def;
    }
  }
  return null;
}

function chooseScopedDefinition(originDocument, defDocument, currentOffset, defs) {
  if (!defs.length) return null;
  if (originDocument.uri.toString() !== defDocument.uri.toString() || currentOffset < 0) {
    return defs[0];
  }
  const scoped = defs
    .filter(d => d.scopeStart <= currentOffset && currentOffset <= d.scopeEnd)
    .sort((a, b) => (a.scopeEnd - a.scopeStart) - (b.scopeEnd - b.scopeStart));
  return scoped[0] || defs.find(d => d.scopeStart == null) || defs[0];
}

async function localReferences(document, position) {
  const symbol = templateSymbolAt(document, position);
  if (!symbol) return [];
  const name = symbol.name.split(".")[0];
  const docs = await relatedTemplateDocuments(document);
  const refs = [];
  const re = new RegExp(`\\b${escapeRegExp(name)}\\b`, "g");
  for (const doc of docs) {
    const text = doc.getText();
    for (const m of text.matchAll(re)) {
      refs.push(new vscode.Location(doc.uri, new vscode.Range(doc.positionAt(m.index), doc.positionAt(m.index + name.length))));
    }
  }
  return refs;
}

function collectTemplateDefinitions(document) {
  const text = document.getText();
  const defs = [];
  const add = (name, index, length, detail, scopeStart = null, scopeEnd = null) => {
    defs.push({
      name,
      uri: document.uri,
      range: new vscode.Range(document.positionAt(index), document.positionAt(index + length)),
      detail,
      scopeStart,
      scopeEnd
    });
  };
  for (const m of text.matchAll(/@(component|define|block|fill|render)\s*\(\s*["']([^"']+)["']/g)) {
    add(m[2], m.index + m[0].lastIndexOf(m[2]), m[2].length, `@${m[1]} "${m[2]}"`);
  }
  for (const comp of componentDeclarations(text)) {
    for (const prop of comp.props) {
      const name = prop.alias || prop.name;
      add(name, prop.index, name.length, `@component "${comp.name}" prop ${prop.name}${prop.alias ? ` as ${prop.alias}` : ""}${prop.defaultValue ? ` = ${prop.defaultValue}` : ""}`, comp.bodyStart, comp.bodyEnd);
    }
  }
  for (const m of text.matchAll(/@(let|computed|signal|handler)\s*\(\s*([A-Za-z_][\w]*)/g)) {
    add(m[2], m.index + m[0].lastIndexOf(m[2]), m[2].length, `@${m[1]} ${m[2]}`);
  }
  for (const m of text.matchAll(/@for\s*\(\s*(?:([A-Za-z_][\w]*)\s*,\s*)?([A-Za-z_][\w]*)\s+in\s+([^)]+)\)/g)) {
    if (m[1]) add(m[1], m.index + m[0].indexOf(m[1]), m[1].length, "loop key");
    add(m[2], m.index + m[0].lastIndexOf(m[2]), m[2].length, "loop value");
  }
  return defs;
}

async function relatedTemplateDocuments(document) {
  const docs = new Map([[document.uri.toString(), document]]);
  const queue = [document];
  for (let i = 0; i < queue.length && i < 50; i++) {
    for (const link of templateLinks(queue[i])) {
      if (docs.has(link.target.toString())) continue;
      try {
        const doc = await vscode.workspace.openTextDocument(link.target);
        docs.set(doc.uri.toString(), doc);
        queue.push(doc);
      } catch {}
    }
  }
  const patterns = ["**/*.spl", "**/*.spl.html", "**/*.tmpl", "**/*.html"];
  for (const pattern of patterns) {
    let uris = [];
    try {
      uris = await vscode.workspace.findFiles(pattern, "**/{node_modules,.git}/**", 200);
    } catch {}
    for (const uri of uris) {
      if (docs.has(uri.toString())) continue;
      try {
        const doc = await vscode.workspace.openTextDocument(uri);
        docs.set(doc.uri.toString(), doc);
      } catch {}
    }
  }
  return [...docs.values()];
}

function componentDeclarations(text) {
  const out = [];
  const re = /@component\s*\(/g;
  for (const m of text.matchAll(re)) {
    const open = m.index + m[0].length - 1;
    const close = findMatching(text, open, "(", ")");
    if (close < 0) continue;
    const inner = text.slice(open + 1, close);
    const parts = splitTopLevel(inner, ",");
    if (!parts.length) continue;
    const name = unquote(parts[0].text.trim());
    if (!name) continue;
    const bodyOpen = findNextNonWhitespace(text, close + 1);
    const bodyClose = bodyOpen >= 0 && text[bodyOpen] === "{" ? findMatching(text, bodyOpen, "{", "}") : -1;
    const props = [];
    for (const part of parts.slice(1)) {
      const parsed = parseComponentProp(part.text);
      if (!parsed) continue;
      const localName = parsed.alias || parsed.name;
      const relative = part.text.indexOf(localName);
      props.push({ ...parsed, index: open + 1 + part.start + Math.max(relative, 0) });
    }
    out.push({ name, props, bodyStart: bodyOpen >= 0 ? bodyOpen : close, bodyEnd: bodyClose >= 0 ? bodyClose : text.length });
  }
  return out;
}

function findNextNonWhitespace(text, start) {
  for (let i = start; i < text.length; i++) {
    if (!/\s/.test(text[i])) return i;
  }
  return -1;
}

function parseComponentProp(raw) {
  const eq = findTopLevelAssign(raw);
  const left = (eq >= 0 ? raw.slice(0, eq) : raw).trim();
  const defaultValue = eq >= 0 ? raw.slice(eq + 1).trim() : "";
  const asMatch = /^([A-Za-z_][\w-]*)\s+as\s+([A-Za-z_][\w-]*)$/.exec(left);
  if (asMatch) return { name: asMatch[1], alias: asMatch[2], defaultValue };
  const nameMatch = /^([A-Za-z_][\w-]*)$/.exec(left);
  return nameMatch ? { name: nameMatch[1], alias: "", defaultValue } : null;
}

function splitTopLevel(input, sep) {
  const out = [];
  let start = 0, depth = 0, quote = "";
  for (let i = 0; i < input.length; i++) {
    const ch = input[i];
    if (quote) {
      if (ch === "\\") i++;
      else if (ch === quote) quote = "";
      continue;
    }
    if (ch === "\"" || ch === "'" || ch === "`") quote = ch;
    else if (ch === "(" || ch === "{" || ch === "[") depth++;
    else if (ch === ")" || ch === "}" || ch === "]") depth--;
    else if (ch === sep && depth === 0) {
      out.push({ text: input.slice(start, i), start });
      start = i + 1;
    }
  }
  out.push({ text: input.slice(start), start });
  return out;
}

function findTopLevelAssign(input) {
  let depth = 0, quote = "";
  for (let i = 0; i < input.length; i++) {
    const ch = input[i];
    if (quote) {
      if (ch === "\\") i++;
      else if (ch === quote) quote = "";
      continue;
    }
    if (ch === "\"" || ch === "'" || ch === "`") quote = ch;
    else if (ch === "(" || ch === "{" || ch === "[") depth++;
    else if (ch === ")" || ch === "}" || ch === "]") depth--;
    else if (ch === "=" && depth === 0 && input[i - 1] !== "!" && input[i - 1] !== "<" && input[i - 1] !== ">" && input[i + 1] !== "=") return i;
  }
  return -1;
}

function findMatching(text, open, left, right) {
  let depth = 0, quote = "";
  for (let i = open; i < text.length; i++) {
    const ch = text[i];
    if (quote) {
      if (ch === "\\") i++;
      else if (ch === quote) quote = "";
      continue;
    }
    if (ch === "\"" || ch === "'" || ch === "`") quote = ch;
    else if (ch === left) depth++;
    else if (ch === right) {
      depth--;
      if (depth === 0) return i;
    }
  }
  return -1;
}

function unquote(value) {
  const s = value.trim();
  if ((s.startsWith("\"") && s.endsWith("\"")) || (s.startsWith("'") && s.endsWith("'"))) return s.slice(1, -1);
  return s;
}

function fsExists(file) {
  try {
    return require("fs").existsSync(file);
  } catch {
    return false;
  }
}

function escapeRegExp(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function readMessages(chunk) {
  const key = "server";
  buffers.set(key, Buffer.concat([buffers.get(key) || Buffer.alloc(0), chunk]));
  let buf = buffers.get(key);
  while (buf.length) {
    const headerEnd = buf.indexOf("\r\n\r\n");
    if (headerEnd < 0) break;
    const header = buf.slice(0, headerEnd).toString("utf8");
    const match = /Content-Length:\s*(\d+)/i.exec(header);
    if (!match) break;
    const length = Number(match[1]);
    const start = headerEnd + 4;
    if (buf.length < start + length) break;
    const message = JSON.parse(buf.slice(start, start + length).toString("utf8"));
    buf = buf.slice(start + length);
    const entry = pending.get(message.id);
    if (entry) {
      pending.delete(message.id);
      message.error ? entry.reject(new Error(message.error.message)) : entry.resolve(message.result);
    }
  }
  buffers.set(key, buf);
}

function buildSemanticTokens(document) {
  const builder = new vscode.SemanticTokensBuilder(semanticLegend);
  const text = document.getText();
  const tokens = [];
  const push = (index, length, type) => {
    if (length <= 0) return;
    const start = document.positionAt(index);
    const end = document.positionAt(index + length);
    if (start.line !== end.line) return;
    tokens.push({ line: start.line, character: start.character, length, type });
  };

  for (const m of text.matchAll(/@\/\/.*$/gm)) push(m.index, m[0].length, "comment");
  for (const m of text.matchAll(/@(if|elseif|else|for|empty|switch|case|default|match|raw|include|import|handler|extends|block|define|component|render|slot|fill|let|computed|watch|signal|bind|effect|reactive|click|stream|defer|lazy|fallback)\b/g)) {
    push(m.index, m[0].length, "keyword");
  }
  for (const m of text.matchAll(/\$\{[^}\n]*(?:\}|\n|$)/g)) {
    const start = m.index;
    const expr = m[0];
    push(start, Math.min(2, expr.length), "operator");
    if (expr.endsWith("}")) push(start + expr.length - 1, 1, "operator");
    for (const raw of expr.matchAll(/\braw\b/g)) push(start + raw.index, raw[0].length, "keyword");
    for (const f of expr.matchAll(/\|\s*([A-Za-z_][A-Za-z0-9_]*)/g)) {
      push(start + f.index, 1, "operator");
      push(start + f.index + f[0].lastIndexOf(f[1]), f[1].length, "function");
    }
    for (const op of expr.matchAll(/==|!=|<=|>=|\|\||&&|[+\-*/%<>!]/g)) push(start + op.index, op[0].length, "operator");
    for (const n of expr.matchAll(/\b\d+(?:\.\d+)?\b/g)) push(start + n.index, n[0].length, "number");
    for (const v of expr.matchAll(/\b[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*\b/g)) {
      if (["raw", "true", "false", "null", "nil"].includes(v[0])) continue;
      push(start + v.index, v[0].length, "variable");
    }
  }
  for (const m of text.matchAll(/\bdata-spl-(?:model|bind(?:-[A-Za-z0-9_-]+)?|on-[A-Za-z0-9_-]+|api-[A-Za-z0-9_-]+|if|else|ref|hydration|runtime)\b/g)) {
    push(m.index, m[0].length, "property");
  }
  tokens.sort((a, b) => a.line - b.line || a.character - b.character || b.length - a.length);
  let lastLine = -1;
  let lastEnd = -1;
  for (const token of tokens) {
    if (token.line === lastLine && token.character < lastEnd) continue;
    builder.push(token.line, token.character, token.length, token.type, 0);
    lastLine = token.line;
    lastEnd = token.character + token.length;
  }
  return builder.build();
}

module.exports = { activate, deactivate };
