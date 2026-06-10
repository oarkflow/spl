package spl

import (
	"encoding/json"
	"fmt"
	"html"
	"math"
	"regexp"
	"strconv"
	"strings"
)

type SchemaType string

const (
	SchemaTypeObject  SchemaType = "object"
	SchemaTypeString  SchemaType = "string"
	SchemaTypeNumber  SchemaType = "number"
	SchemaTypeInteger SchemaType = "integer"
	SchemaTypeBoolean SchemaType = "boolean"
	SchemaTypeArray   SchemaType = "array"
	SchemaTypeNull    SchemaType = "null"
)

type Schema struct {
	Title            string             `json:"title,omitempty"`
	Description      string             `json:"description,omitempty"`
	Type             SchemaType         `json:"type,omitempty"`
	Properties       map[string]*Schema `json:"properties,omitempty"`
	Items            *Schema            `json:"items,omitempty"`
	Required         []string           `json:"required,omitempty"`
	Enum             []any              `json:"enum,omitempty"`
	Format           string             `json:"format,omitempty"`
	Pattern          string             `json:"pattern,omitempty"`
	MinLength        *int               `json:"minLength,omitempty"`
	MaxLength        *int               `json:"maxLength,omitempty"`
	Minimum          *float64           `json:"minimum,omitempty"`
	Maximum          *float64           `json:"maximum,omitempty"`
	ExclusiveMinimum *float64           `json:"exclusiveMinimum,omitempty"`
	ExclusiveMaximum *float64           `json:"exclusiveMaximum,omitempty"`
	MultipleOf       *float64           `json:"multipleOf,omitempty"`
	MinItems         *int               `json:"minItems,omitempty"`
	MaxItems         *int               `json:"maxItems,omitempty"`
	Default          any                `json:"default,omitempty"`
	Examples         []any              `json:"examples,omitempty"`
	UI               SchemaUI           `json:"ui,omitempty"`

	// Legacy ui:* keys are still accepted for existing schemas. Prefer "ui": {...}.
	UIWidget      string         `json:"ui:widget,omitempty"`
	UIOptions     map[string]any `json:"ui:options,omitempty"`
	UIPlaceholder string         `json:"ui:placeholder,omitempty"`
	UIOrder       int            `json:"ui:order,omitempty"`
	UIHidden      bool           `json:"ui:hidden,omitempty"`
	UIDisabled    bool           `json:"ui:disabled,omitempty"`
	UIRows        int            `json:"ui:rows,omitempty"`
	UIAccept      string         `json:"ui:accept,omitempty"`
}

type SchemaUI struct {
	Widget      string         `json:"widget,omitempty"`
	Options     map[string]any `json:"options,omitempty"`
	Placeholder string         `json:"placeholder,omitempty"`
	Order       int            `json:"order,omitempty"`
	Hidden      bool           `json:"hidden,omitempty"`
	Disabled    bool           `json:"disabled,omitempty"`
	Rows        int            `json:"rows,omitempty"`
	Accept      string         `json:"accept,omitempty"`
}

type SchemaRegistry struct {
	schemas map[string]*Schema
}

func NewSchemaRegistry() *SchemaRegistry {
	return &SchemaRegistry{schemas: make(map[string]*Schema)}
}

func (r *SchemaRegistry) Register(name string, schema *Schema) {
	r.schemas[name] = schema
}

func (r *SchemaRegistry) Get(name string) (*Schema, bool) {
	s, ok := r.schemas[name]
	return s, ok
}

func SchemaFromMap(m map[string]any) (*Schema, error) {
	return schemaFromMap(m)
}

func schemaFromMap(m map[string]any) (*Schema, error) {
	s := &Schema{}
	if v, ok := m["title"]; ok {
		s.Title, _ = v.(string)
	}
	if v, ok := m["description"]; ok {
		s.Description, _ = v.(string)
	}
	if v, ok := m["type"]; ok {
		s.Type = SchemaType(v.(string))
	}
	if v, ok := m["format"]; ok {
		s.Format, _ = v.(string)
	}
	if v, ok := m["pattern"]; ok {
		s.Pattern, _ = v.(string)
	}
	if v, ok := m["default"]; ok {
		s.Default = v
	}
	if v, ok := m["required"]; ok {
		if req, ok := v.([]any); ok {
			for _, r := range req {
				s.Required = append(s.Required, r.(string))
			}
		}
	}
	if v, ok := m["enum"]; ok {
		if en, ok := v.([]any); ok {
			s.Enum = en
		}
	}
	if v, ok := m["examples"]; ok {
		if ex, ok := v.([]any); ok {
			s.Examples = ex
		}
	}
	if v, ok := m["minLength"]; ok {
		if f, ok := toFloat64(v); ok {
			i := int(f)
			s.MinLength = &i
		}
	}
	if v, ok := m["maxLength"]; ok {
		if f, ok := toFloat64(v); ok {
			i := int(f)
			s.MaxLength = &i
		}
	}
	if v, ok := m["minimum"]; ok {
		if f, ok := toFloat64(v); ok {
			s.Minimum = &f
		}
	}
	if v, ok := m["maximum"]; ok {
		if f, ok := toFloat64(v); ok {
			s.Maximum = &f
		}
	}
	if v, ok := m["exclusiveMinimum"]; ok {
		if f, ok := toFloat64(v); ok {
			s.ExclusiveMinimum = &f
		}
	}
	if v, ok := m["exclusiveMaximum"]; ok {
		if f, ok := toFloat64(v); ok {
			s.ExclusiveMaximum = &f
		}
	}
	if v, ok := m["multipleOf"]; ok {
		if f, ok := toFloat64(v); ok {
			s.MultipleOf = &f
		}
	}
	if v, ok := m["minItems"]; ok {
		if f, ok := toFloat64(v); ok {
			i := int(f)
			s.MinItems = &i
		}
	}
	if v, ok := m["maxItems"]; ok {
		if f, ok := toFloat64(v); ok {
			i := int(f)
			s.MaxItems = &i
		}
	}
	if v, ok := m["properties"]; ok {
		if props, ok := v.(map[string]any); ok {
			s.Properties = make(map[string]*Schema, len(props))
			for k, pv := range props {
				if pm, ok := pv.(map[string]any); ok {
					sub, err := schemaFromMap(pm)
					if err != nil {
						return nil, fmt.Errorf("property %q: %w", k, err)
					}
					s.Properties[k] = sub
				}
			}
		}
	}
	if v, ok := m["items"]; ok {
		if im, ok := v.(map[string]any); ok {
			sub, err := schemaFromMap(im)
			if err != nil {
				return nil, fmt.Errorf("items: %w", err)
			}
			s.Items = sub
		}
	}
	if v, ok := m["ui"]; ok {
		if ui, ok := v.(map[string]any); ok {
			s.applyUIMap(ui)
		}
	}
	if v, ok := m["ui:widget"]; ok {
		s.UI.Widget, _ = v.(string)
	}
	if v, ok := m["ui:options"]; ok {
		s.UI.Options, _ = v.(map[string]any)
	}
	if v, ok := m["ui:placeholder"]; ok {
		s.UI.Placeholder, _ = v.(string)
	}
	if v, ok := m["ui:order"]; ok {
		if f, ok := toFloat64(v); ok {
			s.UI.Order = int(f)
		}
	}
	if v, ok := m["ui:hidden"]; ok {
		s.UI.Hidden, _ = v.(bool)
	}
	if v, ok := m["ui:disabled"]; ok {
		s.UI.Disabled, _ = v.(bool)
	}
	if v, ok := m["ui:rows"]; ok {
		if f, ok := toFloat64(v); ok {
			s.UI.Rows = int(f)
		}
	}
	if v, ok := m["ui:accept"]; ok {
		s.UI.Accept, _ = v.(string)
	}
	s.syncLegacyUIFields()
	return s, nil
}

