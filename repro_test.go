package spl

import (
	"strings"
	"testing"
)

func TestReproExactTemplate1(t *testing.T) {
	e := New()
	// Exact template from user's issue - components with CSS braces
	tmpl := `@component("Badge", label, color) {
  <style>.badge { display: inline-block; padding: 2px 8px; border-radius: 12px; font-size: 12px; color: white; background: ${color | default "#666"}; }</style>
  <span class="badge">${label}</span>
}

@component("Card", title, body, tag, tagColor) {
  <style>.card { border: 1px solid #ddd; border-radius: 8px; padding: 16px; margin: 8px 0; }</style>
  <div class="card">
    <h3>${title} @render("Badge", {"label": tag, "color": tagColor})</h3>
    <p>${body}</p>
  </div>
}

<h1>Component Demo</h1>
@render("Card", {"title": "Getting Started", "body": "Install SPL and run your first script.", "tag": "New", "tagColor": "#22c55e"})
@render("Card", {"title": "Templates", "body": "Build dynamic HTML with SPL expressions.", "tag": "Guide", "tagColor": "#3b82f6"})
@render("Card", {"title": "Filters", "body": "Transform output with 25+ built-in filters.", "tag": "Popular", "tagColor": "#ef4444"})`

	out, err := e.Render(tmpl, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Getting Started") {
		t.Fatalf("expected 'Getting Started' in output, got %q", out)
	}
	if !strings.Contains(out, `">New</span>`) {
		t.Fatalf("expected Badge rendered, got %q", out)
	}
	if !strings.Contains(out, `">Guide</span>`) {
		t.Fatalf("expected Badge 'Guide' rendered, got %q", out)
	}
	if !strings.Contains(out, `">Popular</span>`) {
		t.Fatalf("expected Badge 'Popular' rendered, got %q", out)
	}
	if got := strings.Count(out, `.card { border: 1px solid #ddd;`); got != 1 {
		t.Fatalf("expected duplicate Card CSS to render once, got %d occurrences in %q", got, out)
	}
	if got := strings.Count(out, `.badge--spl-`); got != 3 {
		t.Fatalf("expected 3 Badge style instances (different backgrounds), got %d in %q", got, out)
	}
}

