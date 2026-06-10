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
		"type": "object",
		"title": "Contact Form",
		"properties": map[string]any{
			"name": map[string]any{
				"type":        "string",
				"title":       "Name",
				"minLength":   float64(2),
			},
			"email": map[string]any{
				"type":   "string",
				"format": "email",
				"title":  "Email",
			},
			"message": map[string]any{
				"type":        "string",
				"title":       "Message",
				"ui:widget":   "textarea",
				"ui:rows":     float64(6),
			},
			"subscribe": map[string]any{
				"type":  "boolean",
				"title": "Subscribe to newsletter",
			},
			"country": map[string]any{
				"type": "string",
				"enum": []any{"US", "CA", "UK", "AU"},
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
}

func TestSchemaRenderDetailHTML(t *testing.T) {
	m := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string", "title": "Name"},
			"age":  map[string]any{"type": "integer", "title": "Age"},
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
		"type": "object",
		"title": "Profile",
		"properties": map[string]any{
			"name": map[string]any{"type": "string", "title": "Name"},
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