func (s *Schema) applyUIMap(ui map[string]any) {
	if v, ok := ui["widget"]; ok {
		s.UI.Widget, _ = v.(string)
	}
	if v, ok := ui["options"]; ok {
		s.UI.Options, _ = v.(map[string]any)
	}
	if v, ok := ui["placeholder"]; ok {
		s.UI.Placeholder, _ = v.(string)
	}
	if v, ok := ui["order"]; ok {
		if f, ok := toFloat64(v); ok {
			s.UI.Order = int(f)
		}
	}
	if v, ok := ui["hidden"]; ok {
		s.UI.Hidden, _ = v.(bool)
	}
	if v, ok := ui["disabled"]; ok {
		s.UI.Disabled, _ = v.(bool)
	}
	if v, ok := ui["rows"]; ok {
		if f, ok := toFloat64(v); ok {
			s.UI.Rows = int(f)
		}
	}
	if v, ok := ui["accept"]; ok {
		s.UI.Accept, _ = v.(string)
	}
}

func (s *Schema) syncLegacyUIFields() {
	s.UIWidget = s.UI.Widget
	s.UIOptions = s.UI.Options
	s.UIPlaceholder = s.UI.Placeholder
	s.UIOrder = s.UI.Order
	s.UIHidden = s.UI.Hidden
	s.UIDisabled = s.UI.Disabled
	s.UIRows = s.UI.Rows
	s.UIAccept = s.UI.Accept
}

func toFloat64(v any) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case uint64:
		return float64(val), true
	case json.Number:
		f, err := val.Float64()
		return f, err == nil
	}
	return 0, false
}

func (s *Schema) isRequired(name string) bool {
	for _, r := range s.Required {
		if r == name {
			return true
		}
	}
	return false
}

func (s *Schema) uiWidget() string {
	if s.UI.Widget != "" {
		return s.UI.Widget
	}
	return s.UIWidget
}

func (s *Schema) uiOptions() map[string]any {
	if s.UI.Options != nil {
		return s.UI.Options
	}
	return s.UIOptions
}

func (s *Schema) uiPlaceholder() string {
	if s.UI.Placeholder != "" {
		return s.UI.Placeholder
	}
	return s.UIPlaceholder
}

func (s *Schema) uiOrder() int {
	if s.UI.Order != 0 {
		return s.UI.Order
	}
	return s.UIOrder
}

func (s *Schema) uiHidden() bool {
	return s.UI.Hidden || s.UIHidden
}

func (s *Schema) uiDisabled() bool {
	return s.UI.Disabled || s.UIDisabled
}

func (s *Schema) uiRows() int {
	if s.UI.Rows != 0 {
		return s.UI.Rows
	}
	return s.UIRows
}

func (s *Schema) uiAccept() string {
	if s.UI.Accept != "" {
		return s.UI.Accept
	}
	return s.UIAccept
}

func (s *Schema) uiBoolOption(defaultValue bool, names ...string) bool {
	opts := s.uiOptions()
	if opts == nil {
		return defaultValue
	}
	for _, name := range names {
		if v, ok := opts[name]; ok {
			if b, ok := v.(bool); ok {
				return b
			}
		}
	}
	return defaultValue
}

func (s *Schema) uiStringOption(defaultValue string, names ...string) string {
	opts := s.uiOptions()
	if opts == nil {
		return defaultValue
	}
	for _, name := range names {
		if v, ok := opts[name]; ok {
			if str, ok := v.(string); ok && str != "" {
				return str
			}
		}
	}
	return defaultValue
}

func (s *Schema) arrayMessages() (string, string, string) {
	return s.uiStringOption("No items", "emptyMessage", "emptyItemsMessage"),
		s.uiStringOption("Minimum items reached", "minItemsMessage", "minMessage"),
		s.uiStringOption("Maximum items reached", "maxItemsMessage", "maxMessage")
}

type schemaArrayActionConfig struct {
	Label     string
	Action    string
	Scope     string
	ClassName string
	Direction int
	Value     any
	HasValue  bool
}

func (s *Schema) uiArrayActions() []schemaArrayActionConfig {
	opts := s.uiOptions()
	if opts == nil {
		return nil
	}
	raw, ok := opts["actions"]
	if !ok {
		return nil
	}
	list, ok := raw.([]any)
	if !ok {
		return nil
	}
	actions := make([]schemaArrayActionConfig, 0, len(list))
	for _, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		cfg := schemaArrayActionConfig{}
		if v, ok := m["label"].(string); ok {
			cfg.Label = v
		}
		if v, ok := m["action"].(string); ok {
			cfg.Action = strings.TrimSpace(v)
		}
		if v, ok := m["scope"].(string); ok {
			cfg.Scope = strings.TrimSpace(v)
		}
		if v, ok := m["class"].(string); ok {
			cfg.ClassName = strings.TrimSpace(v)
		}
		if v, ok := m["className"].(string); ok && cfg.ClassName == "" {
			cfg.ClassName = strings.TrimSpace(v)
		}
		if v, ok := toFloat64(m["direction"]); ok {
			cfg.Direction = int(v)
		}
		if v, ok := m["value"]; ok {
			cfg.Value = v
			cfg.HasValue = true
		}
		cfg.normalize()
		if cfg.Action != "" && cfg.Label != "" {
			actions = append(actions, cfg)
		}
	}
	return actions
}

func (a *schemaArrayActionConfig) normalize() {
	action := strings.ToLower(strings.TrimSpace(a.Action))
	switch action {
	case "up", "moveup", "move-up":
		a.Action = "move"
		a.Direction = -1
	case "down", "movedown", "move-down":
		a.Action = "move"
		a.Direction = 1
	case "move":
		a.Action = "move"
		if a.Direction == 0 {
			a.Direction = 1
		}
	case "delete":
		a.Action = "remove"
	case "add", "remove":
		a.Action = action
	default:
		a.Action = action
	}
	if a.Scope == "" {
		if a.Action == "add" {
			a.Scope = "array"
		} else {
			a.Scope = "item"
		}
	}
	a.Scope = strings.ToLower(a.Scope)
}

func (s *Schema) defaultValue() any {
	if s.Default != nil {
		return s.Default
	}
	if len(s.Enum) > 0 {
		return s.Enum[0]
	}
	switch s.Type {
	case SchemaTypeObject:
		obj := make(map[string]any)
		for _, prop := range orderedProps(s) {
			obj[prop] = s.Properties[prop].defaultValue()
		}
		return obj
	case SchemaTypeArray:
		return []any{}
	case SchemaTypeInteger:
		return 0
	case SchemaTypeNumber:
		return 0
	case SchemaTypeBoolean:
		return false
	default:
		return ""
	}
}

func (s *Schema) inputType() string {
	if s.uiWidget() != "" {
		return s.uiWidget()
	}
	switch s.Type {
	case SchemaTypeString:
		switch s.Format {
		case "email":
			return "email"
		case "uri", "url":
			return "url"
		case "date":
			return "date"
		case "date-time":
			return "datetime-local"
		case "time":
			return "time"
		case "tel", "phone":
			return "tel"
		case "color":
			return "color"
		case "textarea":
			return "textarea"
		}
		if s.Enum != nil {
			return "select"
		}
		return "text"
	case SchemaTypeInteger:
		return "number"
	case SchemaTypeNumber:
		return "number"
	case SchemaTypeBoolean:
		return "checkbox"
	case SchemaTypeArray:
		return "array"
	case SchemaTypeObject:
		return "object"
	}
	return "text"
}