func TestComponentAssetsDeduplicateIdenticalStyleScriptAndLink(t *testing.T) {
	e := New()
	tmpl := `@component("Widget", label) {
  <link rel="stylesheet" href="/widget.css">
  <style>.widget { color: red; }</style>
  <script>function bootWidget(){ return true; }</script>
  <div class="widget">${label}</div>
}
@render("Widget", {"label": "One"})
@render("Widget", {"label": "Two"})
@render("Widget", {"label": "Three"})`

	out, err := e.Render(tmpl, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.Count(out, `<link rel="stylesheet" href="/widget.css">`); got != 1 {
		t.Fatalf("expected duplicate link to render once, got %d in %q", got, out)
	}
	if got := strings.Count(out, `<style>.widget { color: red; }</style>`); got != 1 {
		t.Fatalf("expected duplicate style to render once, got %d in %q", got, out)
	}
	if got := strings.Count(out, `<script>function bootWidget(){ return true; }</script>`); got != 1 {
		t.Fatalf("expected duplicate script to render once, got %d in %q", got, out)
	}
}

func TestComponentAssetsUniqueStyleScopesSelectors(t *testing.T) {
	e := New()
	tmpl := `@component("Badge", label, color) {
  <style data-spl-unique="true">.badge { color: white; background: ${color}; } #badgeRoot { display: inline-block; } .asset { background-image: url("https://cdn.example.com/badge.svg#icon"); content: ".badge #badgeRoot"; }</style>
  <span id="badgeRoot" class="badge">${label}</span>
}
@render("Badge", {"label": "New", "color": "#22c55e"})
@render("Badge", {"label": "Hot", "color": "#ef4444"})`

	out, err := e.Render(tmpl, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.Count(out, `data-spl-unique="true"`); got != 2 {
		t.Fatalf("expected unique style per render, got %d in %q", got, out)
	}
	for _, want := range []string{
		`.badge--spl-u-1 { color: white; background: #22c55e; }`,
		`.badge--spl-u-2 { color: white; background: #ef4444; }`,
		`id="badgeRoot--spl-u-1" class="badge--spl-u-1"`,
		`id="badgeRoot--spl-u-2" class="badge--spl-u-2"`,
		`https://cdn.example.com/badge.svg#icon`,
		`content: ".badge #badgeRoot"`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected scoped unique asset output to contain %q, got %q", want, out)
		}
	}
}

func TestComponentAssetsDoNotDedupeDataScripts(t *testing.T) {
	e := New()
	tmpl := `@component("Payload", name) {
  <script type="application/json">{"name":"${name}"}</script>
  <div>${name}</div>
}
@render("Payload", {"name": "One"})
@render("Payload", {"name": "One"})`

	out, err := e.Render(tmpl, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.Count(out, `<script type="application/json">{"name":"One"}</script>`); got != 2 {
		t.Fatalf("expected data scripts to be preserved, got %d in %q", got, out)
	}
}

func TestComponentAssetsUniqueScriptIsIsolated(t *testing.T) {
	e := New()
	tmpl := `@component("Widget") {
  <script data-spl-unique="true">var mounted = true; function boot(){ return mounted; }</script>
  <div>Widget</div>
}
@render("Widget")
@render("Widget")`

	out, err := e.Render(tmpl, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.Count(out, `(function(){`); got != 2 {
		t.Fatalf("expected each unique script to be isolated, got %d wrappers in %q", got, out)
	}
	if !strings.Contains(out, `const SPL_UNIQUE_SCOPE = "spl-u-1";`) || !strings.Contains(out, `const SPL_UNIQUE_SCOPE = "spl-u-2";`) {
		t.Fatalf("expected unique script scopes in %q", out)
	}
}

func TestReproExactTemplate2(t *testing.T) {
	e := New()
	// Named slots with CSS braces in component
	tmpl := `@component("Panel") {
  <style>.panel { border: 1px solid #ccc; border-radius: 8px; overflow: hidden; margin: 12px 0; } .panel-header { background: #f0f0f0; padding: 8px 16px; font-weight: bold; border-bottom: 1px solid #ccc; } .panel-body { padding: 16px; } .panel-footer { background: #fafafa; padding: 8px 16px; font-size: 12px; color: #666; border-top: 1px solid #ccc; } .status-green { color: green; }</style>
  <div class="panel">
    <div class="panel-header">
      @slot("header")
    </div>
    <div class="panel-body">
      @slot
    </div>
    <div class="panel-footer">
      @slot("footer")
    </div>
  </div>
}

<h1>Named Slots Demo</h1>

@let(userName = "John Doe")
@let(role = "Developer")
@let(lastLogin = "2023-10-01")

@render("Panel") {
  @fill("header") { User Profile }
  <p>Name: ${userName}</p>
  <p>Role: ${role | title}</p>
  @fill("footer") { Last login: ${lastLogin} }
}

@render("Panel") {
  @fill("header") { System Status }
  <p class="status-green">All systems operational.</p>
  @fill("footer") { Updated just now }
}`

	out, err := e.Render(tmpl, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "User Profile") {
		t.Fatalf("expected 'User Profile' in header slot, got %q", out)
	}
	if !strings.Contains(out, "John Doe") {
		t.Fatalf("expected 'John Doe' in body, got %q", out)
	}
	if !strings.Contains(out, "System Status") {
		t.Fatalf("expected 'System Status' in second panel, got %q", out)
	}
}

func TestReproExactTemplate3(t *testing.T) {
	e := New()
	// @let, @computed, @for with CSS braces in style block
	tmpl := `<style>
table { border-collapse: collapse; width: 100%; }
th, td { padding: 8px; }
th { background: #f0f0f0; text-align: left; }
td:nth-child(2) { text-align: center; }
td:nth-child(3) { text-align: right; }
td:nth-child(4) { text-align: right; font-weight: bold; }
</style>
@let(name = "World")
@let(greeting = "Hello, " + name + "!")
<h1>${greeting}</h1>

<h2>Order Summary</h2>
@let(items = [{"name": "Widget", "price": 19.99, "qty": 3}, {"name": "Gadget", "price": 9.99, "qty": 1}])
<table>
  <tr><th>Item</th><th>Qty</th><th>Price</th><th>Total</th></tr>
@for(item in items) {
  @computed(lineTotal = item.price * item.qty)
  <tr><td>${item.name}</td><td>${item.qty}</td><td>$${item.price}</td><td>$${lineTotal}</td></tr>
}
</table>`

	out, err := e.Render(tmpl, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Hello, World!") {
		t.Fatalf("expected 'Hello, World!' in output, got %q", out)
	}
	if !strings.Contains(out, "Widget") {
		t.Fatalf("expected 'Widget' in output, got %q", out)
	}
	if !strings.Contains(out, "59.97") {
		t.Fatalf("expected lineTotal 59.97 in output, got %q", out)
	}
}
