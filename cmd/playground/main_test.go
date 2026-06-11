package main

import (
	"encoding/json"
	"strings"
	"testing"

	template "github.com/oarkflow/spl"
)

func TestBuiltinExamplesRender(t *testing.T) {
	for _, ex := range builtinExamples() {
		name, _ := ex["name"].(string)
		if strings.TrimSpace(name) == "" {
			t.Fatalf("example has empty name: %#v", ex)
		}

		tmpl, _ := ex["template"].(string)
		if strings.TrimSpace(tmpl) == "" {
			t.Fatalf("%s: template is empty", name)
		}

		dataText, _ := ex["data"].(string)
		data := map[string]any{}
		if strings.TrimSpace(dataText) != "" {
			if err := json.Unmarshal([]byte(dataText), &data); err != nil {
				t.Fatalf("%s: invalid data JSON: %v", name, err)
			}
		}

		schemaText, _ := ex["schema"].(string)

		engine := template.New()
		engine.AutoEscape = true
		engine.SecureMode = false

		if strings.TrimSpace(schemaText) != "" {
			var schemas map[string]any
			if err := json.Unmarshal([]byte(schemaText), &schemas); err != nil {
				t.Fatalf("%s: invalid schema JSON: %v", name, err)
			}
			for name, schemaDef := range schemas {
				if schemaMap, ok := schemaDef.(map[string]any); ok {
					if parsed, err := template.SchemaFromMap(schemaMap); err == nil {
						engine.SchemaRegistry.Register(name, parsed)
					}
				}
			}
		}

		out, err := engine.RenderSSR(tmpl, data)
		if err != nil {
			t.Fatalf("%s: render failed: %v", name, err)
		}
		if strings.TrimSpace(out) == "" {
			t.Fatalf("%s: rendered empty output", name)
		}
	}
}

func TestPlaygroundCacheExampleUsesPersistentFragmentCache(t *testing.T) {
	engine := newPlaygroundRenderEngine(".")
	engine.AutoEscape = false

	tmpl := `@cache("demo", "30") {cached:${value}} live:${value}`
	first, err := engine.RenderSSR(tmpl, map[string]any{"value": "first"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := engine.RenderSSR(tmpl, map[string]any{"value": "second"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(first, "@cache") || strings.Contains(second, "@cache") {
		t.Fatalf("cache directive markers should not render: first=%q second=%q", first, second)
	}
	if first != "cached:first live:first" {
		t.Fatalf("unexpected first render: %q", first)
	}
	if second != "cached:first live:second" {
		t.Fatalf("expected cached fragment to survive across playground renders, got %q", second)
	}
}

func TestPlaygroundCacheExampleWithProseApostropheParsesDirective(t *testing.T) {
	var tmpl string
	for _, ex := range builtinExamples() {
		if ex["name"] == "cache-directive" {
			tmpl, _ = ex["template"].(string)
			break
		}
	}
	if tmpl == "" {
		t.Fatal("cache-directive example not found")
	}

	engine := newPlaygroundRenderEngine(".")
	engine.AutoEscape = false
	out, err := engine.RenderSSR(tmpl, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, `@cache("demo", "30")`) {
		t.Fatalf("cache directive marker should not render: %q", out)
	}
	if !strings.Contains(out, "Cached fragment") || !strings.Contains(out, "Live (uncached)") {
		t.Fatalf("expected cached body and live content, got %q", out)
	}
}

func TestFormatAPIHTMLIndentsHydrationConditionals(t *testing.T) {
	src := `<div data-spl-if="items">
  <h2>Items (3 total)</h2>
  <div class="item">
    <strong>Widget</strong> - $9.99
  </div>
  </div><div data-spl-else="items" style="display:none">
  <p>No items available.</p>
  </div>`

	out := formatAPIHTML(src)
	if strings.Contains(out, `</div><div data-spl-else`) {
		t.Fatalf("expected adjacent hydration wrappers to be split, got %q", out)
	}
	required := []string{
		`<div data-spl-if="items">`,
		`  <h2>Items (3 total)</h2>`,
		`  <div class="item">`,
		`    <strong>Widget</strong> - $9.99`,
		`<div data-spl-else="items" style="display:none">`,
		`  <p>No items available.</p>`,
	}
	for _, want := range required {
		if !strings.Contains(out, want) {
			t.Fatalf("formatted output missing %q:\n%s", want, out)
		}
	}
}

func TestFormatAPIHTMLLeavesHydrationScriptUntouched(t *testing.T) {
	src := `<div><p>Hello</p></div><script>if (a < b) { console.log("<p>"); }</script>`
	out := formatAPIHTML(src)
	if !strings.Contains(out, `<script>if (a < b) { console.log("<p>"); }</script>`) {
		t.Fatalf("script content should be untouched, got %q", out)
	}
	if strings.Contains(out, `</div><script`) {
		t.Fatalf("expected script to start on a new line, got %q", out)
	}
}

func TestPracticalExamplesHaveDescriptions(t *testing.T) {
	for _, ex := range builtinExamples() {
		category, _ := ex["category"].(string)
		if category != "Practical Patterns" {
			continue
		}
		name, _ := ex["name"].(string)
		description, _ := ex["description"].(string)
		if strings.TrimSpace(description) == "" {
			t.Fatalf("%s: practical example should include a description", name)
		}
	}
}

func TestFormErrorsExampleControlsAreReactive(t *testing.T) {
	var tmpl string
	for _, ex := range builtinExamples() {
		if ex["name"] == "form-errors" {
			tmpl, _ = ex["template"].(string)
			break
		}
	}
	if tmpl == "" {
		t.Fatal("form-errors example not found")
	}

	required := []string{
		`data-spl-model="formValues.name"`,
		`data-spl-model="formValues.email"`,
		`data-spl-model="formValues.plan"`,
		`data-spl-model="formValues.teamSize"`,
		`data-spl-model="formValues.goals"`,
		`data-spl-model="formValues.accepted"`,
		`on:input="validateForm"`,
		`on:change="validateForm"`,
		`on:click="submitReactiveForm"`,
		`on:click="resetReactiveForm"`,
		`@reactive(formValues, formErrors, fieldClasses, errorSummary, errorCount, submitted, statusMessage)`,
	}
	for _, want := range required {
		if !strings.Contains(tmpl, want) {
			t.Fatalf("form-errors example is missing reactive marker %q", want)
		}
	}
}