type schemaValidation struct {
	Type      SchemaType `json:"type"`
	Required  bool       `json:"required,omitempty"`
	MinLength *int       `json:"minLength,omitempty"`
	MaxLength *int       `json:"maxLength,omitempty"`
	Minimum   *float64   `json:"minimum,omitempty"`
	Maximum   *float64   `json:"maximum,omitempty"`
	Pattern   string     `json:"pattern,omitempty"`
	MinItems  *int       `json:"minItems,omitempty"`
	MaxItems  *int       `json:"maxItems,omitempty"`
	Enum      []any      `json:"enum,omitempty"`
}

func (s *Schema) validation() schemaValidation {
	return schemaValidation{
		Type:      s.Type,
		Required:  false,
		MinLength: s.MinLength,
		MaxLength: s.MaxLength,
		Minimum:   s.Minimum,
		Maximum:   s.Maximum,
		Pattern:   s.Pattern,
		MinItems:  s.MinItems,
		MaxItems:  s.MaxItems,
		Enum:      s.Enum,
	}
}

func (s *Schema) validateValue(name string, value any, required bool) []string {
	var errs []string
	if value == nil {
		if required {
			errs = append(errs, fmt.Sprintf("%s is required", name))
		}
		return errs
	}
	switch s.Type {
	case SchemaTypeString:
		str, ok := value.(string)
		if !ok {
			errs = append(errs, fmt.Sprintf("%s must be a string", name))
			return errs
		}
		if s.MinLength != nil && len(str) < *s.MinLength {
			errs = append(errs, fmt.Sprintf("%s must be at least %d characters", name, *s.MinLength))
		}
		if s.MaxLength != nil && len(str) > *s.MaxLength {
			errs = append(errs, fmt.Sprintf("%s must be at most %d characters", name, *s.MaxLength))
		}
		if s.Pattern != "" {
			matched, _ := regexp.MatchString(s.Pattern, str)
			if !matched {
				errs = append(errs, fmt.Sprintf("%s does not match required pattern", name))
			}
		}
		if s.Enum != nil {
			if !containsEnum(s.Enum, str) {
				errs = append(errs, fmt.Sprintf("%s must be one of: %v", name, s.Enum))
			}
		}
	case SchemaTypeInteger:
		var num int64
		switch v := value.(type) {
		case float64:
			num = int64(v)
		case int:
			num = int64(v)
		case int64:
			num = v
		default:
			errs = append(errs, fmt.Sprintf("%s must be an integer", name))
			return errs
		}
		if s.Minimum != nil && float64(num) < *s.Minimum {
			errs = append(errs, fmt.Sprintf("%s must be >= %v", name, *s.Minimum))
		}
		if s.Maximum != nil && float64(num) > *s.Maximum {
			errs = append(errs, fmt.Sprintf("%s must be <= %v", name, *s.Maximum))
		}
		if s.Enum != nil {
			if !containsEnum(s.Enum, num) {
				errs = append(errs, fmt.Sprintf("%s must be one of: %v", name, s.Enum))
			}
		}
	case SchemaTypeNumber:
		var num float64
		switch v := value.(type) {
		case float64:
			num = v
		case int:
			num = float64(v)
		case int64:
			num = float64(v)
		default:
			errs = append(errs, fmt.Sprintf("%s must be a number", name))
			return errs
		}
		if s.Minimum != nil && num < *s.Minimum {
			errs = append(errs, fmt.Sprintf("%s must be >= %v", name, *s.Minimum))
		}
		if s.Maximum != nil && num > *s.Maximum {
			errs = append(errs, fmt.Sprintf("%s must be <= %v", name, *s.Maximum))
		}
		if s.Enum != nil {
			if !containsEnum(s.Enum, num) {
				errs = append(errs, fmt.Sprintf("%s must be one of: %v", name, s.Enum))
			}
		}
	case SchemaTypeBoolean:
		if _, ok := value.(bool); !ok {
			errs = append(errs, fmt.Sprintf("%s must be a boolean", name))
		}
	case SchemaTypeArray:
		arr, ok := value.([]any)
		if !ok {
			errs = append(errs, fmt.Sprintf("%s must be an array", name))
			return errs
		}
		if s.MinItems != nil && len(arr) < *s.MinItems {
			errs = append(errs, fmt.Sprintf("%s must have at least %d items", name, *s.MinItems))
		}
		if s.MaxItems != nil && len(arr) > *s.MaxItems {
			errs = append(errs, fmt.Sprintf("%s must have at most %d items", name, *s.MaxItems))
		}
	}
	return errs
}

func containsEnum(enum []any, val any) bool {
	for _, e := range enum {
		if fmt.Sprintf("%v", e) == fmt.Sprintf("%v", val) {
			return true
		}
	}
	return false
}

