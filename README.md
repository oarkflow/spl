# SPL Template Engine

**Server-Powered Logic** — a Go template engine with SSR hydration, reactive signals, streaming, components, filters, and CSP-safe client interactivity.

> **Module:** `github.com/oarkflow/spl` · **Go:** 1.26.1
> **Dependency:** `github.com/oarkflow/interpreter v0.0.11`

---

## Table of Contents

- [Quick Start](#quick-start)
- [Template Language](#template-language)
  - [Interpolation](#interpolation)
  - [Filters](#filters)
  - [Variables & State](#variables--state)
  - [Conditionals](#conditionals)
  - [Loops](#loops)
  - [Pattern Matching](#pattern-matching)
  - [Components & Slots](#components--slots)
  - [Layout System](#layout-system)
  - [Includes & Imports](#includes--imports)
  - [Streaming & Lazy Loading](#streaming--lazy-loading)
  - [Event Handlers](#event-handlers)
  - [Reactive Primitives](#reactive-primitives)
  - [Declarative API Calls](#declarative-api-calls)
  - [Data Binding Attributes](#data-binding-attributes)
- [Engine API](#engine-api)
  - [Engine Configuration](#engine-configuration)
  - [Engine Methods](#engine-methods)
  - [SSR & Hydration](#ssr--hydration)
  - [Streaming API](#streaming-api)
  - [Secure Mode](#secure-mode)
  - [Cache Management](#cache-management)
  - [Hot Reload](#hot-reload)
  - [Adapter for interpreter](#adapter-for-interpreter)
- [Built-in Filters Reference](#built-in-filters-reference)
- [Hydration Runtime API](#hydration-runtime-api)
- [Examples](#examples)
- [VS Code Extension](#vs-code-extension)

---

## Quick Start

```go
package main

import (
    "fmt"
    spl "github.com/oarkflow/spl"
)

func main() {
    engine := spl.New()
    engine.Globals["siteName"] = "My App"

    out, err := engine.Render(`<h1>Hello, ${name | upper}!</h1>`, map[string]any{
        "name": "world",
    })
    if err != nil {
        panic(err)
    }
    fmt.Println(out)
    // <h1>Hello, WORLD!</h1>
}
```

### With SSR + Hydration

```go
engine.SecureMode = true // CSP-safe output
out, err := engine.RenderSSR(`
    @signal(count = 1)
    <div>@bind(count)</div>
    @click("+1", count, "inc", "1")
    @reactive(count) { <span>${count}</span> }
`, nil)
```

---

## Template Language

### Interpolation

Expression syntax: `${expr}` — evaluates any SPL expression and inserts the result.

```
<p>Hello, ${user.Name}!</p>
<p>${count + 1} items</p>
<p>${"direct string" | upper}</p>
```

Filters are applied via the pipe `|` operator:

```
${name | upper}
${name | truncate 20 "..."}
${value | default "N/A"}
${num | format "%.2f"}
```

### Filters

26 built-in filters. See [Built-in Filters Reference](#built-in-filters-reference).

```
<p>${"hello" | upper}</p>              <!-- HELLO -->
<p>${"hello" | capitalize}</p>         <!-- Hello -->
<p>${"hello world"|slug}</p>           <!-- hello-world -->
<p>${"Hi\nThere"|nl2br}</p>            <!-- Hi<br>There -->
<p>${"abc" | repeat 3}</p>             <!-- abcabcabc -->
<p>${"hello" | padend 10 "-"}</p>      <!-- hello----- -->
```

### Variables & State

#### `@let` — server-side computed variable

```
@let(total = len(items))
@let(discount = price * 0.9)
```

#### `@computed` — derived value (same semantics as `@let`, expresses intent)

```
@computed(taxRate = 0.08)
@computed(finalPrice = subtotal * (1 + taxRate))
```

#### `@computedClient` — hydrated derived signal

```
@signal(first = "Ada")
@signal(last = "Lovelace")
@computedClient(fullName = first + " " + last, first, last)
<p>@bind(fullName)</p>
```

#### `@signal` — reactive client-side state (SSR hydrated)

```
@signal(count = 0)
@signal(user = {"name": "Alice", "role": "admin"})
@signal(items = [])
```

Signals are serialized into the hydration payload and become live JavaScript variables on the client. Changes automatically update all reactive bindings.

#### `@local` — component-instance signal

```
@component("Counter") {
  @local(count = 0)
  <button on:click="count += 1">@bind(count)</button>
}
```

Each component render receives its own hydrated signal name, so reusable interactive components do not share accidental state.

#### `@raw` — literal block (no parsing)

```
@raw {
    This ${won't} be @parsed at all.
}
```

### Conditionals

#### `@if` / `@elseif` / `@else`

```
@if(user.role == "admin") {
    <a href="/admin">Admin Panel</a>
} @elseif(user.role == "editor") {
    <a href="/editor">Editor</a>
} @else {
    <span>Guest</span>
}
```

SSR with hydration: `@if` inside `@reactive` generates `data-spl-if` / `data-spl-else` markers for client-side reactivation.

#### Conditional fragments

```
{change.breakingNote != '' && (
  <div class="breaking">${change.breakingNote}</div>
)}
{fallbackVisible || <span>No fallback needed</span>}
```

### Loops

#### `@for`

```
<ul>
@for(item in items) {
    <li>${item.name | title}</li>
}
</ul>
```

With key and value:

```
@for(i, item in items) {
    <li>#${i+1}: ${item}</li>
}
```

With a stable hydration key:

```
@for(item in todos; key item.id) {
    <article>${item.title}</article>
}
```

The `$loop` variable is available inside `@for`:

```
@for(item in items) {
    <li class="${loop.first ? "first" : ""} ${loop.last ? "last" : ""}">
        ${item} (${loop.index} of ${loop.length})
    </li>
}
```

`$loop` properties: `index` (0-based), `index1` (1-based), `first`, `last`, `length`.

#### `@for` with `@empty`

```
@for(x in []) {
    <li>${x}</li>
} @empty {
    <p class="empty">This list is empty.</p>
}
```

### Pattern Matching

#### `@match` / `@case` (pattern matching via interpreter)

```
@match(priority) {
    @case("low")    { <span class="badge">Low</span> }
    @case("medium") { <span class="badge">Medium</span> }
    @case("high")   { <span class="badge">High</span> }
    @default        { <span class="badge">Unknown</span> }
}
```

Patterns support: literals, type checks (`x: integer`), destructuring (`[a, b]`), comparison guards.

#### `@switch` / `@case` (string comparison)

```
@switch(status) {
    @case("pending")          { <span>Pending</span> }
    @case("shipped","in_transit") { <span>In Transit</span> }
    @case("delivered")        { <span>Delivered</span> }
    @default                  { <span>Unknown</span> }
}
```

### Components & Slots

#### Basic example

Define reusable components with declared props, compose them, and pass data via a variable:

```spl
@component("Badge", label string, color string = '#666') {
  <style>.badge { display: inline-block; padding: 2px 8px; border-radius: 12px; font-size: 12px; color: white; background: ${color}; }</style>
  <span class="badge">${label}</span>
}

@component("Card", title string, body string, tag string, tagColor string) {
  <style>.card { border: 1px solid #ddd; border-radius: 8px; padding: 16px; margin: 8px 0; }</style>
  <div class="card">
    <h3>${title} @render("Badge", {"label": tag, "color": tagColor})</h3>
    <p>${body}</p>
  </div>
}

<h1>Component Demo</h1>
@for (item in items) {
  @render("Card", item)
}
```

Declared component props are validated at render time. Props without a default and without a `?` prefix are required.

With data:

```json
{
  "items": [
    {"title": "Getting Started", "body": "Install SPL and run your first script.", "tag": "New", "tagColor": "#22c55e"},
    {"title": "Templates", "body": "Build dynamic HTML with SPL expressions.", "tag": "Guide", "tagColor": "#3b82f6"},
    {"title": "Filters", "body": "Transform output with 25+ built-in filters.", "tag": "Popular", "tagColor": "#ef4444"}
  ]
}
```

Components can be defined inline or in separate files loaded via `@import` or `RegisterComponent`. Declared props are matched by name from the passed hash or variable — no need to destructure.

#### Named slots

```
@component("Panel") {
    <div class="panel">
        <header>@slot("header")</header>
        <div>@slot</div>              <!-- default slot -->
        <footer>@slot("footer")</footer>
    </div>
}

@render("Panel") {
    @fill("header") { <h2>User Profile</h2> }
    <p>Name: Alice — Role: Admin</p>
    @fill("footer") { Last login: 2 hours ago }
}
```

#### Prop aliases & defaults

```
@component("InfoCard", title as heading = "Untitled", subtitle = "No description", color = "#3b82f6") {
    <div style="border:2px solid ${color};">
        <h4 style="color:${color};">${heading}</h4>
        <p>${subtitle}</p>
    </div>
}

@render("InfoCard", {"title": "Custom Card", "subtitle": "All props set", "color": "#ef4444"})
@render("InfoCard", {"title": "Partial Props"})
@render("InfoCard")       <!-- uses defaults -->
```

#### Optional props (`?`)

Mark a prop as optional with `?` — when not passed, it resolves to `undefined` instead of throwing:

```
@component("Badge", label, ?color) {
  <span>${label} - ${color}</span>
}
@render("Badge", {"label": "hello"})   <!-- color is undefined → renders as "" -->
```

#### Registering components programmatically

```go
engine.RegisterComponent("Alert", `<div class="alert alert-${type}">@slot</div>`)
```

### Layout System

#### `@extends` / `@block` / `@define`

**layout.html:**
```html
<!DOCTYPE html>
<html>
<body>
    <header>My Site</header>
    <main>@block("content"){<p>Default content</p>}</main>
</body>
</html>
```

**page.html:**
```html
@extends("layout.html")
@define("content") {
    <h1>${title}</h1>
    <p>${body}</p>
}
```

### Includes & Imports

#### `@include`

Renders another template inline with optional scoped data:

```
@include("header.html")
@include("sidebar.html", data)
```

#### `@import`

Loads component definitions from another template file into the current scope:

```
@import("components/card.html")
@import("components/field.html")

@render("Card", ...) { ... }
```

### Streaming & Lazy Loading

#### `@stream`

Renders content immediately and flushes the writer:

```
@stream {
    <p>This content is flushed to the client immediately.</p>
}
```

#### `@defer` / `@fallback`

Renders fallback content immediately, then replaces it with the deferred content asynchronously:

```
@defer {
    <p>This loaded asynchronously after page render.</p>
} @fallback {
    <p>Loading...</p>
}
```

#### `@lazy` / `@fallback`

Conditionally renders content based on a server-side expression:

```
@lazy(item.ready) {
    <span class="ready">Ready</span>
} @fallback {
    <span class="pending">Pending</span>
}
```

### Event Handlers

#### `@handler` — named server-registered handler

```
@handler(resetAll) {
    form.name='';form.email='';form.age=25;
}
```

Referenced as `on:click="resetAll"`. In secure mode, handler bodies compile to safe structured actions (only `=`, `+=`, `-=`, `toggle()`, `setSignal()`).

#### `on:*` attributes

```
<button on:click="tabForms=true;tabInteractive=false">Forms</button>
<button on:click="counter += 1">+1</button>
<button on:click="toggle(panelOpen)">Toggle</button>
<input on:input="debounce(query = value, 250)" />
```

In SSR mode, `on:click="..."` is transformed to `data-spl-on-click="..."`.

#### `@click` — declarative click button

```
@click("+1", counter, "inc", "1")
@click("Toggle", panelOpen, "toggle")
```

### Reactive Primitives

#### `@reactive(deps...) { ... }`

Renders a reactive view on the server; wraps in `data-spl-view` for client-side reactivation. Any `${signal}` references become live subscriptions.

```
@reactive(counter) {
    <p>Count: <strong>${counter}</strong></p>
}
@reactive(personal, prefs) {
    <div>${personal.name} — ${prefs.role}</div>
}
```

#### `@effect(deps...) { ... }`

Tracks dependencies and re-renders when signals change:

```
@effect(counter) {
    <div class="effect">Counter is ${counter}</div>
}
```

#### `@bind(signal)`

Renders a live-readable signal value with two-way binding:

```
<p>@bind(counter)</p>
```

In SSR → outputs `<span data-spl-bind="counter" data-spl-attr="textContent">0</span>`.

#### `@watch(expr) { ... }`

Renders body only when the watched expression value changes:

```
@watch(theme) {
    <link rel="stylesheet" href="/css/${theme}.css" />
}
```

### Declarative API Calls

The `data-spl-api-*` attributes enable REST calls without JavaScript.

```
<button
  data-spl-api-url="/api/quote"
  data-spl-api-method="GET"
  data-spl-api-target="quote"
  data-spl-api-parse="json"
>Load Quote</button>
```

**Attributes:**

| Attribute | Description |
|---|---|
| `data-spl-api-url` | API endpoint (supports `${}` template expressions) |
| `data-spl-api-method` | HTTP method: `GET`, `POST`, `PUT`, `DELETE` |
| `data-spl-api-target` | Signal name to write the response into |
| `data-spl-api-parse` | Parse mode: `auto`, `json`, `text`, `html` |
| `data-spl-api-body` | JSON body template (uses `{{signal}}` interpolation) |
| `data-spl-api-form` | `"closest"` to serialize the closest `<form>` as JSON body |
| `data-spl-api-event` | Trigger event: `click` (default), `load`, `submit` |
| `data-spl-api-reset` | Comma-separated signal names to clear after success |
| `data-spl-api-content-type` | Content-Type override (default: `application/json`) |

**Form POST example:**

```html
<form>
  <input type="text" name="title" data-spl-model="todoDraft" />
  <input type="hidden" name="priority" value="medium" />
  <button type="submit"
    data-spl-api-method="POST"
    data-spl-api-url="/api/todos"
    data-spl-api-form="closest"
    data-spl-api-target="todoResult"
    data-spl-api-reset="todoDraft"
  >Create</button>
</form>
```

### Data Binding Attributes

| Attribute | Description |
|---|---|
| `data-spl-model="signal"` | Two-way form field binding (input, select, textarea, checkbox, radio) |
| `data-spl-bind="signal"` | Display binding for textContent |
| `data-spl-if="signal"` | Conditional visibility (client-side) |
| `data-spl-else` | Inverse of previous `data-spl-if` |
| `data-spl-bind-{attr}="signal"` | Attribute binding (e.g. `data-spl-bind-class`) |
| `data-spl-ref="name"` | Element reference accessible as `SPL.refs.name` |
| `data-spl-attr="attrName"` | Which attribute to bind (used with `data-spl-bind`) |
| `data-spl-on-{event}="..."` | Compiled event handler (output from `on:*` transformation) |
| `bind:value="signal"` | Shorthand transformed to `data-spl-bind-value` |
| `class:name="signal"` | Toggle a class reactively |
| `style:property="signal"` | Update an inline style property reactively |
| `data-spl-form-state="signal"` | Publish form validity, errors, touched, and dirty fields |
| `data-spl-error-for="field"` | Render the current validation message for a form field |

### Asset Policy

```
@assets("dedupe")
@assets("raw")
```

Rendered CSS/JS/link assets are deduplicated by default. Use `raw` when a template intentionally needs repeated asset tags.

---

## Engine API

### Engine Configuration

```go
type Engine struct {
    BaseDir             string   // directory for resolving includes/layouts
    Filters             map[string]Filter
    Globals             map[string]any   // merged into every render
    AutoEscape          bool             // HTML-escape ${} output (default: true)
    MaxDepth            int              // max include/layout nesting (default: 64)
    Components          map[string]componentDef
    HydrationRuntimeURL string           // external JS runtime URL
    CSPNonce            string           // nonce for hydration <script> tags
    SecureMode          bool             // CSP-safe, non-eval hydration (default: false)
    DisableDebug        bool             // exclude debug from runtime
    DisableAPI          bool             // exclude API from runtime
}
```

### Engine Methods

```go
func New() *Engine
    // Creates engine with sensible defaults and registers 26 built-in filters.

func (e *Engine) RegisterFilter(name string, fn Filter)
    // Adds or replaces a named filter. Filter = func(value any, args ...string) string

func (e *Engine) RegisterComponent(name string, body string) error
    // Parses and registers a component body.

func (e *Engine) Render(tmpl string, data map[string]any) (string, error)
    // Parses and renders a template string with the given data.

func (e *Engine) RenderFile(path string, data map[string]any) (string, error)
    // Loads template file relative to BaseDir and renders it.

func (e *Engine) RenderSSRFile(path string, data map[string]any) (string, error)
    // SSR render with hydration metadata injection.

func (e *Engine) RenderStreamFile(w io.Writer, path string, data map[string]any) error
    // Streams a template file to a writer.

func (e *Engine) InvalidateCaches()
    // Clears all parsed template and expression caches.

func (e *Engine) CacheStats() CacheStats
    // Returns cache entry counts (ParsedFiles, ParsedTemplates, CompiledFiles, etc.)

func (e *Engine) RuntimeJS() string
    // Returns obfuscated hydration runtime JS (all features included).

func (e *Engine) RuntimeJSRaw() string
    // Returns unobfuscated, minified hydration runtime for debugging.
```

### SSR & Hydration

```go
// From string
out, err := engine.RenderSSR(tmpl, data)

// From file
out, err := engine.RenderSSRFile(path, data)

// With explicit renderer
ssr := spl.NewSSRRenderer(engine, data)
out, err := ssr.RenderSSR(tmpl)
```

SSR rendering:
1. Parses template into AST
2. Evaluates expressions server-side
3. Wraps reactive elements with `data-spl-*` markers
4. Builds a hydration payload (`<script type="application/json" data-spl-hydration>`) containing signals, handlers, effects, and views
5. Injects the hydration runtime (inline or via `HydrationRuntimeURL`)
6. In secure mode, validates all expressions and blocks unsafe patterns

**Hydration payload structure:**
```json
{
  "signals": { "count": 0, "user": {"name":"Alice"} },
  "handlers": { "resetAll": [{"kind":"set","target":"name","value":""}] },
  "effects": [{"Selector":"[data-spl-effect=\"1\"]","Source":"<span>...</span>","Deps":["count"]}],
  "views": [{"Selector":"[data-spl-view=\"1\"]","Source":"<div>...</div>","Deps":["user"]}],
  "secure": true
}
```

### Streaming API

```go
type StreamRenderer struct { ... }

func NewStreamRenderer(engine *Engine, data map[string]any) *StreamRenderer

func (sr *StreamRenderer) RenderStream(w io.Writer, tmpl string) error
    // Renders template to writer with streaming semantics.

func (sr *StreamRenderer) RenderStreamNodes(w io.Writer, nodes []Node) error
    // Renders pre-parsed nodes to writer.

func (sr *StreamRenderer) RenderStreamToString(tmpl string) (string, error)
    // Streaming semantics returning string.

type StreamChunk struct {
    ID      string
    Content string
    IsDefer bool
    Error   error
}
```

### Secure Mode

When `engine.SecureMode = true`:

1. **Handler body validation:** only simple assignments (`x = v`), arithmetic mutations (`x += n`, `x -= n`), `toggle(x)`, and `setSignal(x, v)` are allowed. No `var`, `function`, `ref()`, `document.querySelectorAll`, or object literals in handler bodies.
2. **Output security:** rendered HTML is checked against forbidden patterns:
   - `<script>` tags
   - Inline event handlers (`onclick=`, `onload=`, etc.)
   - `javascript:` URLs in `href`, `src`, `xlink:href`, `formaction`
   - `srcdoc` attribute
   - `<iframe>`, `<object>`, `<embed>` elements
   - `<meta http-equiv="refresh">`
3. **Event attribute compilation:** `on:click="..."` is compiled to structured JSON actions instead of eval'd strings.
4. **Debounced events** also use structured JSON specs.

```go
engine.SecureMode = true
out, err := engine.RenderSSR(tmpl, data)
// safe for CSP with no inline eval
```

### Cache Management

```go
engine.InvalidateCaches()
// CacheStats() returns:
type CacheStats struct {
    ParsedFiles       int
    ParsedTemplates   int
    CompiledFiles     int
    CompiledTemplates int
    ExprPrograms      int
    ExprFastPaths     int
    Components        int
    GlobalEnvReady    bool
}
```

**Cache size limits:**
| Cache | Default Max |
|---|---|
| Parsed template files | 1000 |
| Parsed template strings | 500 |
| Compiled files | 500 |
| Compiled strings | 500 |
| Expression ASTs | 10000 |
| Expression metadata | 10000 |

Eviction: random 20% eviction when limit is exceeded.

### Hot Reload

The engine auto-registers with the `interpreter` package's hot reload hook:

```go
// On file change notification, all engine instances invalidate their caches:
interpreter.RegisterHotReloadHook(func(path string) {
    // iterates all registered engines and calls InvalidateCaches()
})
```

### Adapter for interpreter

The package auto-registers a `TemplateRuntime` factory with `github.com/oarkflow/interpreter`:

```go
// Registered in init():
interpreter.RegisterTemplateRuntimeFactory(func(baseDir string) interpreter.TemplateRuntime {
    engine := spl.New()
    engine.BaseDir = baseDir
    return &runtimeAdapter{engine: engine}
})
```

The adapter implements:
- `Render(tmpl, data)` → `engine.Render(tmpl, data)`
- `RenderFile(path, data)` → `engine.RenderFile(path, data)`
- `RenderSSR(tmpl, data)` → `engine.RenderSSR(tmpl, data)`
- `RenderSSRFile(path, data)` → `engine.RenderSSRFile(path, data)`
- `RenderStream(w, tmpl, data)` → streaming
- `RenderStreamFile(w, path, data)` → streaming
- `InvalidateCaches()` → cache clear

---

## Built-in Filters Reference

| Filter | Description | Example |
|---|---|---|
| `upper` | Uppercase | `"hello" \| upper` → `HELLO` |
| `lower` | Lowercase | `"HELLO" \| lower` → `hello` |
| `trim` | Trim whitespace | `"  hi  " \| trim` → `hi` |
| `title` | Title-case each word | `"hello world" \| title` → `Hello World` |
| `capitalize` | Uppercase first char | `"hello" \| capitalize` → `Hello` |
| `escape` | HTML-escape | `"<br>" \| escape` → `&lt;br&gt;` |
| `json` | JSON marshal | `user \| json` |
| `format` | `fmt.Sprintf` | `3.14159 \| format "%.2f"` → `3.14` |
| `default` | Fallback if empty | `"" \| default "N/A"` → `N/A` |
| `join` | Join array | `["a","b"] \| join ", "` → `a, b` |
| `first` | First character | `"abc" \| first` → `a` |
| `last` | Last character | `"abc" \| last` → `c` |
| `length` | Rune length | `"hello" \| length` → `5` |
| `reverse` | Reverse string | `"abc" \| reverse` → `cba` |
| `truncate` | Truncate to N chars | `"long text" \| truncate 3` → `lon...` |
| `nl2br` | Newlines to `<br>` | `"a\nb" \| nl2br` → `a<br>b` |
| `urlencode` | URL query escape | `"a b" \| urlencode` → `a+b` |
| `striptags` | Strip HTML tags | `"<b>Hi</b>" \| striptags` → `Hi` |
| `slug` | URL-friendly slug | `"Hello World!" \| slug` → `hello-world` |
| `replace` | Replace all | `"foo-bar" \| replace "-" " "` → `foo bar` |
| `split` | Split into JSON array | `"a,b" \| split` → `["a","b"]` |
| `repeat` | Repeat string | `"ab" \| repeat 3` → `ababab` |
| `padstart` | Left-pad | `"hi" \| padstart 5 "-"` → `---hi` |
| `padend` | Right-pad | `"hi" \| padend 5 "-"` → `hi---` |
| `wrap` | Wrap with prefix/suffix | `"hi" \| wrap "<b>" "</b>"` → `<b>hi</b>` |

Custom filters:

```go
engine.RegisterFilter("stars", func(val any, args ...string) string {
    n, _ := strconv.Atoi(fmt.Sprintf("%v", val))
    return strings.Repeat("★", n)
})
```

Template usage: `${rating | stars}`

---

## Hydration Runtime API

The client-side runtime (`SPL` namespace) provides:

| Function | Purpose |
|---|---|
| `SPL.read(name)` | Read a signal value |
| `SPL.write(name, value)` | Write a signal value (notifies subscribers) |
| `SPL.subscribe(name, fn)` | Subscribe to signal changes |
| `SPL.signalRef(name)` | Get signal object `{value, subscribers}` |
| `SPL.ensureSignal(name, initial)` | Create signal if not exists |
| `SPL.registerHandler(name, fn)` | Register a handler function |
| `SPL.interpolate(source)` | Replace `${}` placeholders with signal values |
| `SPL.resolveTemplate(source)` | Resolve template expressions in attribute values |
| `SPL.readPath(path)` | Deep read: `SPL.readPath("user.name")` |
| `SPL.writePath(path, value)` | Deep write: `SPL.writePath("user.name", "Alice")` |
| `SPL.debugRecord(kind, key)` | Record debug statistics |
| `SPL.getRenderStats()` | Get render performance stats |

---

## Examples

### Fiber Web App

Located at `examples/fiber/`:

```go
package main

import template "github.com/oarkflow/spl"

type SPLViews struct {
    engine    *template.Engine
    directory string
    extension string
    reload    bool
    ssr       bool
}
```

**Features demonstrated:**
- All 25+ template directives (`@signal`, `@reactive`, `@component`, `@for`, `@match`, `@switch`, `@lazy`, `@slot`/`@fill`, `@let`, `@computed`, `@handler`, `@click`, `@bind`, `@effect`, etc.)
- `data-spl-model`, `data-spl-api-*`, `data-spl-ref`, `data-spl-if`, `data-spl-bind`
- SSR hydration with `SecureMode = true`
- CSP security headers
- 4 API endpoints: `GET/POST /api/todos`, `GET /api/quote`, `POST /api/submit`
- Declarative API forms with `data-spl-api-form="closest"`
- 26 built-in filters
- Component system with named slots, prop aliases, and defaults
- Auto-escaping / XSS prevention

```bash
cd examples/fiber
go run .
# Open http://localhost:3000
```

---

## VS Code Extension

Syntax highlighting for `.spl` files is available as a VS Code extension in `vscode-extension/`.

```bash
make vscode-extension
# or
make install-extension
```

The extension provides:
- Full syntax highlighting for all SPL directives (`@if`, `@for`, `@component`, `@signal`, etc.)
- Interpolation highlighting (`${expr}`)
- Filter pipe highlighting (`| upper`)
- Comment highlighting (`@//`)

---

## License

`github.com/oarkflow/spl` — MIT
