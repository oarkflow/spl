package spl

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/oarkflow/interpreter"
)

// TranslationBundle holds locale key-value pairs with optional pluralization support.
type TranslationBundle struct {
	Locale string
	// Messages maps message keys to translations.
	// For pluralization, keys can be "key.zero", "key.one", "key.two", "key.few", "key.many", "key.other".
	Messages map[string]string
	// Parent is an optional fallback locale bundle.
	Parent *TranslationBundle
}

// get returns the translated message for the given key, falling back to parent and finally the key itself.
func (b *TranslationBundle) get(key string) string {
	if msg, ok := b.Messages[key]; ok {
		return msg
	}
	if b.Parent != nil {
		return b.Parent.get(key)
	}
	return key
}

// resolvePlural selects the appropriate plural form key based on CLDR plural rules.
// count is the numeric value for plural resolution.
func (b *TranslationBundle) resolvePlural(key string, count int) string {
	form := pluralForm(count)
	// Try specific plural form first: "key.other", "key.one", etc.
	for _, candidate := range pluralCandidates(key, form) {
		if msg, ok := b.Messages[candidate]; ok {
			return msg
		}
	}
	// Fall back to parent
	if b.Parent != nil {
		return b.Parent.resolvePlural(key, count)
	}
	// Fall back to base key
	if msg, ok := b.Messages[key]; ok {
		return msg
	}
	return key
}

// pluralCandidates generates ordered candidates for plural form resolution.
func pluralCandidates(key, form string) []string {
	return []string{key + "." + form, key + ".other", key}
}

// pluralForm returns the CLDR plural category for a given count.
// Supports English-style plural rules: 1 → "one", everything else → "other".
func pluralForm(count int) string {
	if count == 1 {
		return "one"
	}
	return "other"
}

// I18nConfig configures internationalization for an Engine.
type I18nConfig struct {
	// DefaultLocale is the locale used when no locale is specified in a @translate directive.
	DefaultLocale string
	// Bundles maps locale codes to their translation bundles.
	Bundles map[string]*TranslationBundle
	// FallbackLocale is used when a key is not found in the requested locale.
	FallbackLocale string
	mu             sync.RWMutex
}

// NewI18nConfig creates a new I18nConfig with the given default locale.
func NewI18nConfig(defaultLocale string) *I18nConfig {
	return &I18nConfig{
		DefaultLocale:  defaultLocale,
		Bundles:        make(map[string]*TranslationBundle),
		FallbackLocale: "en",
	}
}

// LoadMessages loads translations from a JSON byte slice for the given locale.
// JSON format: {"key": "value", "key.one": "value", ...}
func (c *I18nConfig) LoadMessages(locale string, data []byte) error {
	var messages map[string]string
	if err := json.Unmarshal(data, &messages); err != nil {
		return fmt.Errorf("i18n: failed to parse messages for %q: %w", locale, err)
	}
	bundle := &TranslationBundle{
		Locale:   locale,
		Messages: messages,
	}
	// Set up parent fallback chain
	if locale != c.FallbackLocale {
		if parent, ok := c.Bundles[c.FallbackLocale]; ok {
			bundle.Parent = parent
		}
	}
	c.mu.Lock()
	c.Bundles[locale] = bundle
	c.mu.Unlock()
	return nil
}

// LoadMessagesMap loads translations from a Go map for the given locale.
func (c *I18nConfig) LoadMessagesMap(locale string, messages map[string]string) {
	bundle := &TranslationBundle{
		Locale:   locale,
		Messages: messages,
	}
	if locale != c.FallbackLocale {
		if parent, ok := c.Bundles[c.FallbackLocale]; ok {
			bundle.Parent = parent
		}
	}
	c.mu.Lock()
	c.Bundles[locale] = bundle
	c.mu.Unlock()
}

// GetBundle returns the bundle for the given locale, or nil if not found.
func (c *I18nConfig) GetBundle(locale string) *TranslationBundle {
	c.mu.RLock()
	defer c.mu.RUnlock()
	b, _ := c.Bundles[locale]
	return b
}

// Translate resolves a translation key for the given locale and optional count.
func (c *I18nConfig) Translate(locale, key string, count *int) string {
	if locale == "" {
		locale = c.DefaultLocale
	}
	bundle := c.GetBundle(locale)
	if bundle == nil {
		// Fall back to default
		bundle = c.GetBundle(c.DefaultLocale)
	}
	if bundle == nil {
		return key
	}
	if count != nil {
		return bundle.resolvePlural(key, *count)
	}
	return bundle.get(key)
}

// TranslateNode represents a @translate directive in the template AST.
type TranslateNode struct {
	Key    string // SPL expression for the translation key
	Locale string // optional locale expression (empty = use default)
	Count  string // optional count expression for plurals (empty = no plural)
	Body   []Node // fallback content if key is not found
}

