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
	Title             string             `json:"title,omitempty"`
	Description       string             `json:"description,omitempty"`
	Type              SchemaType         `json:"type,omitempty"`
	Properties        map[string]*Schema `json:"properties,omitempty"`
	Items             *Schema            `json:"items,omitempty"`
	Required          []string           `json:"required,omitempty"`
	Enum              []any              `json:"enum,omitempty"`
	Format            string             `json:"format,omitempty"`
	Pattern           string             `json:"pattern,omitempty"`
	MinLength         *int               `json:"minLength,omitempty"`
	MaxLength         *int               `json:"maxLength,omitempty"`
	Minimum           *float64           `json:"minimum,omitempty"`
	Maximum           *float64           `json:"maximum,omitempty"`
	ExclusiveMinimum  *float64           `json:"exclusiveMinimum,omitempty"`
	ExclusiveMaximum  *float64           `json:"exclusiveMaximum,omitempty"`
	MultipleOf        *float64           `json:"multipleOf,omitempty"`
	MinItems          *int               `json:"minItems,omitempty"`
	MaxItems          *int               `json:"maxItems,omitempty"`
	Default           any                `json:"default,omitempty"`
	Examples          []any              `json:"examples,omitempty"`

	UIWidget      string         `json:"ui:widget,omitempty"`
	UIOptions     map[string]any `json:"ui:options,omitempty"`
	UIPlaceholder string         `json:"ui:placeholder,omitempty"`
	UIOrder       int            `json:"ui:order,omitempty"`
	UIHidden      bool           `json:"ui:hidden,omitempty"`
	UIDisabled    bool           `json:"ui:disabled,omitempty"`
	UIRows        int            `json:"ui:rows,omitempty"`
	UIAccept      string         `json:"ui:accept,omitempty"`
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
	if v, ok := m["ui:widget"]; ok {
		s.UIWidget, _ = v.(string)
	}
	if v, ok := m["ui:options"]; ok {
		s.UIOptions, _ = v.(map[string]any)
	}
	if v, ok := m["ui:placeholder"]; ok {
		s.UIPlaceholder, _ = v.(string)
	}
	if v, ok := m["ui:hidden"]; ok {
		s.UIHidden, _ = v.(bool)
	}
	if v, ok := m["ui:disabled"]; ok {
		s.UIDisabled, _ = v.(bool)
	}
	if v, ok := m["ui:rows"]; ok {
		if f, ok := toFloat64(v); ok {
			s.UIRows = int(f)
		}
	}
	if v, ok := m["ui:accept"]; ok {
		s.UIAccept, _ = v.(string)
	}
	return s, nil
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

func (s *Schema) inputType() string {
	if s.UIWidget != "" {
		return s.UIWidget
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
	Type        SchemaType `json:"type"`
	Required    bool       `json:"required,omitempty"`
	MinLength   *int       `json:"minLength,omitempty"`
	MaxLength   *int       `json:"maxLength,omitempty"`
	Minimum     *float64   `json:"minimum,omitempty"`
	Maximum     *float64   `json:"maximum,omitempty"`
	Pattern     string     `json:"pattern,omitempty"`
	MinItems    *int       `json:"minItems,omitempty"`
	MaxItems    *int       `json:"maxItems,omitempty"`
	Enum        []any      `json:"enum,omitempty"`
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
	if s.UIHidden {
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
	if s.UIPlaceholder != "" {
		ph = fmt.Sprintf(` placeholder="%s"`, html.EscapeString(s.UIPlaceholder))
	}
	dis := ""
	if s.UIDisabled {
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
		if s.UIRows > 0 {
			rows = s.UIRows
		}
		return fmt.Sprintf(`%s<div class="spl-schema-field">
%s  <label class="spl-schema-label" for="%s">%s%s</label>
%s  %s
%s  <textarea id="%s" name="%s" rows="%d" class="spl-schema-input"%s%s>%s</textarea>
%s</div>`, indent, indent, fieldID, dispName, reqMark, indent, desc, indent, fieldID, fieldName, rows, extraAttrs, dis, htmlVal, indent)

	case "select":
		var opts strings.Builder
		if s.UIPlaceholder != "" {
			opts.WriteString(fmt.Sprintf(`        <option value="">%s</option>`, html.EscapeString(s.UIPlaceholder)))
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
		o := prop.UIOrder
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
		if propSchema.UIHidden {
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
		if propSchema.UIHidden {
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
			if propSchema.UIHidden {
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
