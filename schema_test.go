package spl

import (
	"strings"
	"testing"
)

func TestSchemaFromMap(t *testing.T) {
	m := map[string]any{
		"title":       "User",
		"description": "A user profile",
		"type":        "object",
		"properties": map[string]any{
			"name": map[string]any{
				"type":        "string",
				"title":       "Full Name",
				"minLength":   float64(2),
				"maxLength":   float64(100),
				"description": "The user's full name",
			},
			"age": map[string]any{
				"type":    "integer",
				"title":   "Age",
				"minimum": float64(0),
				"maximum": float64(150),
			},
			"email": map[string]any{
				"type":   "string",
				"format": "email",
				"title":  "Email Address",
			},
			"role": map[string]any{
				"type": "string",
				"enum": []any{"admin", "user", "viewer"},
			},
			"active": map[string]any{
				"type":  "boolean",
				"title": "Active Status",
			},
		},
		"required": []any{"name", "email"},
	}

	s, err := schemaFromMap(m)
	if err != nil {
		t.Fatal(err)
	}
	if s.Title != "User" {
		t.Fatalf("expected title 'User', got %q", s.Title)
	}
	if s.Type != SchemaTypeObject {
		t.Fatalf("expected type object, got %q", s.Type)
	}
	if len(s.Properties) != 5 {
		t.Fatalf("expected 5 properties, got %d", len(s.Properties))
	}
	if !s.isRequired("name") {
		t.Fatal("expected 'name' to be required")
	}
	if s.isRequired("age") {
		t.Fatal("expected 'age' to not be required")
	}
	if s.Properties["email"].Format != "email" {
		t.Fatalf("expected email format, got %q", s.Properties["email"].Format)
	}
	if len(s.Properties["role"].Enum) != 3 {
		t.Fatalf("expected 3 enum values, got %d", len(s.Properties["role"].Enum))
	}
}

func TestSchemaValidation(t *testing.T) {
	m := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{
				"type":      "string",
				"minLength": float64(2),
				"maxLength": float64(10),
			},
			"age": map[string]any{
				"type":    "integer",
				"minimum": float64(0),
				"maximum": float64(150),
			},
			"email": map[string]any{
				"type":    "string",
				"pattern": "^[a-z0-9._%+\\-]+@[a-z0-9.\\-]+\\.[a-z]{2,}$",
			},
		},
		"required": []any{"name"},
	}
	s, _ := schemaFromMap(m)

	errs := s.ValidateAll(map[string]any{
		"name":  "A",
		"age":   float64(200),
		"email": "invalid",
	})
	if len(errs) == 0 {
		t.Fatal("expected validation errors")
	}
	if _, ok := errs["name"]; !ok {
		t.Fatal("expected error for 'name' being too short")
	}
	if _, ok := errs["age"]; !ok {
		t.Fatal("expected error for 'age' being too large")
	}
	if _, ok := errs["email"]; !ok {
		t.Fatal("expected error for 'email' pattern mismatch")
	}

	errs = s.ValidateAll(map[string]any{
		"name":  "Alice",
		"age":   float64(30),
		"email": "alice@example.com",
	})
	if len(errs) != 0 {
		t.Fatalf("expected no validation errors, got %v", errs)
	}
}