func (n *TranslateNode) nodeType() string { return "translate" }

// parseTranslate parses @translate directive:
// @translate("key") { fallback content }
// @translate("key", locale) { ... }
// @translate("key", locale, count) { ... }
func (p *parser) parseTranslate() (*TranslateNode, error) {
	p.advanceN(len("@translate") + 1) // skip "@translate("
	key := p.readUntil(',', ')')
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, p.errorf("@translate requires a key argument")
	}
	var locale, count string
	if p.peek() == ',' {
		p.advance() // skip ','
		p.skipWhitespace()
		locale = p.readUntil(',', ')')
		locale = strings.TrimSpace(locale)
	}
	if p.peek() == ',' {
		p.advance()
		p.skipWhitespace()
		count = p.readUntil(')')
		count = strings.TrimSpace(count)
	}
	if p.peek() == ')' {
		p.advance()
	}
	p.skipWhitespaceAndNewlines()
	var body []Node
	if p.peek() == '{' {
		p.advance()
		var err error
		body, err = p.parseNodes(true)
		if err != nil {
			return nil, p.errorf("@translate body: %w", err)
		}
	}
	return &TranslateNode{Key: key, Locale: locale, Count: count, Body: body}, nil
}

// readUntil reads characters until one of the given delimiters is found (not inside quotes or parens).
func (p *parser) readUntil(delims ...rune) string {
	var buf strings.Builder
	depth := 0
	inStr := rune(0)
	for !p.eof() {
		ch := p.peek()
		if inStr != 0 {
			buf.WriteRune(p.advance())
			if ch == '\\' && !p.eof() {
				buf.WriteRune(p.advance())
			} else if ch == inStr {
				inStr = 0
			}
			continue
		}
		switch ch {
		case '"', '\'', '`':
			inStr = ch
			buf.WriteRune(p.advance())
		case '(':
			depth++
			buf.WriteRune(p.advance())
		case ')':
			if depth == 0 {
				// Check if ')' is in our delims
				for _, d := range delims {
					if ch == d {
						return buf.String()
					}
				}
			}
			depth--
			buf.WriteRune(p.advance())
		default:
			// Check delimiters
			for _, d := range delims {
				if ch == d && depth == 0 {
					return buf.String()
				}
			}
			buf.WriteRune(p.advance())
		}
	}
	return buf.String()
}

// renderTranslate renders a @translate node.
func (e *Engine) renderTranslate(n *TranslateNode, env *interpreter.Environment, data map[string]any, depth int) (string, error) {
	// Evaluate key
	keyObj, err := e.evalExpr(n.Key, env)
	if err != nil {
		return "", fmt.Errorf("@translate key: %w", err)
	}
	key := objectToString(keyObj)
	if key == "" {
		return "", nil
	}

	// Evaluate locale
	locale := ""
	if n.Locale != "" {
		localeObj, err := e.evalExpr(n.Locale, env)
		if err != nil {
			return "", fmt.Errorf("@translate locale: %w", err)
		}
		locale = objectToString(localeObj)
	}

	// Evaluate count
	var count *int
	if n.Count != "" {
		countObj, err := e.evalExpr(n.Count, env)
		if err != nil {
			return "", fmt.Errorf("@translate count: %w", err)
		}
		if v, ok := countObj.(*interpreter.Integer); ok {
			c := int(v.Value)
			count = &c
		}
	}

	if e.I18n != nil {
		translated := e.I18n.Translate(locale, key, count)
		if translated != key || len(n.Body) == 0 {
			return translated, nil
		}
	}

	// Fallback to body content
	return e.renderBody(n.Body, env, data, depth)
}

// parseCache parses @cache("key") { ... }
// @cache("key", ttl) { content }
// @cache("key", ttl, "dep1", "dep2") { content }
func (p *parser) parseCache() (*CacheNode, error) {
	p.advanceN(len("@cache") + 1) // skip "@cache("
	key := p.readUntil(',', ')')
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, p.errorf("@cache requires a key argument")
	}
	node := &CacheNode{Key: key}
	if p.peek() == ',' {
		p.advance()
		p.skipWhitespace()
		ttl := p.readUntil(',', ')')
		ttl = strings.TrimSpace(ttl)
		node.TTL = ttl
	}
	for p.peek() == ',' {
		p.advance()
		p.skipWhitespace()
		dep := p.readUntil(',', ')')
		dep = strings.TrimSpace(dep)
		if dep != "" {
			node.Deps = append(node.Deps, dep)
		}
	}
	if p.peek() == ')' {
		p.advance()
	}
	p.skipWhitespaceAndNewlines()
	if p.peek() != '{' {
		return nil, p.errorf("@cache: expected '{'")
	}
	p.advance()
	body, err := p.parseNodes(true)
	if err != nil {
		return nil, p.errorf("@cache body: %w", err)
	}
	node.Body = body
	return node, nil
}
