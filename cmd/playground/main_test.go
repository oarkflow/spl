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

		engine := template.New()
		engine.AutoEscape = true
		engine.SecureMode = false
		out, err := engine.RenderSSR(tmpl, data)
		if err != nil {
			t.Fatalf("%s: render failed: %v", name, err)
		}
		if strings.TrimSpace(out) == "" {
			t.Fatalf("%s: rendered empty output", name)
		}
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