func TestSchemaRenderFormHTML(t *testing.T) {
	m := map[string]any{
		"type":  "object",
		"title": "Contact Form",
		"properties": map[string]any{
			"name": map[string]any{
				"type":      "string",
				"title":     "Name",
				"minLength": float64(2),
			},
			"email": map[string]any{
				"type":   "string",
				"format": "email",
				"title":  "Email",
			},
			"message": map[string]any{
				"type":  "string",
				"title": "Message",
				"ui": map[string]any{
					"widget": "textarea",
					"rows":   float64(6),
				},
			},
			"subscribe": map[string]any{
				"type":  "boolean",
				"title": "Subscribe to newsletter",
			},
			"country": map[string]any{
				"type":  "string",
				"enum":  []any{"US", "CA", "UK", "AU"},
				"title": "Country",
			},
		},
		"required": []any{"name", "email"},
	}
	s, err := schemaFromMap(m)
	if err != nil {
		t.Fatal(err)
	}
	html := s.RenderFormHTML("contact", map[string]any{
		"name":    "Alice",
		"email":   "alice@example.com",
		"message": "",
	})
	if html == "" {
		t.Fatal("expected non-empty HTML")
	}
	if !strings.Contains(html, "spl-schema-form") {
		t.Fatal("expected spl-schema-form class")
	}
	if !strings.Contains(html, "Contact Form") {
		t.Fatal("expected title")
	}
	if !strings.Contains(html, "input type=\"email\"") {
		t.Fatal("expected email input")
	}
	if !strings.Contains(html, "textarea") {
		t.Fatal("expected textarea")
	}
	if !strings.Contains(html, "checkbox") {
		t.Fatal("expected checkbox")
	}
	if !strings.Contains(html, "<select") {
		t.Fatal("expected select")
	}
	if !strings.Contains(html, "required") {
		t.Fatal("expected required markers")
	}
	if s.Properties["message"].UI.Widget != "textarea" {
		t.Fatal("expected nested ui.widget to be parsed")
	}
	if s.Properties["message"].UI.Rows != 6 {
		t.Fatal("expected nested ui.rows to be parsed")
	}
}

func TestSchemaRenderDetailHTML(t *testing.T) {
	m := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":   map[string]any{"type": "string", "title": "Name"},
			"age":    map[string]any{"type": "integer", "title": "Age"},
			"active": map[string]any{"type": "boolean", "title": "Active"},
		},
	}
	s, _ := schemaFromMap(m)
	html := s.RenderDetailHTML("user", map[string]any{
		"name":   "Alice",
		"age":    float64(30),
		"active": true,
	})
	if !strings.Contains(html, "spl-schema-detail") {
		t.Fatal("expected detail container")
	}
	if !strings.Contains(html, "Alice") {
		t.Fatal("expected name value")
	}
}

func TestSchemaRenderTableHTML(t *testing.T) {
	m := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":  map[string]any{"type": "string", "title": "Name"},
			"email": map[string]any{"type": "string", "title": "Email"},
		},
	}
	s, _ := schemaFromMap(m)
	items := []map[string]any{
		{"name": "Alice", "email": "alice@example.com"},
		{"name": "Bob", "email": "bob@example.com"},
	}
	html := s.RenderTableHTML(items)
	if !strings.Contains(html, "spl-schema-table") {
		t.Fatal("expected table class")
	}
	if !strings.Contains(html, "alice@example.com") {
		t.Fatal("expected Alice's email")
	}
	if !strings.Contains(html, "<th>Name</th>") {
		t.Fatal("expected Name header")
	}
	if !strings.Contains(html, "<th>Email</th>") {
		t.Fatal("expected Email header")
	}
}