func formatValue(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == math.Trunc(val) {
			return strconv.FormatInt(int64(val), 10)
		}
		return strconv.FormatFloat(val, 'f', -1, 64)
	case int:
		return strconv.Itoa(val)
	case int64:
		return strconv.FormatInt(val, 10)
	case bool:
		if val {
			return "Yes"
		}
		return "No"
	case map[string]any, []any:
		b, _ := json.Marshal(val)
		return string(b)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func (s *Schema) renderFieldHTML(name string, value any, path string, required bool, depth int) string {
	if s.uiHidden() {
		return ""
	}
	inpType := s.inputType()
	fieldID := html.EscapeString(strings.ReplaceAll(path, ".", "_"))
	fieldName := html.EscapeString(path)
	displayName := s.Title
	if displayName == "" {
		displayName = name
	}
	dispName := html.EscapeString(displayName)
	var desc string
	if s.Description != "" {
		desc = fmt.Sprintf(`<p class="spl-schema-desc">%s</p>`, html.EscapeString(s.Description))
	}
	reqMark := ""
	if required {
		reqMark = ` <span class="spl-schema-required" title="Required">*</span>`
	}
	strVal := formatValue(value)
	htmlVal := html.EscapeString(strVal)
	ph := ""
	if s.uiPlaceholder() != "" {
		ph = fmt.Sprintf(` placeholder="%s"`, html.EscapeString(s.uiPlaceholder()))
	}
	dis := ""
	if s.uiDisabled() {
		dis = ` disabled`
	}
	extraAttrs := ""
	if s.MinLength != nil {
		extraAttrs += fmt.Sprintf(` minlength="%d"`, *s.MinLength)
	}
	if s.MaxLength != nil {
		extraAttrs += fmt.Sprintf(` maxlength="%d"`, *s.MaxLength)
	}
	if s.Minimum != nil {
		extraAttrs += fmt.Sprintf(` min="%v"`, *s.Minimum)
	}
	if s.Maximum != nil {
		extraAttrs += fmt.Sprintf(` max="%v"`, *s.Maximum)
	}
	if s.Pattern != "" {
		extraAttrs += fmt.Sprintf(` pattern="%s"`, html.EscapeString(s.Pattern))
	}

	indent := strings.Repeat("  ", depth+1)

	switch inpType {
	case "textarea":
		rows := 4
		if s.uiRows() > 0 {
			rows = s.uiRows()
		}
		return fmt.Sprintf(`%s<div class="spl-schema-field">
%s  <label class="spl-schema-label" for="%s">%s%s</label>
%s  %s
%s  <textarea id="%s" name="%s" rows="%d" class="spl-schema-input"%s%s>%s</textarea>
%s</div>`, indent, indent, fieldID, dispName, reqMark, indent, desc, indent, fieldID, fieldName, rows, extraAttrs, dis, htmlVal, indent)

	case "select":
		var opts strings.Builder
		if s.uiPlaceholder() != "" {
			opts.WriteString(fmt.Sprintf(`        <option value="">%s</option>`, html.EscapeString(s.uiPlaceholder())))
		} else {
			opts.WriteString(`        <option value="">Select...</option>`)
		}
		for _, opt := range s.Enum {
			optStr := formatValue(opt)
			sel := ""
			if optStr == strVal {
				sel = ` selected`
			}
			opts.WriteString(fmt.Sprintf("\n        <option value=\"%s\"%s>%s</option>", html.EscapeString(optStr), sel, html.EscapeString(optStr)))
		}
		return fmt.Sprintf(`%s<div class="spl-schema-field">
%s  <label class="spl-schema-label" for="%s">%s%s</label>
%s  %s
%s  <select id="%s" name="%s" class="spl-schema-input"%s%s>
%s
%s  </select>
%s</div>`, indent, indent, fieldID, dispName, reqMark, indent, desc, indent, fieldID, fieldName, extraAttrs, dis, indent+opts.String(), indent, indent)

	case "checkbox":
		checked := ""
		if b, ok := value.(bool); ok && b {
			checked = ` checked`
		}
		return fmt.Sprintf(`%s<div class="spl-schema-field spl-schema-field-checkbox">
%s  <label class="spl-schema-label-checkbox">
%s    <input type="checkbox" id="%s" name="%s" class="spl-schema-input"%s%s%s />
%s    %s%s
%s  </label>
%s  %s
%s</div>`, indent, indent, indent, fieldID, fieldName, extraAttrs, dis, checked, indent, dispName, reqMark, indent, indent, desc, indent)

	case "object":
		if s.Properties == nil {
			return ""
		}
		var valMap map[string]any
		if m, ok := value.(map[string]any); ok {
			valMap = m
		} else {
			valMap = make(map[string]any)
		}
		var inner strings.Builder
		for _, prop := range orderedProps(s) {
			propSchema := s.Properties[prop]
			propRequired := s.isRequired(prop)
			propVal := valMap[prop]
			propPath := path + "." + prop
			inner.WriteString(propSchema.renderFieldHTML(prop, propVal, propPath, propRequired, depth+1))
		}
		return fmt.Sprintf(`%s<fieldset class="spl-schema-object">
%s  <legend class="spl-schema-legend">%s%s</legend>
%s  %s
%s
%s</fieldset>`, indent, indent, dispName, reqMark, indent, desc, inner.String(), indent)

	case "array":
		var items []any
		if arr, ok := value.([]any); ok {
			items = arr
		}
		var inner strings.Builder
		inner.WriteString(fmt.Sprintf(`%s  <div class="spl-schema-array-items">`, indent))
		for i, item := range items {
			itemPath := fmt.Sprintf("%s.%d", path, i)
			if s.Items != nil {
				inner.WriteString(s.Items.renderFieldHTML(fmt.Sprintf("%s[%d]", name, i), item, itemPath, false, depth+1))
			}
		}
		inner.WriteString(fmt.Sprintf(`%s  </div>`, indent))
		return fmt.Sprintf(`%s<div class="spl-schema-field">
%s  <label class="spl-schema-label">%s%s</label>
%s  %s
%s
%s</div>`, indent, indent, dispName, reqMark, indent, desc, inner.String(), indent)

	default:
		return fmt.Sprintf(`%s<div class="spl-schema-field">
%s  <label class="spl-schema-label" for="%s">%s%s</label>
%s  %s
%s  <input type="%s" id="%s" name="%s" value="%s" class="spl-schema-input"%s%s%s />
%s</div>`, indent, indent, fieldID, dispName, reqMark, indent, desc, indent, inpType, fieldID, fieldName, htmlVal, extraAttrs, ph, dis, indent)
	}
}

func orderedProps(s *Schema) []string {
	type namedProp struct {
		name   string
		schema *Schema
		order  int
	}
	var list []namedProp
	for name, prop := range s.Properties {
		o := prop.uiOrder()
		if o == 0 {
			o = math.MaxInt32
		}
		list = append(list, namedProp{name, prop, o})
	}
	for i := 0; i < len(list); i++ {
		for j := i + 1; j < len(list); j++ {
			if list[i].order > list[j].order || (list[i].order == list[j].order && list[i].name > list[j].name) {
				list[i], list[j] = list[j], list[i]
			}
		}
	}
	result := make([]string, len(list))
	for i, np := range list {
		result[i] = np.name
	}
	return result
}

func (s *Schema) RenderFormHTML(name string, data map[string]any) string {
	var buf strings.Builder
	buf.WriteString(`<div class="spl-schema-form">`)
	if s.Title != "" {
		buf.WriteString(fmt.Sprintf(`<h3 class="spl-schema-title">%s</h3>`, html.EscapeString(s.Title)))
	}
	if s.Description != "" {
		buf.WriteString(fmt.Sprintf(`<p class="spl-schema-description">%s</p>`, html.EscapeString(s.Description)))
	}
	for _, prop := range orderedProps(s) {
		propSchema := s.Properties[prop]
		required := s.isRequired(prop)
		val := data[prop]
		buf.WriteString(propSchema.renderFieldHTML(prop, val, prop, required, 1))
	}
	buf.WriteString("\n</div>")
	return buf.String()
}

func (s *Schema) RenderFormSPL(name string, data map[string]any, prefix, dataSignal string) string {
	if data == nil {
		data = make(map[string]any)
	}
	if dataSignal == "" {
		dataSignal = prefix + "Data"
	}
	submittedSignal := prefix + "Submitted"
	statusSignal := prefix + "Status"
	resultSignal := prefix + "Result"
	fieldComponent := prefix + "Field"
	submitHandler := prefix + "Submit"
	resetHandler := prefix + "Reset"
	editHandler := prefix + "Edit"
	arrayHandler := prefix + "ArrayAction"
	initialJSON, _ := json.Marshal(data)
	if len(initialJSON) == 0 {
		initialJSON = []byte(`{}`)
	}
	initialStatus := "Complete the schema form."
	if len(s.ValidateAll(data)) == 0 {
		initialStatus = "Ready to submit."
	}

	var buf strings.Builder
	buf.WriteString(fmt.Sprintf("@signal(%s = %s)\n", dataSignal, initialJSON))
	buf.WriteString(fmt.Sprintf("@signal(%s = false)\n", submittedSignal))
	buf.WriteString(fmt.Sprintf("@signal(%s = %s)\n", statusSignal, strconv.Quote(initialStatus)))
	buf.WriteString(fmt.Sprintf("@signal(%s = \"\")\n", resultSignal))
	buf.WriteString(fmt.Sprintf(`@component(%s, label, required = false, description = "") {
  <div class="spl-schema-field">
    <label class="spl-schema-label">@slot("label")@if(required) { <span class="spl-schema-required" title="Required">*</span> }</label>
    @slot
    @if(description) { <p class="spl-schema-desc">${description}</p> }
  </div>
}
`, strconv.Quote(fieldComponent)))
	buf.WriteString(fmt.Sprintf(`@handler(%s) {
  setSignal(%s, false);
  setSignal(%s, 'Editing schema form.');
}
`, editHandler, submittedSignal, statusSignal))
	buf.WriteString(fmt.Sprintf(`@handler(%s) {
  var form = element && element.closest ? element.closest('[data-spl-schema-name]') : null;
  var fields = form && form.querySelectorAll ? Array.from(form.querySelectorAll('input, select, textarea')) : [];
  var ok = fields.every(function(field){ return !field.checkValidity || field.checkValidity(); });
  setSignal(%s, true);
  setSignal(%s, ok ? 'Schema form submitted.' : 'Fix the highlighted fields.');
  setSignal(%s, JSON.stringify(signal(%s), null, 2));
  if(!ok){
    var invalid = fields.find(function(field){ return field.reportValidity && !field.checkValidity(); });
    if(invalid){ invalid.reportValidity(); }
  }
}
`, submitHandler, submittedSignal, statusSignal, resultSignal, strconv.Quote(dataSignal)))
	buf.WriteString(fmt.Sprintf(`@handler(%s) {
  if(SPL && SPL.schemaArrayAction){ SPL.schemaArrayAction(element); }
  setSignal(%s, false);
  setSignal(%s, 'Editing schema form.');
}
`, arrayHandler, submittedSignal, statusSignal))
	buf.WriteString(fmt.Sprintf(`@handler(%s) {
  setSignal(%s, %s);
  setSignal(%s, false);
  setSignal(%s, 'Complete the schema form.');
  setSignal(%s, '');
}
`, resetHandler, dataSignal, initialJSON, submittedSignal, statusSignal, resultSignal))
	buf.WriteString(`<div class="spl-schema-form" role="form" data-spl-schema-name="`)
	buf.WriteString(html.EscapeString(name))
	buf.WriteString(`">`)
	if s.Title != "" {
		buf.WriteString(fmt.Sprintf(`<h3 class="spl-schema-title">%s</h3>`, html.EscapeString(s.Title)))
	}
	if s.Description != "" {
		buf.WriteString(fmt.Sprintf(`<p class="spl-schema-description">%s</p>`, html.EscapeString(s.Description)))
	}
	buf.WriteString(fmt.Sprintf(`<div class="spl-schema-live" aria-live="polite">@bind(%s)</div>`, statusSignal))
	for _, prop := range orderedProps(s) {
		propSchema := s.Properties[prop]
		required := s.isRequired(prop)
		propSchema.renderFieldSPL(&buf, fieldComponent, editHandler, arrayHandler, dataSignal, prop, prop, data[prop], required, 1, 0)
	}
	buf.WriteString(fmt.Sprintf(`<div class="spl-schema-actions">
  <button class="spl-schema-submit" type="button" on:click="%s">Submit</button>
  <button class="spl-schema-reset" type="button" on:click="%s">Reset</button>
</div>
@effect(%s) {
  <span class="spl-schema-effect" hidden>Updated @bind(%s)</span>
}
@reactive(%s, %s, %s) {
  @if(%s) {
    <pre class="spl-schema-result">${%s}</pre>
  }
}
</div>`, submitHandler, resetHandler, dataSignal, statusSignal, dataSignal, submittedSignal, resultSignal, submittedSignal, resultSignal))
	return buf.String()
}

func (s *Schema) renderFieldSPL(buf *strings.Builder, fieldComponent, editHandler, arrayHandler, dataSignal, name, path string, value any, required bool, depth, arrayLevel int) {
	if s.uiHidden() {
		return
	}
	inpType := s.inputType()
	fieldID := html.EscapeString(strings.ReplaceAll(path, ".", "_"))
	fieldName := html.EscapeString(path)
	displayName := s.Title
	if displayName == "" {
		displayName = name
	}
	modelPath := dataSignal + "." + path
	extraAttrs := s.splInputAttrs(required)
	if s.uiDisabled() {
		extraAttrs += ` disabled`
	}
	placeholder := ""
	if s.uiPlaceholder() != "" {
		placeholder = fmt.Sprintf(` placeholder="%s"`, html.EscapeString(s.uiPlaceholder()))
	}
	watchGroup := inpType
	if s.Type == SchemaTypeObject {
		watchGroup = "object"
	}
	buf.WriteString(fmt.Sprintf("\n@watch(%s) { <div class=\"spl-schema-watch\" data-spl-watch=\"%s\"></div> }\n", strconv.Quote(watchGroup), html.EscapeString(watchGroup)))
	switch inpType {
	case "object":
		buf.WriteString(`<fieldset class="spl-schema-object">`)
		buf.WriteString(fmt.Sprintf(`<legend class="spl-schema-legend">%s`, html.EscapeString(displayName)))
		if required {
			buf.WriteString(` <span class="spl-schema-required" title="Required">*</span>`)
		}
		buf.WriteString(`</legend>`)
		if s.Description != "" {
			buf.WriteString(fmt.Sprintf(`<p class="spl-schema-desc">%s</p>`, html.EscapeString(s.Description)))
		}
		var valMap map[string]any
		if m, ok := value.(map[string]any); ok {
			valMap = m
		} else {
			valMap = make(map[string]any)
		}
		for _, prop := range orderedProps(s) {
			propSchema := s.Properties[prop]
			propPath := path + "." + prop
			propSchema.renderFieldSPL(buf, fieldComponent, editHandler, arrayHandler, dataSignal, prop, propPath, valMap[prop], s.isRequired(prop), depth+1, arrayLevel)
		}
		buf.WriteString(`</fieldset>`)
	case "array":
		s.renderArrayFieldSPL(buf, fieldComponent, editHandler, arrayHandler, dataSignal, name, path, value, required, depth, arrayLevel)
	case "textarea":
		rows := 4
		if s.uiRows() > 0 {
			rows = s.uiRows()
		}
		buf.WriteString(fmt.Sprintf(`@render(%s, {"label": %s, "required": %t, "description": %s}) {
  @fill("label") { <span for="%s">%s</span> }
  <textarea id="%s" name="%s" rows="%d" class="spl-schema-input" data-spl-model="%s" on:input="%s"%s%s>%s</textarea>
}`, strconv.Quote(fieldComponent), strconv.Quote(displayName), required, strconv.Quote(s.Description), fieldID, html.EscapeString(displayName), fieldID, fieldName, rows, modelPath, editHandler, extraAttrs, placeholder, html.EscapeString(formatValue(value))))
	case "select":
		buf.WriteString(fmt.Sprintf(`@render(%s, {"label": %s, "required": %t, "description": %s}) {
  @fill("label") { <span for="%s">%s</span> }
  <select id="%s" name="%s" class="spl-schema-input" data-spl-model="%s" on:change="%s"%s>`, strconv.Quote(fieldComponent), strconv.Quote(displayName), required, strconv.Quote(s.Description), fieldID, html.EscapeString(displayName), fieldID, fieldName, modelPath, editHandler, extraAttrs))
		if s.uiPlaceholder() != "" {
			buf.WriteString(fmt.Sprintf(`<option value="">%s</option>`, html.EscapeString(s.uiPlaceholder())))
		} else {
			buf.WriteString(`<option value="">Select...</option>`)
		}
		strVal := formatValue(value)
		for _, opt := range s.Enum {
			optStr := formatValue(opt)
			selected := ""
			if optStr == strVal {
				selected = ` selected`
			}
			buf.WriteString(fmt.Sprintf(`<option value="%s"%s>%s</option>`, html.EscapeString(optStr), selected, html.EscapeString(optStr)))
		}
		buf.WriteString(`</select>
}`)
	case "checkbox":
		checked := ""
		if b, ok := value.(bool); ok && b {
			checked = ` checked`
		}
		buf.WriteString(fmt.Sprintf(`@render(%s, {"label": %s, "required": %t, "description": %s}) {
  @fill("label") { <span>%s</span> }
  <input type="checkbox" id="%s" name="%s" class="spl-schema-input" data-spl-model="%s" on:change="%s"%s%s />
}`, strconv.Quote(fieldComponent), strconv.Quote(displayName), required, strconv.Quote(s.Description), html.EscapeString(displayName), fieldID, fieldName, modelPath, editHandler, extraAttrs, checked))
	default:
		buf.WriteString(fmt.Sprintf(`@render(%s, {"label": %s, "required": %t, "description": %s}) {
  @fill("label") { <span for="%s">%s</span> }
  <input type="%s" id="%s" name="%s" value="%s" class="spl-schema-input" data-spl-model="%s" on:input="%s"%s%s />
}`, strconv.Quote(fieldComponent), strconv.Quote(displayName), required, strconv.Quote(s.Description), fieldID, html.EscapeString(displayName), html.EscapeString(inpType), fieldID, fieldName, html.EscapeString(formatValue(value)), modelPath, editHandler, extraAttrs, placeholder))
	}
}

func (s *Schema) renderArrayFieldSPL(buf *strings.Builder, fieldComponent, editHandler, arrayHandler, dataSignal, name, path string, value any, required bool, depth, arrayLevel int) {
	displayName := s.Title
	if displayName == "" {
		displayName = name
	}
	fullPath := dataSignal + "." + path
	addable := s.uiBoolOption(true, "add", "addable", "canAdd")
	removable := s.uiBoolOption(true, "remove", "removable", "canRemove")
	reorderable := s.uiBoolOption(true, "reorder", "orderable", "sortable", "canReorder")
	addLabel := s.uiStringOption("Add item", "addLabel", "addButtonLabel")
	removeLabel := s.uiStringOption("Remove", "removeLabel", "removeButtonLabel")
	moveUpLabel := s.uiStringOption("Up", "moveUpLabel", "moveUpButtonLabel")
	moveDownLabel := s.uiStringOption("Down", "moveDownLabel", "moveDownButtonLabel")
	actions := s.uiArrayActions()
	arrayActions, itemActions := splitSchemaArrayActions(actions)
	if len(actions) == 0 {
		if addable {
			arrayActions = append(arrayActions, schemaArrayActionConfig{Label: addLabel, Action: "add", Scope: "array"})
		}
		if reorderable {
			itemActions = append(itemActions,
				schemaArrayActionConfig{Label: moveUpLabel, Action: "move", Scope: "item", Direction: -1},
				schemaArrayActionConfig{Label: moveDownLabel, Action: "move", Scope: "item", Direction: 1},
			)
		}
		if removable {
			itemActions = append(itemActions, schemaArrayActionConfig{Label: removeLabel, Action: "remove", Scope: "item"})
		}
	}
	defaultItem := any("")
	if s.Items != nil {
		defaultItem = s.Items.defaultValue()
	}
	defaultJSON, _ := json.Marshal(defaultItem)
	if len(defaultJSON) == 0 {
		defaultJSON = []byte(`null`)
	}
	minItems := ""
	if s.MinItems != nil {
		minItems = strconv.Itoa(*s.MinItems)
	}
	maxItems := ""
	if s.MaxItems != nil {
		maxItems = strconv.Itoa(*s.MaxItems)
	}
	emptyMessage, minItemsMessage, maxItemsMessage := s.arrayMessages()
	var itemTemplate strings.Builder
	if s.Items != nil {
		s.Items.renderArrayItemTemplateSPL(&itemTemplate, fieldComponent, editHandler, arrayHandler, displayName, arrayLevel, itemActions)
	} else {
		(&Schema{Type: SchemaTypeString}).renderArrayItemTemplateSPL(&itemTemplate, fieldComponent, editHandler, arrayHandler, displayName, arrayLevel, itemActions)
	}
	buf.WriteString(fmt.Sprintf(`<div class="spl-schema-array" data-spl-schema-array="true" data-spl-schema-array-path="%s" data-spl-schema-array-level="%d" data-spl-schema-array-min="%s" data-spl-schema-array-max="%s" data-spl-schema-array-add="%t" data-spl-schema-array-remove="%t" data-spl-schema-array-reorder="%t" data-spl-schema-array-default="%s" data-spl-schema-array-empty-message="%s" data-spl-schema-array-min-message="%s" data-spl-schema-array-max-message="%s">
  <div class="spl-schema-array-head">
    <div>
      <div class="spl-schema-label">%s`, html.EscapeString(fullPath), arrayLevel, html.EscapeString(minItems), html.EscapeString(maxItems), addable, removable, reorderable, html.EscapeString(string(defaultJSON)), html.EscapeString(emptyMessage), html.EscapeString(minItemsMessage), html.EscapeString(maxItemsMessage), html.EscapeString(displayName)))
	if required {
		buf.WriteString(` <span class="spl-schema-required" title="Required">*</span>`)
	}
	buf.WriteString(`</div>`)
	if s.Description != "" {
		buf.WriteString(fmt.Sprintf(`<p class="spl-schema-desc">%s</p>`, html.EscapeString(s.Description)))
	}
	buf.WriteString(`</div>`)
	if len(arrayActions) > 0 {
		buf.WriteString(`<div class="spl-schema-array-actions">`)
		for _, action := range arrayActions {
			writeSchemaArrayActionButton(buf, action, fullPath, "", arrayHandler, "")
		}
		buf.WriteString(`</div>`)
	}
	buf.WriteString(`</div>
  <div class="spl-schema-array-message" aria-live="polite"></div>
  <div class="spl-schema-array-items"></div>
  <template data-spl-schema-array-template>`)
	buf.WriteString(itemTemplate.String())
	buf.WriteString(`</template>
</div>`)
}

func (s *Schema) renderArrayItemTemplateSPL(buf *strings.Builder, fieldComponent, editHandler, arrayHandler, label string, level int, itemActions []schemaArrayActionConfig) {
	parentToken := fmt.Sprintf("__SPL_ARRAY_PARENT_PATH_%d__", level)
	pathToken := fmt.Sprintf("__SPL_ARRAY_PATH_%d__", level)
	indexToken := fmt.Sprintf("__SPL_ARRAY_INDEX_%d__", level)
	positionToken := fmt.Sprintf("__SPL_ARRAY_POSITION_%d__", level)
	removeDisabledToken := fmt.Sprintf("__SPL_ARRAY_REMOVE_DISABLED_%d__", level)
	upDisabledToken := fmt.Sprintf("__SPL_ARRAY_MOVE_UP_DISABLED_%d__", level)
	downDisabledToken := fmt.Sprintf("__SPL_ARRAY_MOVE_DOWN_DISABLED_%d__", level)
	buf.WriteString(fmt.Sprintf(`<div class="spl-schema-array-item" data-spl-schema-array-index="%s">
  <div class="spl-schema-array-item-head">
    <span class="spl-schema-array-item-title">%s %s</span>
    <div class="spl-schema-array-item-actions">`, indexToken, html.EscapeString(label), positionToken))
	for _, action := range itemActions {
		disabledToken := ""
		if action.Action == "remove" {
			disabledToken = removeDisabledToken
		} else if action.Action == "move" && action.Direction < 0 {
			disabledToken = upDisabledToken
		} else if action.Action == "move" && action.Direction > 0 {
			disabledToken = downDisabledToken
		}
		writeSchemaArrayActionButton(buf, action, parentToken, indexToken, arrayHandler, disabledToken)
	}
	buf.WriteString(`</div>
  </div>`)
	switch s.inputType() {
	case "object":
		buf.WriteString(`<fieldset class="spl-schema-object spl-schema-array-object">`)
		if s.Description != "" {
			buf.WriteString(fmt.Sprintf(`<p class="spl-schema-desc">%s</p>`, html.EscapeString(s.Description)))
		}
		for _, prop := range orderedProps(s) {
			propSchema := s.Properties[prop]
			propSchema.renderTemplateFieldSPL(buf, fieldComponent, editHandler, arrayHandler, prop, pathToken+"."+prop, s.isRequired(prop), level+1)
		}
		buf.WriteString(`</fieldset>`)
	case "array":
		s.renderTemplateArrayFieldSPL(buf, fieldComponent, editHandler, arrayHandler, label, pathToken, false, level+1)
	default:
		s.renderTemplateFieldSPL(buf, fieldComponent, editHandler, arrayHandler, label, pathToken, false, level+1)
	}
	buf.WriteString(`</div>`)
}

func (s *Schema) renderTemplateFieldSPL(buf *strings.Builder, fieldComponent, editHandler, arrayHandler, name, fullPath string, required bool, level int) {
	if s.uiHidden() {
		return
	}
	displayName := s.Title
	if displayName == "" {
		displayName = name
	}
	fieldID := strings.ReplaceAll(fullPath, ".", "_")
	extraAttrs := s.splInputAttrs(required)
	if s.uiDisabled() {
		extraAttrs += ` disabled`
	}
	placeholder := ""
	if s.uiPlaceholder() != "" {
		placeholder = fmt.Sprintf(` placeholder="%s"`, html.EscapeString(s.uiPlaceholder()))
	}
	switch s.inputType() {
	case "object":
		buf.WriteString(fmt.Sprintf(`<fieldset class="spl-schema-object"><legend class="spl-schema-legend">%s</legend>`, html.EscapeString(displayName)))
		for _, prop := range orderedProps(s) {
			propSchema := s.Properties[prop]
			propSchema.renderTemplateFieldSPL(buf, fieldComponent, editHandler, arrayHandler, prop, fullPath+"."+prop, s.isRequired(prop), level)
		}
		buf.WriteString(`</fieldset>`)
	case "array":
		s.renderTemplateArrayFieldSPL(buf, fieldComponent, editHandler, arrayHandler, displayName, fullPath, required, level)
	case "textarea":
		rows := 4
		if s.uiRows() > 0 {
			rows = s.uiRows()
		}
		buf.WriteString(fmt.Sprintf(`<div class="spl-schema-field"><label class="spl-schema-label" for="%s">%s</label><textarea id="%s" name="%s" rows="%d" class="spl-schema-input" data-spl-model="%s" data-spl-on-input="%s"%s%s></textarea>`, html.EscapeString(fieldID), html.EscapeString(displayName), html.EscapeString(fieldID), html.EscapeString(fullPath), rows, html.EscapeString(fullPath), html.EscapeString(editHandler), extraAttrs, placeholder))
		if s.Description != "" {
			buf.WriteString(fmt.Sprintf(`<p class="spl-schema-desc">%s</p>`, html.EscapeString(s.Description)))
		}
		buf.WriteString(`</div>`)
	case "select":
		buf.WriteString(fmt.Sprintf(`<div class="spl-schema-field"><label class="spl-schema-label" for="%s">%s</label><select id="%s" name="%s" class="spl-schema-input" data-spl-model="%s" data-spl-on-change="%s"%s>`, html.EscapeString(fieldID), html.EscapeString(displayName), html.EscapeString(fieldID), html.EscapeString(fullPath), html.EscapeString(fullPath), html.EscapeString(editHandler), extraAttrs))
		if s.uiPlaceholder() != "" {
			buf.WriteString(fmt.Sprintf(`<option value="">%s</option>`, html.EscapeString(s.uiPlaceholder())))
		} else {
			buf.WriteString(`<option value="">Select...</option>`)
		}
		for _, opt := range s.Enum {
			optStr := formatValue(opt)
			buf.WriteString(fmt.Sprintf(`<option value="%s">%s</option>`, html.EscapeString(optStr), html.EscapeString(optStr)))
		}
		buf.WriteString(`</select></div>`)
	case "checkbox":
		buf.WriteString(fmt.Sprintf(`<div class="spl-schema-field spl-schema-field-checkbox"><label class="spl-schema-label-checkbox"><input type="checkbox" id="%s" name="%s" class="spl-schema-input" data-spl-model="%s" data-spl-on-change="%s"%s /> %s</label></div>`, html.EscapeString(fieldID), html.EscapeString(fullPath), html.EscapeString(fullPath), html.EscapeString(editHandler), extraAttrs, html.EscapeString(displayName)))
	default:
		buf.WriteString(fmt.Sprintf(`<div class="spl-schema-field"><label class="spl-schema-label" for="%s">%s</label><input type="%s" id="%s" name="%s" class="spl-schema-input" data-spl-model="%s" data-spl-on-input="%s"%s%s />`, html.EscapeString(fieldID), html.EscapeString(displayName), html.EscapeString(s.inputType()), html.EscapeString(fieldID), html.EscapeString(fullPath), html.EscapeString(fullPath), html.EscapeString(editHandler), extraAttrs, placeholder))
		if s.Description != "" {
			buf.WriteString(fmt.Sprintf(`<p class="spl-schema-desc">%s</p>`, html.EscapeString(s.Description)))
		}
		buf.WriteString(`</div>`)
	}
}

func (s *Schema) renderTemplateArrayFieldSPL(buf *strings.Builder, fieldComponent, editHandler, arrayHandler, label, fullPath string, required bool, level int) {
	if s.uiHidden() {
		return
	}
	addable := s.uiBoolOption(true, "add", "addable", "canAdd")
	removable := s.uiBoolOption(true, "remove", "removable", "canRemove")
	reorderable := s.uiBoolOption(true, "reorder", "orderable", "sortable", "canReorder")
	addLabel := s.uiStringOption("Add item", "addLabel", "addButtonLabel")
	removeLabel := s.uiStringOption("Remove", "removeLabel", "removeButtonLabel")
	moveUpLabel := s.uiStringOption("Up", "moveUpLabel", "moveUpButtonLabel")
	moveDownLabel := s.uiStringOption("Down", "moveDownLabel", "moveDownButtonLabel")
	actions := s.uiArrayActions()
	arrayActions, itemActions := splitSchemaArrayActions(actions)
	if len(actions) == 0 {
		if addable {
			arrayActions = append(arrayActions, schemaArrayActionConfig{Label: addLabel, Action: "add", Scope: "array"})
		}
		if reorderable {
			itemActions = append(itemActions,
				schemaArrayActionConfig{Label: moveUpLabel, Action: "move", Scope: "item", Direction: -1},
				schemaArrayActionConfig{Label: moveDownLabel, Action: "move", Scope: "item", Direction: 1},
			)
		}
		if removable {
			itemActions = append(itemActions, schemaArrayActionConfig{Label: removeLabel, Action: "remove", Scope: "item"})
		}
	}
	defaultItem := any("")
	if s.Items != nil {
		defaultItem = s.Items.defaultValue()
	}
	defaultJSON, _ := json.Marshal(defaultItem)
	if len(defaultJSON) == 0 {
		defaultJSON = []byte(`null`)
	}
	minItems := ""
	if s.MinItems != nil {
		minItems = strconv.Itoa(*s.MinItems)
	}
	maxItems := ""
	if s.MaxItems != nil {
		maxItems = strconv.Itoa(*s.MaxItems)
	}
	emptyMessage, minItemsMessage, maxItemsMessage := s.arrayMessages()
	var itemTemplate strings.Builder
	if s.Items != nil {
		s.Items.renderArrayItemTemplateSPL(&itemTemplate, fieldComponent, editHandler, arrayHandler, label, level, itemActions)
	}
	buf.WriteString(fmt.Sprintf(`<div class="spl-schema-array" data-spl-schema-array="true" data-spl-schema-array-path="%s" data-spl-schema-array-level="%d" data-spl-schema-array-min="%s" data-spl-schema-array-max="%s" data-spl-schema-array-add="%t" data-spl-schema-array-remove="%t" data-spl-schema-array-reorder="%t" data-spl-schema-array-default="%s" data-spl-schema-array-empty-message="%s" data-spl-schema-array-min-message="%s" data-spl-schema-array-max-message="%s"><div class="spl-schema-array-head"><div class="spl-schema-label">%s`, html.EscapeString(fullPath), level, html.EscapeString(minItems), html.EscapeString(maxItems), addable, removable, reorderable, html.EscapeString(string(defaultJSON)), html.EscapeString(emptyMessage), html.EscapeString(minItemsMessage), html.EscapeString(maxItemsMessage), html.EscapeString(label)))
	if required {
		buf.WriteString(` <span class="spl-schema-required" title="Required">*</span>`)
	}
	buf.WriteString(`</div>`)
	if len(arrayActions) > 0 {
		buf.WriteString(`<div class="spl-schema-array-actions">`)
		for _, action := range arrayActions {
			writeSchemaArrayActionButton(buf, action, fullPath, "", arrayHandler, "")
		}
		buf.WriteString(`</div>`)
	}
	buf.WriteString(`</div><div class="spl-schema-array-message" aria-live="polite"></div><div class="spl-schema-array-items"></div><template data-spl-schema-array-template>`)
	buf.WriteString(itemTemplate.String())
	buf.WriteString(`</template></div>`)
}

func splitSchemaArrayActions(actions []schemaArrayActionConfig) ([]schemaArrayActionConfig, []schemaArrayActionConfig) {
	var arrayActions []schemaArrayActionConfig
	var itemActions []schemaArrayActionConfig
	for _, action := range actions {
		if action.Scope == "array" {
			arrayActions = append(arrayActions, action)
		} else {
			itemActions = append(itemActions, action)
		}
	}
	return arrayActions, itemActions
}

func writeSchemaArrayActionButton(buf *strings.Builder, action schemaArrayActionConfig, path, index, handler, disabledToken string) {
	className := strings.TrimSpace(action.ClassName)
	if className == "" {
		if action.Scope == "array" && action.Action == "add" {
			className = "spl-schema-array-add"
		} else {
			className = "spl-schema-array-action"
		}
	}
	buf.WriteString(fmt.Sprintf(`<button class="%s" type="button" data-spl-schema-array-action="%s" data-spl-schema-array-path="%s"`, html.EscapeString(className), html.EscapeString(action.Action), html.EscapeString(path)))
	if index != "" {
		buf.WriteString(fmt.Sprintf(` data-spl-schema-array-index="%s"`, html.EscapeString(index)))
	}
	if action.Action == "move" {
		buf.WriteString(fmt.Sprintf(` data-spl-schema-array-direction="%d"`, action.Direction))
	}
	if action.HasValue {
		if b, err := json.Marshal(action.Value); err == nil {
			buf.WriteString(fmt.Sprintf(` data-spl-schema-array-value="%s"`, html.EscapeString(string(b))))
		}
	}
	buf.WriteString(fmt.Sprintf(` data-spl-on-click="%s"%s>%s</button>`, html.EscapeString(handler), disabledToken, html.EscapeString(action.Label)))
}

func (s *Schema) splInputAttrs(required bool) string {
	var attrs strings.Builder
	if required {
		attrs.WriteString(` required`)
	}
	if s.MinLength != nil {
		attrs.WriteString(fmt.Sprintf(` minlength="%d"`, *s.MinLength))
	}
	if s.MaxLength != nil {
		attrs.WriteString(fmt.Sprintf(` maxlength="%d"`, *s.MaxLength))
	}
	if s.Minimum != nil {
		attrs.WriteString(fmt.Sprintf(` min="%v"`, *s.Minimum))
	}
	if s.Maximum != nil {
		attrs.WriteString(fmt.Sprintf(` max="%v"`, *s.Maximum))
	}
	if s.Pattern != "" {
		attrs.WriteString(fmt.Sprintf(` pattern="%s"`, html.EscapeString(s.Pattern)))
	}
	if s.uiAccept() != "" {
		attrs.WriteString(fmt.Sprintf(` accept="%s"`, html.EscapeString(s.uiAccept())))
	}
	return attrs.String()
}

func (s *Schema) RenderDetailHTML(name string, data map[string]any) string {
	var buf strings.Builder
	buf.WriteString(`<div class="spl-schema-detail">`)
	if s.Title != "" {
		buf.WriteString(fmt.Sprintf(`<h3 class="spl-schema-title">%s</h3>`, html.EscapeString(s.Title)))
	}
	if s.Description != "" {
		buf.WriteString(fmt.Sprintf(`<p class="spl-schema-description">%s</p>`, html.EscapeString(s.Description)))
	}
	for _, prop := range orderedProps(s) {
		propSchema := s.Properties[prop]
		if propSchema.uiHidden() {
			continue
		}
		val := data[prop]
		displayName := propSchema.Title
		if displayName == "" {
			displayName = prop
		}
		formatted := formatValue(val)
		buf.WriteString(fmt.Sprintf(`  <div class="spl-schema-detail-row">
    <span class="spl-schema-detail-label">%s</span>
    <span class="spl-schema-detail-value">%s</span>
  </div>
`, html.EscapeString(displayName), html.EscapeString(formatted)))
	}
	buf.WriteString("</div>")
	return buf.String()
}

func (s *Schema) RenderDetailSPL(name string, data map[string]any, dataSignal string) string {
	var buf strings.Builder
	buf.WriteString(fmt.Sprintf("@reactive(%s) {", dataSignal))
	buf.WriteString(`<div class="spl-schema-detail" data-spl-schema-name="`)
	buf.WriteString(html.EscapeString(name))
	buf.WriteString(`">`)
	if s.Title != "" {
		buf.WriteString(fmt.Sprintf(`<h3 class="spl-schema-title">%s</h3>`, html.EscapeString(s.Title)))
	}
	if s.Description != "" {
		buf.WriteString(fmt.Sprintf(`<p class="spl-schema-description">%s</p>`, html.EscapeString(s.Description)))
	}
	for _, prop := range orderedProps(s) {
		propSchema := s.Properties[prop]
		if propSchema.uiHidden() {
			continue
		}
		displayName := propSchema.Title
		if displayName == "" {
			displayName = prop
		}
		buf.WriteString(fmt.Sprintf(`  <div class="spl-schema-detail-row">
    <span class="spl-schema-detail-label">%s</span>
    <span class="spl-schema-detail-value">${%s.%s}</span>
  </div>
`, html.EscapeString(displayName), dataSignal, prop))
	}
	buf.WriteString("</div>}")
	return buf.String()
}

func (s *Schema) RenderTableHTML(items []map[string]any) string {
	if len(items) == 0 {
		return `<div class="spl-schema-empty">No items</div>`
	}
	var buf strings.Builder
	buf.WriteString(`<table class="spl-schema-table">
<thead><tr>`)
	cols := orderedProps(s)
	for _, col := range cols {
		propSchema := s.Properties[col]
		if propSchema.uiHidden() {
			continue
		}
		displayName := propSchema.Title
		if displayName == "" {
			displayName = col
		}
		buf.WriteString(fmt.Sprintf(`<th>%s</th>`, html.EscapeString(displayName)))
	}
	buf.WriteString(`</tr></thead>
<tbody>`)
	for _, item := range items {
		buf.WriteString("\n  <tr>")
		for _, col := range cols {
			propSchema := s.Properties[col]
			if propSchema.uiHidden() {
				continue
			}
			val := item[col]
			formatted := formatValue(val)
			buf.WriteString(fmt.Sprintf(`<td>%s</td>`, html.EscapeString(formatted)))
		}
		buf.WriteString("</tr>")
	}
	buf.WriteString("\n</tbody>\n</table>")
	return buf.String()
}

func (s *Schema) ValidateAll(data map[string]any) map[string][]string {
	errors := make(map[string][]string)
	for name, prop := range s.Properties {
		val := data[name]
		required := s.isRequired(name)
		if errs := prop.validateValue(name, val, required); len(errs) > 0 {
			errors[name] = errs
		}
		if prop.Type == SchemaTypeObject && prop.Properties != nil {
			if subMap, ok := val.(map[string]any); ok {
				subErrs := prop.ValidateAll(subMap)
				for k, v := range subErrs {
					errors[name+"."+k] = v
				}
			}
		}
	}
	return errors
}