func TestSchemaFormDirective(t *testing.T) {
	e := New()
	schema, _ := schemaFromMap(map[string]any{
		"type":  "object",
		"title": "Profile",
		"properties": map[string]any{
			"name":  map[string]any{"type": "string", "title": "Name"},
			"email": map[string]any{"type": "string", "format": "email", "title": "Email"},
		},
		"required": []any{"name"},
	})
	e.SchemaRegistry.Register("profile", schema)

	out, err := e.Render(`@schema_form("profile", data)`, map[string]any{
		"data": map[string]any{"name": "Alice", "email": "alice@test.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "spl-schema-form") {
		t.Fatal("expected schema form wrapper, got: " + out)
	}
	if !strings.Contains(out, "Alice") {
		t.Fatal("expected data value to appear")
	}
}

func TestSchemaFormDirectiveSSRUsesSPLFeatures(t *testing.T) {
	e := New()
	schema, _ := schemaFromMap(map[string]any{
		"type":        "object",
		"title":       "Profile",
		"description": "Edit your profile",
		"properties": map[string]any{
			"name":      map[string]any{"type": "string", "title": "Name", "minLength": float64(2)},
			"email":     map[string]any{"type": "string", "format": "email", "title": "Email"},
			"role":      map[string]any{"type": "string", "title": "Role", "enum": []any{"Admin", "Member"}},
			"subscribe": map[string]any{"type": "boolean", "title": "Subscribe"},
		},
		"required": []any{"name", "email"},
	})
	e.SchemaRegistry.Register("profile", schema)

	out, err := e.RenderSSR(`@schema_form("profile", data)`, map[string]any{
		"data": map[string]any{
			"name":      "Alice",
			"email":     "alice@test.com",
			"role":      "Member",
			"subscribe": true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<div class="spl-schema-form" role="form"`,
		`type="button" data-spl-on-click="schemaProfile1Submit">Submit</button>`,
		`data-spl-model="data.name"`,
		`data-spl-model="data.subscribe"`,
		`data-spl-on-click=`,
		`data-spl-watch="text"`,
		`data-spl-view=`,
		`data-spl-effect=`,
		`data-spl-hydration`,
		`schemaProfile1Submit`,
		`schemaProfile1Reset`,
		`schemaProfile1Edit`,
		`signal(\"data\")`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected SSR schema form to contain %q, got: %s", want, out)
		}
	}
}

func TestSchemaFormAndDetailShareReactiveDataSignal(t *testing.T) {
	e := New()
	schema, _ := schemaFromMap(map[string]any{
		"type":  "object",
		"title": "Contact",
		"properties": map[string]any{
			"name": map[string]any{"type": "string", "title": "Name"},
			"message": map[string]any{
				"type":  "string",
				"title": "Message",
				"ui":    map[string]any{"widget": "textarea", "rows": float64(3)},
			},
		},
	})
	e.SchemaRegistry.Register("contact", schema)

	out, err := e.RenderSSR(`@schema_form("contact", contactData) @schema_detail("contact", contactData)`, map[string]any{
		"contactData": map[string]any{"name": "Alice", "message": "Hello"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`data-spl-model="contactData.name"`,
		`data-spl-model="contactData.message"`,
		`<textarea`,
		`data-spl-view=`,
		`__SPL_SIGNAL__contactData.name__`,
		`__SPL_SIGNAL__contactData.message__`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected shared reactive schema output to contain %q, got: %s", want, out)
		}
	}
	if strings.Contains(out, `<form`) {
		t.Fatal("reactive schema form should not emit a native form in sandboxed previews")
	}
}

func TestSchemaComplexNestedObjectsAndArrayControlsSSR(t *testing.T) {
	e := New()
	schema, _ := schemaFromMap(map[string]any{
		"type":  "object",
		"title": "Profile",
		"properties": map[string]any{
			"address": map[string]any{
				"type":  "object",
				"title": "Address",
				"properties": map[string]any{
					"street": map[string]any{"type": "string", "title": "Street"},
					"city":   map[string]any{"type": "string", "title": "City"},
				},
				"required": []any{"street"},
			},
			"tags": map[string]any{
				"type":     "array",
				"title":    "Tags",
				"minItems": float64(1),
				"maxItems": float64(3),
				"ui": map[string]any{"options": map[string]any{
					"minItemsMessage": "Keep at least one tag.",
					"maxItemsMessage": "Only three tags.",
					"actions": []any{
						map[string]any{"label": "Add tag", "action": "add", "scope": "array", "value": "new-tag"},
						map[string]any{"label": "Delete tag", "action": "remove", "scope": "item"},
					},
				}},
				"items": map[string]any{"type": "string", "title": "Tag"},
			},
			"contacts": map[string]any{
				"type":  "array",
				"title": "Contacts",
				"ui": map[string]any{"options": map[string]any{
					"actions": []any{
						map[string]any{"label": "Add email contact", "action": "add", "scope": "array", "value": map[string]any{"kind": "Email", "value": "new@example.com", "phones": []any{}}},
						map[string]any{"label": "Add phone contact", "action": "add", "scope": "array", "value": map[string]any{"kind": "Phone", "value": "+1-555-0100", "phones": []any{"+1-555-0100"}}},
						map[string]any{"label": "Earlier", "action": "moveUp", "scope": "item"},
						map[string]any{"label": "Later", "action": "moveDown", "scope": "item"},
						map[string]any{"label": "Delete", "action": "remove", "scope": "item"},
					},
				}},
				"items": map[string]any{
					"type":  "object",
					"title": "Contact",
					"properties": map[string]any{
						"kind":   map[string]any{"type": "string", "enum": []any{"Email", "Phone"}, "title": "Kind"},
						"value":  map[string]any{"type": "string", "title": "Value"},
						"phones": map[string]any{"type": "array", "title": "Phones", "minItems": float64(1), "maxItems": float64(3), "ui": map[string]any{"options": map[string]any{"minItemsMessage": "Keep a phone row.", "maxItemsMessage": "Only three phones.", "actions": []any{map[string]any{"label": "Add phone", "action": "add", "scope": "array", "value": "+1-555-0100"}, map[string]any{"label": "Remove phone", "action": "remove", "scope": "item"}}}}, "items": map[string]any{"type": "string", "title": "Phone"}},
					},
					"required": []any{"value"},
				},
			},
		},
	})
	e.SchemaRegistry.Register("profile", schema)

	out, err := e.RenderSSR(`@schema_form("profile", profileData)`, map[string]any{
		"profileData": map[string]any{
			"address":  map[string]any{"street": "1 Main", "city": "Portland"},
			"tags":     []any{"vip"},
			"contacts": []any{map[string]any{"kind": "Email", "value": "a@example.com", "phones": []any{"555-0100"}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`data-spl-model="profileData.address.street"`,
		`data-spl-schema-array-path="profileData.tags"`,
		`data-spl-schema-array-path="profileData.contacts"`,
		`data-spl-schema-array-max="3"`,
		`data-spl-schema-array-min-message="Keep at least one tag."`,
		`data-spl-schema-array-max-message="Only three tags."`,
		`Add tag`,
		`data-spl-schema-array-value="&#34;new-tag&#34;"`,
		`Delete tag`,
		`Add email contact`,
		`Add phone contact`,
		`data-spl-schema-array-value="{&#34;kind&#34;:&#34;Email&#34;,&#34;phones&#34;:[],&#34;value&#34;:&#34;new@example.com&#34;}"`,
		`Delete`,
		`Earlier`,
		`Later`,
		`data-spl-schema-array-path="__SPL_ARRAY_PATH_0__.phones"`,
		`data-spl-schema-array-min="1"`,
		`data-spl-schema-array-max="3"`,
		`data-spl-schema-array-min-message="Keep a phone row."`,
		`data-spl-schema-array-max-message="Only three phones."`,
		`Add phone`,
		`Remove phone`,
		`data-spl-model="__SPL_ARRAY_PATH_0__.value"`,
		`data-spl-model="__SPL_ARRAY_PATH_1__"`,
		`SPL.schemaArrayAction`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected complex schema output to contain %q, got: %s", want, out)
		}
	}
	if strings.Contains(out, "window.SPL") {
		t.Fatal("schema array handler should use the runtime SPL scope, not window.SPL")
	}
}

func TestSchemaArrayRuntimeAllowsMissingMaxAndFindsContainerHost(t *testing.T) {
	runtime := assembleRuntime(featAll, false, false)
	for _, want := range []string{
		`if(value==null || value===''){return fallback;}`,
		`closest('[data-spl-schema-array="true"][data-spl-schema-array-path="'+path+'"]')`,
		`while(arr.length<min)`,
		`if(isFinite(max) && arr.length>max)`,
		`removeButtons.forEach(function(removeButton){removeButton.disabled=arr.length<=min;});`,
		`messageEl.textContent=message;`,
		`selector='[id="'+SPL.escapeSelectorValue(active.id)+'"]'`,
		`try{next=root.querySelector(snapshot.selector);}`,
	} {
		if !strings.Contains(runtime, want) {
			t.Fatalf("expected schema array runtime to contain %q", want)
		}
	}
	if strings.Contains(runtime, `selector='#'+active.id`) {
		t.Fatal("focus capture should not build raw id selectors")
	}
}

func TestSchemaDetailDirective(t *testing.T) {
	e := New()
	schema, _ := schemaFromMap(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string", "title": "Name"},
		},
	})
	e.SchemaRegistry.Register("user", schema)

	out, err := e.Render(`@schema_detail("user", data)`, map[string]any{
		"data": map[string]any{"name": "Bob"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "spl-schema-detail") {
		t.Fatal("expected detail view, got: " + out)
	}
	if !strings.Contains(out, "Bob") {
		t.Fatal("expected data value to appear")
	}
}

func TestSchemaTableDirective(t *testing.T) {
	e := New()
	schema, _ := schemaFromMap(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string", "title": "Name"},
		},
	})
	e.SchemaRegistry.Register("item", schema)

	out, err := e.Render(`@schema_table("item", items)`, map[string]any{
		"items": []any{
			map[string]any{"name": "Alice"},
			map[string]any{"name": "Bob"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "spl-schema-table") {
		t.Fatal("expected table, got: " + out)
	}
}

func TestSchemaRegistry(t *testing.T) {
	r := NewSchemaRegistry()
	s := &Schema{Title: "Test"}
	r.Register("test", s)
	got, ok := r.Get("test")
	if !ok {
		t.Fatal("expected schema to be found")
	}
	if got.Title != "Test" {
		t.Fatalf("expected 'Test', got %q", got.Title)
	}
	_, ok = r.Get("nonexistent")
	if ok {
		t.Fatal("expected nonexistent schema to not be found")
	}
}

func TestSchemaInputTypeMapping(t *testing.T) {
	tests := []struct {
		schema   *Schema
		expected string
	}{
		{&Schema{Type: SchemaTypeString}, "text"},
		{&Schema{Type: SchemaTypeString, Format: "email"}, "email"},
		{&Schema{Type: SchemaTypeString, Format: "uri"}, "url"},
		{&Schema{Type: SchemaTypeString, Format: "date"}, "date"},
		{&Schema{Type: SchemaTypeString, Enum: []any{"a", "b"}}, "select"},
		{&Schema{Type: SchemaTypeString, UIWidget: "textarea"}, "textarea"},
		{&Schema{Type: SchemaTypeInteger}, "number"},
		{&Schema{Type: SchemaTypeNumber}, "number"},
		{&Schema{Type: SchemaTypeBoolean}, "checkbox"},
	}
	for _, tt := range tests {
		got := tt.schema.inputType()
		if got != tt.expected {
			t.Fatalf("expected %q, got %q for type=%q format=%q", tt.expected, got, tt.schema.Type, tt.schema.Format)
		}
	}
}

func TestSchemaEngineRegistration(t *testing.T) {
	e := New()
	if e.SchemaRegistry == nil {
		t.Fatal("expected SchemaRegistry to be initialized")
	}
	s := &Schema{Title: "Test"}
	e.SchemaRegistry.Register("test", s)
	got, ok := e.SchemaRegistry.Get("test")
	if !ok || got.Title != "Test" {
		t.Fatal("schema registration/retrieval through Engine failed")
	}
}
