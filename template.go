package spl

import (
	"fmt"
	"io"
	"io/fs"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/oarkflow/interpreter"
)

// Cache size limits to prevent unbounded memory growth.
const (
	maxExprCacheSize         = 10000 // parsed expression ASTs
	maxExprMetaCacheSize     = 10000 // fast-path expression metadata
	maxFileCacheSize         = 1000  // parsed template files
	maxTmplCacheSize         = 500   // parsed template strings
	maxCompiledFileCacheSize = 500   // compiled file templates
	maxCompiledTextCacheSize = 500   // compiled text templates
	maxFragmentCacheSize     = 500   // cached rendered fragments
)

// CachePolicy controls TTL-based expiry per cache type.
// A zero-value CachePolicy means no TTL expiry (entries live until evicted by size).
type CachePolicy struct {
	// ExprTTL is the TTL for parsed expression ASTs and metadata. 0 = no expiry.
	ExprTTL int
	// FileTTL is the TTL for parsed template files. 0 = no expiry.
	FileTTL int
	// CompiledFileTTL is the TTL for compiled file templates. 0 = no expiry.
	CompiledFileTTL int
	// CompiledTextTTL is the TTL for compiled text templates. 0 = no expiry.
	CompiledTextTTL int
	// FragmentTTL is the TTL for @cache directive fragment cache entries. 0 = no expiry.
	FragmentTTL int
}

func (p CachePolicy) hasTTL() bool {
	return p.ExprTTL > 0 || p.FileTTL > 0 || p.CompiledFileTTL > 0 || p.CompiledTextTTL > 0 || p.FragmentTTL > 0
}

// cacheEntry wraps any cached value with its creation time for TTL checks.
type cacheEntry struct {
	created int64 // unix nano
}

// exprCacheValue holds a parsed expression program with timestamp.
type exprCacheValue struct {
	cacheEntry
	program *interpreter.Program
}

// exprMetaCacheValue holds expression fast-path metadata with timestamp.
type exprMetaCacheValue struct {
	cacheEntry
	meta exprFastPath
}

// nodesCacheValue holds a parsed node slice with timestamp.
type nodesCacheValue struct {
	cacheEntry
	nodes []Node
}

// compiledCacheValue holds a compiled template with timestamp.
type compiledCacheValue struct {
	cacheEntry
	ct *compiledTemplate
}

// fragmentCacheValue holds a rendered fragment string with timestamp.
type fragmentCacheValue struct {
	cacheEntry
	html string
}

// isExpired checks whether this entry has exceeded the given TTL (in seconds).
func (e cacheEntry) isExpired(ttl int) bool {
	if ttl <= 0 {
		return false
	}
	return time.Now().UnixNano()-e.created > int64(ttl)*1e9
}

// evictRandom removes ~20% of entries from a map by random sampling.
// Called under write lock. Uses random eviction for O(1) amortized cost.
func evictMap[K comparable, V any](m map[K]V, maxSize int) {
	if len(m) <= maxSize {
		return
	}
	toEvict := len(m) / 5 // remove 20%
	if toEvict < 1 {
		toEvict = 1
	}
	evicted := 0
	for k := range m {
		if evicted >= toEvict {
			break
		}
		// random skip to avoid always evicting the same iteration-order entries
		if rand.IntN(3) != 0 {
			continue
		}
		delete(m, k)
		evicted++
	}
	// if we didn't evict enough (unlucky random), do a second pass
	for k := range m {
		if evicted >= toEvict || len(m) <= maxSize {
			break
		}
		delete(m, k)
		evicted++
	}
}

// Filter transforms a value into a string. Extra positional args may be passed from the template syntax.
type Filter func(value any, args ...string) string

// exprKind classifies an expression for fast-path evaluation.
type exprKind int

const (
	exprGeneric     exprKind = iota // requires full interpreter.Eval
	exprIdent                       // simple identifier: ${name}
	exprDot                         // single dot access: ${item.name}
	exprStringLit                   // string literal: ${"hello"}
	exprIntLit                      // integer literal: ${42}
	exprBoolTrue                    // true
	exprBoolFalse                   // false
	exprConstHash                   // constant hash literal: {"key": "val", ...}
	exprCmpEqStr                    // identifier == "literal" or "literal" == identifier
	exprCmpNeqStr                   // identifier != "literal" or "literal" != identifier
	exprCmpEqIdent                  // identifier == identifier
	exprCmpNeqIdent                 // identifier != identifier
)

// exprFastPath holds pre-analyzed metadata for fast expression evaluation.
type exprFastPath struct {
	kind        exprKind
	ident       string             // for exprIdent: the variable name; for exprDot: the left identifier
	field       string             // for exprDot: the field name
	strVal      string             // for exprStringLit
	intVal      int64              // for exprIntLit
	constResult interpreter.Object // for exprConstHash: cached evaluation result (cloned on use)
}

// componentDef holds a registered component's body and declared props.
type componentDef struct {
	Body          []Node
	Props         []PropDef // declared prop definitions (may be empty)
	HasDynamicCSS bool      // true when <style> contains template expressions
}

type compiledTemplate struct {
	Nodes      []Node
	Components map[string]componentDef
	Imports    []string
}

// CacheStats exposes lightweight counts for the engine's in-memory caches.
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

// Engine is the main entry point for rendering SPL templates.
type Engine struct {
	BaseDir             string                     // directory for resolving includes/layouts
	FS                  fs.FS                      // optional embedded filesystem for loading templates
	DelimLeft           string                     // left interpolation delimiter (default: "${")
	DelimRight          string                     // right interpolation delimiter (default: "}")
	Filters             map[string]Filter          // registered filters
	Globals             map[string]any             // global template variables merged into every render
	AutoEscape          bool                       // auto HTML-escape ${} output (default: true)
	MaxDepth            int                        // max include/layout nesting depth (default: 64)
	Components          map[string]componentDef    // registered reusable components
	slotStack           []*slotContext             // stack for nested component slot contexts
	parentBlockStack    []parentBlockContext       // stack for @parent inside layout block overrides
	watchState          map[string]string          // @watch: expr → last evaluated value string
	exprCache           map[string]exprCacheValue  // cached parsed expression ASTs
	fileCache           map[string]nodesCacheValue // cached parsed template files by resolved path
	tmplCache           map[string]nodesCacheValue // cached parsed template strings
	compiledFileCache   map[string]compiledCacheValue
	compiledTextCache   map[string]compiledCacheValue
	baseEnv             *interpreter.Environment // base environment for the current render call
	globalEnv           *interpreter.Environment // cached global environment (created once)
	hydration           *hydrationState          // SSR hydration state for the current render call
	assetScopeSeq       int                      // per-render sequence for data-spl-unique assets
	assetMode           string                   // per-render asset policy: ""/"dedupe" or "raw"
	localSignalSeq      int                      // per-render sequence for @local signal aliases
	localSignals        map[string]string        // scoped signal aliases visible during render
	HydrationRuntimeURL string                   // if set, emit <script src="..."> instead of inlining runtime
	CSPNonce            string                   // optional nonce applied to executable hydration script tags
	SecureMode          bool                     // enforce CSP-safe, non-eval hydration output (default: false)
	DisableDebug        bool                     // exclude debug/getRenderStats from hydration runtime
	DisableAPI          bool                     // exclude API integration (patchAPI, serializeForm, apiParse) from hydration runtime
	mu                  *sync.RWMutex

	// Fast-path expression metadata cache
	exprMeta map[string]exprMetaCacheValue // expression string → fast-path info

	// Helpers holds user-registered functions callable from template expressions.
	// Keys are function names, values are Go functions that accept and return any.
	Helpers map[string]any

	// Holds lifecycle hook registrations.
	Hooks EngineHooks

	// I18n configures internationalization for @translate directives.
	I18n *I18nConfig

	// CachePolicy controls TTL-based expiry per cache type.
	CachePolicy CachePolicy

	// fragmentCache stores rendered @cache directive output.
	fragmentCache map[string]fragmentCacheValue

	// Schema-driven UI generation
	SchemaRegistry *SchemaRegistry
}

// New creates a new Engine with sensible defaults.
func New() *Engine {
	e := &Engine{
		BaseDir:           ".",
		Filters:           make(map[string]Filter),
		Globals:           make(map[string]any),
		AutoEscape:        true,
		MaxDepth:          64,
		Components:        make(map[string]componentDef),
		exprCache:         make(map[string]exprCacheValue),
		exprMeta:          make(map[string]exprMetaCacheValue),
		fileCache:         make(map[string]nodesCacheValue),
		tmplCache:         make(map[string]nodesCacheValue),
		compiledFileCache: make(map[string]compiledCacheValue),
		compiledTextCache: make(map[string]compiledCacheValue),
		fragmentCache:     make(map[string]fragmentCacheValue),
		SecureMode:        false,
		Helpers:           make(map[string]any),
		mu:                &sync.RWMutex{},
		SchemaRegistry:    NewSchemaRegistry(),
	}
	cacheRegistry.mu.Lock()
	cacheRegistry.engines = append(cacheRegistry.engines, e)
	cacheRegistry.mu.Unlock()
	registerBuiltinFilters(e)
	registerBuiltinHelpers(e)
	return e
}

// RegisterFilter adds or replaces a named filter.
func (e *Engine) RegisterFilter(name string, fn Filter) {
	e.mu.Lock()
	e.Filters[name] = fn
	e.mu.Unlock()
}

// RenderHook is a lifecycle hook called during template rendering.
// The RenderContext provides information about the current render operation.
type RenderContext struct {
	// Template is the raw template string being rendered.
	Template string
	// Path is the file path if rendering from a file.
	Path string
	// Data is the merged template data (globals + caller data).
	Data map[string]any
	// Depth is the current nesting depth.
	Depth int
}

// RenderHookFunc is a callback invoked during the render lifecycle.
// Return an error to abort rendering.
type RenderHookFunc func(ctx *RenderContext) error

// EngineHooks holds lifecycle hook registration points.
type EngineHooks struct {
	// BeforeRender is called before each top-level render begins.
	BeforeRender RenderHookFunc

	// AfterRender is called after each top-level render completes successfully.
	AfterRender RenderHookFunc

	// BeforeTemplateLoad is called before a template file is loaded from disk or FS.
	BeforeTemplateLoad RenderHookFunc

	// OnError is called when a render error occurs.
	OnError func(ctx *RenderContext, err error)
}

// HelperFunc is a function that can be registered via RegisterHelper and called
// from template expressions. It receives natively converted Go values and returns
// a value that will be converted back to an interpreter object.
type HelperFunc func(args ...any) any

// RegisterHelper registers a Go function that can be called from template expressions.
// Helpers are invoked in expressions like: ${helperName(arg1, arg2)}
// The function receives native Go values (string, float64, bool, map, slice, nil)
// and should return a value that SPL can convert to an interpreter object.
func (e *Engine) RegisterHelper(name string, fn HelperFunc) {
	e.mu.Lock()
	e.Helpers[name] = fn
	e.mu.Unlock()
}

// injectHelpers sets up all registered helpers in the given environment.
func (e *Engine) injectHelpers(env *interpreter.Environment) {
	e.mu.RLock()
	for name, fn := range e.Helpers {
		// Capture fn in loop variable
		helper := fn.(HelperFunc)
		builtin := &interpreter.Builtin{
			Fn: func(args ...interpreter.Object) interpreter.Object {
				nativeArgs := make([]any, len(args))
				for i, arg := range args {
					nativeArgs[i] = objectToNative(arg)
				}
				result := helper(nativeArgs...)
				return nativeToObject(result)
			},
		}
		env.Set(name, builtin)
	}
	e.mu.RUnlock()
}

// RegisterComponent parses a component body and registers it by name.
func (e *Engine) RegisterComponent(name string, body string) error {
	nodes, err := e.parseWithEngineDelims(body)
	if err != nil {
		return fmt.Errorf("component %q parse error: %w", name, err)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.Components[name] = componentDef{Body: nodes, HasDynamicCSS: bodyHasDynamicCSS(nodes)}
	return nil
}

// newGlobalEnv returns a cached global environment for template rendering.
// Thread-safe: uses RWMutex to protect lazy initialization.
func (e *Engine) newGlobalEnv() *interpreter.Environment {
	e.mu.RLock()
	env := e.globalEnv
	e.mu.RUnlock()
	if env != nil {
		return env
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.globalEnv == nil {
		e.globalEnv = interpreter.NewGlobalEnvironment([]string{})
	}
	return e.globalEnv
}

// Render parses and renders a template string with the given data.
func (e *Engine) Render(tmpl string, data map[string]any) (string, error) {
	if e.Hooks.BeforeRender != nil {
		if err := e.Hooks.BeforeRender(&RenderContext{Template: tmpl, Data: data}); err != nil {
			return "", fmt.Errorf("before render hook: %w", err)
		}
	}
	compiled, err := e.compileStringTemplate(tmpl)
	if err != nil {
		return "", fmt.Errorf("template parse error: %w", err)
	}
	out, err := e.renderCompiled(compiled, data, e.hydration)
	if err != nil {
		if e.Hooks.OnError != nil {
			e.Hooks.OnError(&RenderContext{Template: tmpl, Data: data}, err)
		}
		return "", err
	}
	if err := e.ensureSecureRenderedHTML(out); err != nil {
		if e.Hooks.OnError != nil {
			e.Hooks.OnError(&RenderContext{Template: tmpl, Data: data}, err)
		}
		return "", err
	}
	if e.Hooks.AfterRender != nil {
		if err := e.Hooks.AfterRender(&RenderContext{Template: tmpl, Data: data}); err != nil {
			return "", fmt.Errorf("after render hook: %w", err)
		}
	}
	return out, nil
}

// RenderFile loads a template file relative to BaseDir and renders it.
func (e *Engine) RenderFile(path string, data map[string]any) (string, error) {
	resolved, err := e.resolvePath(path)
	if err != nil {
		return "", fmt.Errorf("template file error (%s): %w", path, err)
	}
	if e.Hooks.BeforeTemplateLoad != nil {
		if err := e.Hooks.BeforeTemplateLoad(&RenderContext{Path: resolved, Data: data}); err != nil {
			return "", fmt.Errorf("before template load hook: %w", err)
		}
	}
	if e.Hooks.BeforeRender != nil {
		if err := e.Hooks.BeforeRender(&RenderContext{Path: resolved, Data: data}); err != nil {
			return "", fmt.Errorf("before render hook: %w", err)
		}
	}
	compiled, err := e.compileFileTemplate(resolved)
	if err != nil {
		return "", fmt.Errorf("template file error (%s): %w", path, err)
	}
	out, err := e.renderCompiled(compiled, data, e.hydration)
	if err != nil {
		if e.Hooks.OnError != nil {
			e.Hooks.OnError(&RenderContext{Path: resolved, Data: data}, err)
		}
		return "", err
	}
	if err := e.ensureSecureRenderedHTML(out); err != nil {
		if e.Hooks.OnError != nil {
			e.Hooks.OnError(&RenderContext{Path: resolved, Data: data}, err)
		}
		return "", err
	}
	if e.Hooks.AfterRender != nil {
		if err := e.Hooks.AfterRender(&RenderContext{Path: resolved, Data: data}); err != nil {
			return "", fmt.Errorf("after render hook: %w", err)
		}
	}
	return out, nil
}

// RenderSSRFile renders a template file and injects hydration metadata.
func (e *Engine) RenderSSRFile(path string, data map[string]any) (string, error) {
	resolved, err := e.resolvePath(path)
	if err != nil {
		return "", fmt.Errorf("template file error (%s): %w", path, err)
	}
	compiled, err := e.compileFileTemplate(resolved)
	if err != nil {
		return "", fmt.Errorf("template file error (%s): %w", path, err)
	}
	state := &hydrationState{Signals: make(map[string]any)}
	renderer := e.cloneForRender(state, e.cloneRegisteredComponents())
	out, err := renderer.renderCompiled(compiled, data, state)
	if err != nil {
		return "", err
	}
	if err := renderer.ensureSecureRenderedHTML(out); err != nil {
		return "", err
	}
	out, err = renderer.prepareHydrationOutput(out)
	if err != nil {
		return "", err
	}
	return out + renderer.renderHydrationScript(out), nil
}

// RenderStreamFile streams a parsed template file to a writer.
func (e *Engine) RenderStreamFile(w io.Writer, path string, data map[string]any) error {
	resolved, err := e.resolvePath(path)
	if err != nil {
		return fmt.Errorf("template file error (%s): %w", path, err)
	}
	compiled, err := e.compileFileTemplate(resolved)
	if err != nil {
		return fmt.Errorf("template file error (%s): %w", path, err)
	}
	return NewStreamRenderer(e, data).RenderStreamNodes(w, compiled.Nodes)
}

// InvalidateCaches clears parsed template caches so subsequent renders re-read source.
// Expression caches (parsed ASTs and metadata) are preserved since they are
// independent of template files and rarely need invalidation.
// Use ClearAllCaches to also clear expression and fragment caches.
func (e *Engine) InvalidateCaches() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.fileCache = make(map[string]nodesCacheValue)
	e.tmplCache = make(map[string]nodesCacheValue)
	e.compiledFileCache = make(map[string]compiledCacheValue)
	e.compiledTextCache = make(map[string]compiledCacheValue)
	e.fragmentCache = make(map[string]fragmentCacheValue)
	e.watchState = make(map[string]string)
}

// ClearAllCaches clears all caches including expression and fragment caches.
func (e *Engine) ClearAllCaches() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.fileCache = make(map[string]nodesCacheValue)
	e.tmplCache = make(map[string]nodesCacheValue)
	e.compiledFileCache = make(map[string]compiledCacheValue)
	e.compiledTextCache = make(map[string]compiledCacheValue)
	e.exprCache = make(map[string]exprCacheValue)
	e.exprMeta = make(map[string]exprMetaCacheValue)
	e.fragmentCache = make(map[string]fragmentCacheValue)
	e.watchState = make(map[string]string)
}

// CacheStats returns current cache entry counts for observability and tests.
func (e *Engine) CacheStats() CacheStats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return CacheStats{
		ParsedFiles:       len(e.fileCache),
		ParsedTemplates:   len(e.tmplCache),
		CompiledFiles:     len(e.compiledFileCache),
		CompiledTemplates: len(e.compiledTextCache),
		ExprPrograms:      len(e.exprCache),
		ExprFastPaths:     len(e.exprMeta),
		Components:        len(e.Components),
		GlobalEnvReady:    e.globalEnv != nil,
	}
}

// ClearFragmentCache clears cached @cache directive entries.
func (e *Engine) ClearFragmentCache() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.fragmentCache = make(map[string]fragmentCacheValue)
}

// loadFile reads and parses a template file, using the file cache.
// When e.FS is set, files are read from the embedded filesystem.
func (e *Engine) loadFile(resolved string) ([]Node, error) {
	e.mu.RLock()
	if entry, ok := e.fileCache[resolved]; ok {
		if !entry.isExpired(e.CachePolicy.FileTTL) {
			e.mu.RUnlock()
			return entry.nodes, nil
		}
		e.mu.RUnlock()
		// Expired — fall through to reload
	} else {
		e.mu.RUnlock()
	}
	var content []byte
	var err error
	if e.FS != nil {
		content, err = fs.ReadFile(e.FS, resolved)
	} else {
		content, err = os.ReadFile(resolved)
	}
	if err != nil {
		return nil, err
	}
	nodes, err := parse(string(content))
	if err != nil {
		return nil, err
	}
	e.mu.Lock()
	evictMap(e.fileCache, maxFileCacheSize)
	e.fileCache[resolved] = nodesCacheValue{
		cacheEntry: cacheEntry{created: time.Now().UnixNano()},
		nodes:      nodes,
	}
	e.mu.Unlock()
	return nodes, nil
}

func (e *Engine) cloneForRender(state *hydrationState, components map[string]componentDef) *Engine {
	globalEnv := e.newGlobalEnv()
	cloned := *e
	cloned.Components = components
	cloned.slotStack = nil
	cloned.parentBlockStack = nil
	cloned.watchState = nil // lazy-init in renderWatch
	cloned.baseEnv = interpreter.NewEnclosedEnvironment(globalEnv)
	cloned.hydration = state
	cloned.assetScopeSeq = 0
	cloned.assetMode = ""
	cloned.localSignalSeq = 0
	cloned.localSignals = nil
	cloned.injectHelpers(cloned.baseEnv)
	return &cloned
}

func cloneComponentDefs(src map[string]componentDef) map[string]componentDef {
	cloned := make(map[string]componentDef, len(src))
	for k, v := range src {
		cloned[k] = v
	}
	return cloned
}

func (e *Engine) cloneRegisteredComponents() map[string]componentDef {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return cloneComponentDefs(e.Components)
}

func (e *Engine) compileStringTemplate(tmpl string) (*compiledTemplate, error) {
	e.mu.RLock()
	if entry, ok := e.compiledTextCache[tmpl]; ok {
		if !entry.isExpired(e.CachePolicy.CompiledTextTTL) {
			e.mu.RUnlock()
			return entry.ct, nil
		}
	}
	e.mu.RUnlock()
	nodes, err := e.parseWithEngineDelims(tmpl)
	if err != nil {
		return nil, err
	}
	ct := e.buildCompiledTemplate(nodes)
	now := time.Now().UnixNano()
	e.mu.Lock()
	if entry, ok := e.compiledTextCache[tmpl]; ok && !entry.isExpired(e.CachePolicy.CompiledTextTTL) {
		e.mu.Unlock()
		return entry.ct, nil
	}
	evictMap(e.tmplCache, maxTmplCacheSize)
	e.tmplCache[tmpl] = nodesCacheValue{
		cacheEntry: cacheEntry{created: now},
		nodes:      nodes,
	}
	evictMap(e.compiledTextCache, maxCompiledTextCacheSize)
	e.compiledTextCache[tmpl] = compiledCacheValue{
		cacheEntry: cacheEntry{created: now},
		ct:         ct,
	}
	e.mu.Unlock()
	return ct, nil
}

func (e *Engine) compileFileTemplate(resolved string) (*compiledTemplate, error) {
	e.mu.RLock()
	if entry, ok := e.compiledFileCache[resolved]; ok {
		if !entry.isExpired(e.CachePolicy.CompiledFileTTL) {
			e.mu.RUnlock()
			return entry.ct, nil
		}
	}
	e.mu.RUnlock()
	nodes, err := e.loadFile(resolved)
	if err != nil {
		return nil, err
	}
	ct := e.buildCompiledTemplate(nodes)
	now := time.Now().UnixNano()
	e.mu.Lock()
	if entry, ok := e.compiledFileCache[resolved]; ok && !entry.isExpired(e.CachePolicy.CompiledFileTTL) {
		e.mu.Unlock()
		return entry.ct, nil
	}
	evictMap(e.compiledFileCache, maxCompiledFileCacheSize)
	e.compiledFileCache[resolved] = compiledCacheValue{
		cacheEntry: cacheEntry{created: now},
		ct:         ct,
	}
	e.mu.Unlock()
	return ct, nil
}

func bodyHasDynamicCSS(body []Node) bool {
	var sb strings.Builder
	for _, n := range body {
		switch v := n.(type) {
		case *TextNode:
			sb.WriteString(v.Text)
		case *ExprNode:
			sb.WriteString("${")
			sb.WriteString(v.Expr)
			sb.WriteString("}")
		}
	}
	raw := sb.String()
	lower := strings.ToLower(raw)
	start := strings.Index(lower, "<style")
	if start < 0 {
		return false
	}
	closeTag := strings.Index(raw[start:], "</style>")
	if closeTag < 0 {
		return false
	}
	section := raw[start : start+closeTag+8]
	return strings.Contains(section, "${")
}

func (e *Engine) buildCompiledTemplate(nodes []Node) *compiledTemplate {
	components := make(map[string]componentDef)
	imports := make([]string, 0)
	for _, n := range nodes {
		if c, ok := n.(*ComponentNode); ok {
			components[c.Name] = componentDef{Body: c.Body, Props: c.Props, HasDynamicCSS: bodyHasDynamicCSS(c.Body)}
			continue
		}
		if imp, ok := n.(*ImportNode); ok {
			imports = append(imports, imp.Path)
		}
	}
	return &compiledTemplate{Nodes: nodes, Components: components, Imports: imports}
}

func (e *Engine) renderCompiled(ct *compiledTemplate, data map[string]any, state *hydrationState) (string, error) {
	// Build components: start from engine's registered components, merge template's
	e.mu.RLock()
	engineCompCount := len(e.Components)
	e.mu.RUnlock()

	var components map[string]componentDef
	if engineCompCount == 0 && len(ct.Components) == 0 && len(ct.Imports) == 0 {
		// Fast path: no components anywhere — avoid map allocation entirely
		components = nil
	} else {
		components = make(map[string]componentDef, engineCompCount+len(ct.Components))
		e.mu.RLock()
		for k, v := range e.Components {
			components[k] = v
		}
		e.mu.RUnlock()

		if err := e.registerImportedComponents(ct.Imports, components, make(map[string]struct{})); err != nil {
			return "", err
		}
		for k, v := range ct.Components {
			components[k] = v
		}
	}

	// Create renderer directly — single environment creation, single clone
	globalEnv := e.newGlobalEnv()
	cloned := *e
	if components != nil {
		cloned.Components = components
	}
	cloned.slotStack = nil
	cloned.parentBlockStack = nil
	cloned.watchState = nil // lazy-init on first use
	cloned.baseEnv = interpreter.NewEnclosedEnvironment(globalEnv)
	cloned.hydration = state
	cloned.assetScopeSeq = 0
	cloned.assetMode = ""
	cloned.localSignalSeq = 0
	cloned.localSignals = nil
	cloned.injectHelpers(cloned.baseEnv)
	out, err := (&cloned).renderNodes(ct.Nodes, data, 0)
	if err != nil {
		return "", err
	}
	if cloned.assetMode == "raw" {
		return out, nil
	}
	return optimizeRenderedAssets(out), nil
}

func (e *Engine) registerImportedComponents(imports []string, components map[string]componentDef, seen map[string]struct{}) error {
	for _, path := range imports {
		resolved, err := e.resolvePath(path)
		if err != nil {
			return fmt.Errorf("import %s: %w", path, err)
		}
		if _, ok := seen[resolved]; ok {
			continue
		}
		seen[resolved] = struct{}{}
		ct, err := e.compileFileTemplate(resolved)
		if err != nil {
			return fmt.Errorf("import %s: %w", path, err)
		}
		if err := e.registerImportedComponents(ct.Imports, components, seen); err != nil {
			return err
		}
		for k, v := range ct.Components {
			components[k] = v
		}
	}
	return nil
}

func (e *Engine) resolvePath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("template path is required")
	}
	cleanPath := filepath.Clean(path)
	if filepath.IsAbs(cleanPath) {
		return "", fmt.Errorf("absolute template paths are not allowed")
	}
	if e.FS != nil {
		// When using an embedded filesystem, resolve relative to FS root
		clean := filepath.ToSlash(cleanPath)
		if strings.HasPrefix(clean, "../") || clean == ".." {
			return "", fmt.Errorf("template path escapes filesystem")
		}
		return clean, nil
	}
	baseDir := e.BaseDir
	if baseDir == "" {
		baseDir = "."
	}
	baseAbs, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("resolve base dir: %w", err)
	}
	resolved := filepath.Join(baseAbs, cleanPath)
	rel, err := filepath.Rel(baseAbs, resolved)
	if err != nil {
		return "", fmt.Errorf("resolve template path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("template path escapes base directory")
	}
	return resolved, nil
}

// resolveFSPath validates and cleans a template path for use with the embedded filesystem.
func (e *Engine) resolveFSPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("template path is required")
	}
	clean := filepath.ToSlash(filepath.Clean(path))
	if strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") || clean == ".." {
		return "", fmt.Errorf("invalid template path: %s", path)
	}
	return clean, nil
}

// loadFSFile reads and parses a template file from the embedded filesystem, using the file cache.
func (e *Engine) loadFSFile(resolved string) ([]Node, error) {
	e.mu.RLock()
	if entry, ok := e.fileCache[resolved]; ok && !entry.isExpired(e.CachePolicy.FileTTL) {
		e.mu.RUnlock()
		return entry.nodes, nil
	}
	e.mu.RUnlock()
	content, err := fs.ReadFile(e.FS, resolved)
	if err != nil {
		return nil, err
	}
	nodes, err := e.parseWithEngineDelims(string(content))
	if err != nil {
		return nil, err
	}
	e.mu.Lock()
	evictMap(e.fileCache, maxFileCacheSize)
	e.fileCache[resolved] = nodesCacheValue{
		cacheEntry: cacheEntry{created: time.Now().UnixNano()},
		nodes:      nodes,
	}
	e.mu.Unlock()
	return nodes, nil
}

// compileFSFileTemplate compiles a template from the embedded filesystem.
func (e *Engine) compileFSFileTemplate(resolved string) (*compiledTemplate, error) {
	e.mu.RLock()
	if entry, ok := e.compiledFileCache[resolved]; ok && !entry.isExpired(e.CachePolicy.CompiledFileTTL) {
		e.mu.RUnlock()
		return entry.ct, nil
	}
	e.mu.RUnlock()
	nodes, err := e.loadFSFile(resolved)
	if err != nil {
		return nil, err
	}
	ct := e.buildCompiledTemplate(nodes)
	now := time.Now().UnixNano()
	e.mu.Lock()
	if entry, ok := e.compiledFileCache[resolved]; ok && !entry.isExpired(e.CachePolicy.CompiledFileTTL) {
		e.mu.Unlock()
		return entry.ct, nil
	}
	evictMap(e.compiledFileCache, maxCompiledFileCacheSize)
	e.compiledFileCache[resolved] = compiledCacheValue{
		cacheEntry: cacheEntry{created: now},
		ct:         ct,
	}
	e.mu.Unlock()
	return ct, nil
}

// RenderFSFile renders a template from the embedded filesystem (e.FS) with the given data.
// The path is resolved relative to the embedded filesystem root.
func (e *Engine) RenderFSFile(path string, data map[string]any) (string, error) {
	if e.FS == nil {
		return "", fmt.Errorf("embedded filesystem not set (FS field is nil)")
	}
	resolved, err := e.resolveFSPath(path)
	if err != nil {
		return "", fmt.Errorf("template file error (%s): %w", path, err)
	}
	compiled, err := e.compileFSFileTemplate(resolved)
	if err != nil {
		return "", fmt.Errorf("template file error (%s): %w", path, err)
	}
	out, err := e.renderCompiled(compiled, data, e.hydration)
	if err != nil {
		return "", err
	}
	if err := e.ensureSecureRenderedHTML(out); err != nil {
		return "", err
	}
	return out, nil
}

// RenderFSFileSSR renders a template from the embedded filesystem with SSR hydration.
func (e *Engine) RenderFSFileSSR(path string, data map[string]any) (string, error) {
	if e.FS == nil {
		return "", fmt.Errorf("embedded filesystem not set (FS field is nil)")
	}
	resolved, err := e.resolveFSPath(path)
	if err != nil {
		return "", fmt.Errorf("template file error (%s): %w", path, err)
	}
	compiled, err := e.compileFSFileTemplate(resolved)
	if err != nil {
		return "", fmt.Errorf("template file error (%s): %w", path, err)
	}
	state := &hydrationState{Signals: make(map[string]any)}
	renderer := e.cloneForRender(state, e.cloneRegisteredComponents())
	out, err := renderer.renderCompiled(compiled, data, state)
	if err != nil {
		return "", err
	}
	if err := renderer.ensureSecureRenderedHTML(out); err != nil {
		return "", err
	}
	out, err = renderer.prepareHydrationOutput(out)
	if err != nil {
		return "", err
	}
	return out + renderer.renderHydrationScript(out), nil
}

// mergeData returns a new map with globals as defaults, overridden by data.
func (e *Engine) mergeData(data map[string]any) map[string]any {
	merged := make(map[string]any, len(e.Globals)+len(data))
	for k, v := range e.Globals {
		merged[k] = v
	}
	for k, v := range data {
		merged[k] = v
	}
	return merged
}

// analyzeExpr pre-analyzes a parsed program to determine if it can use a fast path.
func analyzeExpr(program *interpreter.Program) exprFastPath {
	if len(program.Statements) != 1 {
		return exprFastPath{kind: exprGeneric}
	}
	stmt, ok := program.Statements[0].(*interpreter.ExpressionStatement)
	if !ok {
		return exprFastPath{kind: exprGeneric}
	}
	return classifyExpr(stmt.Expression)
}

func classifyExpr(expr interpreter.Expression) exprFastPath {
	switch v := expr.(type) {
	case *interpreter.Identifier:
		return exprFastPath{kind: exprIdent, ident: v.Name}
	case *interpreter.DotExpression:
		if left, ok := v.Left.(*interpreter.Identifier); ok {
			return exprFastPath{kind: exprDot, ident: left.Name, field: v.Right.Name}
		}
	case *interpreter.StringLiteral:
		return exprFastPath{kind: exprStringLit, strVal: v.Value}
	case *interpreter.IntegerLiteral:
		return exprFastPath{kind: exprIntLit, intVal: v.Value}
	case *interpreter.BooleanLiteral:
		if v.Value {
			return exprFastPath{kind: exprBoolTrue}
		}
		return exprFastPath{kind: exprBoolFalse}
	case *interpreter.HashLiteral:
		if isConstHashLiteral(v) {
			return exprFastPath{kind: exprConstHash}
		}
	case *interpreter.InfixExpression:
		if v.Operator == "==" || v.Operator == "!=" {
			return classifyInfixCmp(v)
		}
	}
	return exprFastPath{kind: exprGeneric}
}

// classifyInfixCmp classifies == and != expressions for fast-path evaluation.
// Handles: ident == "str", "str" == ident, ident == ident (and != variants).
func classifyInfixCmp(v *interpreter.InfixExpression) exprFastPath {
	isEq := v.Operator == "=="

	// ident == "literal" or ident != "literal"
	if left, ok := v.Left.(*interpreter.Identifier); ok {
		if right, ok := v.Right.(*interpreter.StringLiteral); ok {
			kind := exprCmpEqStr
			if !isEq {
				kind = exprCmpNeqStr
			}
			return exprFastPath{kind: kind, ident: left.Name, strVal: right.Value}
		}
		// ident == ident
		if right, ok := v.Right.(*interpreter.Identifier); ok {
			kind := exprCmpEqIdent
			if !isEq {
				kind = exprCmpNeqIdent
			}
			return exprFastPath{kind: kind, ident: left.Name, field: right.Name}
		}
	}

	// "literal" == ident (reversed)
	if left, ok := v.Left.(*interpreter.StringLiteral); ok {
		if right, ok := v.Right.(*interpreter.Identifier); ok {
			kind := exprCmpEqStr
			if !isEq {
				kind = exprCmpNeqStr
			}
			return exprFastPath{kind: kind, ident: right.Name, strVal: left.Value}
		}
	}

	return exprFastPath{kind: exprGeneric}
}

// isConstExpr returns true if the expression contains only literal values (no variable references).
func isConstExpr(expr interpreter.Expression) bool {
	switch v := expr.(type) {
	case *interpreter.StringLiteral, *interpreter.IntegerLiteral,
		*interpreter.FloatLiteral, *interpreter.BooleanLiteral,
		*interpreter.NullLiteral:
		return true
	case *interpreter.HashLiteral:
		return isConstHashLiteral(v)
	case *interpreter.ArrayLiteral:
		for _, el := range v.Elements {
			if !isConstExpr(el) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// isConstHashLiteral returns true if a hash literal contains only literal keys and values.
func isConstHashLiteral(h *interpreter.HashLiteral) bool {
	for _, entry := range h.Entries {
		if entry.IsSpread {
			return false
		}
		if !isConstExpr(entry.Key) || !isConstExpr(entry.Value) {
			return false
		}
	}
	return true
}

// evalExpr evaluates an SPL expression string against the given environment.
func (e *Engine) evalExpr(expr string, env *interpreter.Environment) (interpreter.Object, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, fmt.Errorf("empty expression")
	}
	return e.evalExprTrimmed(expr, env)
}

// evalExprTrimmed evaluates a pre-trimmed expression. Used internally to avoid redundant TrimSpace.
func (e *Engine) evalExprTrimmed(expr string, env *interpreter.Environment) (interpreter.Object, error) {
	// Combined cache lookup — single lock acquisition for both caches
	e.mu.RLock()
	metaEntry, metaOK := e.exprMeta[expr]
	progEntry, progOK := e.exprCache[expr]
	e.mu.RUnlock()

	var meta exprFastPath
	var program *interpreter.Program

	if metaOK {
		if metaEntry.isExpired(e.CachePolicy.ExprTTL) {
			e.mu.Lock()
			delete(e.exprMeta, expr)
			e.mu.Unlock()
			metaOK = false
		} else {
			meta = metaEntry.meta
		}
	}
	if progOK {
		if progEntry.isExpired(e.CachePolicy.ExprTTL) {
			e.mu.Lock()
			delete(e.exprCache, expr)
			e.mu.Unlock()
			progOK = false
		} else {
			program = progEntry.program
		}
	}

	if metaOK {
		switch meta.kind {
		case exprIdent:
			if obj, ok := env.Get(meta.ident); ok {
				return obj, nil
			}
			return nil, fmt.Errorf("expression eval error: identifier not found: %s", meta.ident)
		case exprDot:
			leftObj, ok := env.Get(meta.ident)
			if !ok {
				return nil, fmt.Errorf("expression eval error: identifier not found: %s", meta.ident)
			}
			return fastDotAccess(leftObj, meta.field)
		case exprStringLit:
			return &interpreter.String{Value: meta.strVal}, nil
		case exprIntLit:
			return &interpreter.Integer{Value: meta.intVal}, nil
		case exprBoolTrue:
			return interpreter.TRUE, nil
		case exprBoolFalse:
			return interpreter.FALSE, nil
		case exprConstHash:
			if meta.constResult != nil {
				if h, ok := meta.constResult.(*interpreter.Hash); ok {
					return cloneHash(h), nil
				}
				return meta.constResult, nil
			}
		case exprCmpEqStr, exprCmpNeqStr:
			return evalCmpStr(env, meta)
		case exprCmpEqIdent, exprCmpNeqIdent:
			return evalCmpIdent(env, meta)
		}
		// exprGeneric: fall through to full eval
	}

	if !progOK {
		l := interpreter.NewLexer(expr)
		p := interpreter.NewParser(l)
		program = p.ParseProgram()
		if errs := p.Errors(); len(errs) > 0 {
			return nil, fmt.Errorf("expression parse error: %s", strings.Join(errs, "; "))
		}
		now := time.Now().UnixNano()
		e.mu.Lock()
		evictMap(e.exprCache, maxExprCacheSize)
		e.exprCache[expr] = exprCacheValue{
			cacheEntry: cacheEntry{created: now},
			program:    program,
		}

		// Analyze for fast path
		meta = analyzeExpr(program)
		evictMap(e.exprMeta, maxExprMetaCacheSize)
		e.exprMeta[expr] = exprMetaCacheValue{
			cacheEntry: cacheEntry{created: now},
			meta:       meta,
		}
		e.mu.Unlock()

		// Try fast path on first encounter too
		switch meta.kind {
		case exprIdent:
			if obj, ok := env.Get(meta.ident); ok {
				return obj, nil
			}
			return nil, fmt.Errorf("expression eval error: identifier not found: %s", meta.ident)
		case exprDot:
			leftObj, ok := env.Get(meta.ident)
			if !ok {
				return nil, fmt.Errorf("expression eval error: identifier not found: %s", meta.ident)
			}
			return fastDotAccess(leftObj, meta.field)
		case exprStringLit:
			return &interpreter.String{Value: meta.strVal}, nil
		case exprIntLit:
			return &interpreter.Integer{Value: meta.intVal}, nil
		case exprBoolTrue:
			return interpreter.TRUE, nil
		case exprBoolFalse:
			return interpreter.FALSE, nil
		case exprConstHash:
			// Evaluate once and cache the result for future calls
			result := interpreter.Eval(program, env)
			if result != nil && result.Type() == interpreter.ERROR_OBJ {
				return nil, fmt.Errorf("expression eval error: %s", result.Inspect())
			}
			meta.constResult = result
			e.mu.Lock()
			e.exprMeta[expr] = exprMetaCacheValue{
				cacheEntry: cacheEntry{created: now},
				meta:       meta,
			}
			e.mu.Unlock()
			// Return a clone since the caller may mutate (e.g., adding default props)
			if h, ok := result.(*interpreter.Hash); ok {
				return cloneHash(h), nil
			}
			return result, nil
		case exprCmpEqStr, exprCmpNeqStr:
			return evalCmpStr(env, meta)
		case exprCmpEqIdent, exprCmpNeqIdent:
			return evalCmpIdent(env, meta)
		}
	}

	// Materialize any lazyHash values in the environment before passing to interpreter.Eval,
	// which doesn't know about our lazy wrapper type.
	materializeLazyHashes(env)
	result := interpreter.Eval(program, env)
	if result != nil && result.Type() == interpreter.ERROR_OBJ {
		return nil, fmt.Errorf("expression eval error: %s", result.Inspect())
	}
	return result, nil
}

// materializeLazyHashes converts any lazyHash values in the environment's store
// to real interpreter.Hash objects. Called before passing to interpreter.Eval which
// doesn't know about the lazy wrapper type.
func materializeLazyHashes(env *interpreter.Environment) {
	for k, v := range env.Store {
		if lh, ok := v.(*lazyHash); ok {
			env.Store[k] = lh.materialize()
		}
	}
}

// evalCmpStr evaluates identifier == "literal" or identifier != "literal" fast-path.
func evalCmpStr(env *interpreter.Environment, meta exprFastPath) (interpreter.Object, error) {
	obj, ok := env.Get(meta.ident)
	if !ok {
		return nil, fmt.Errorf("expression eval error: identifier not found: %s", meta.ident)
	}
	var match bool
	switch v := obj.(type) {
	case *interpreter.String:
		match = v.Value == meta.strVal
	default:
		match = obj.Inspect() == meta.strVal
	}
	if meta.kind == exprCmpNeqStr {
		match = !match
	}
	if match {
		return interpreter.TRUE, nil
	}
	return interpreter.FALSE, nil
}

// evalCmpIdent evaluates identifier == identifier or identifier != identifier fast-path.
func evalCmpIdent(env *interpreter.Environment, meta exprFastPath) (interpreter.Object, error) {
	leftObj, ok := env.Get(meta.ident)
	if !ok {
		return nil, fmt.Errorf("expression eval error: identifier not found: %s", meta.ident)
	}
	rightObj, ok := env.Get(meta.field)
	if !ok {
		return nil, fmt.Errorf("expression eval error: identifier not found: %s", meta.field)
	}
	var match bool
	switch l := leftObj.(type) {
	case *interpreter.String:
		if r, ok := rightObj.(*interpreter.String); ok {
			match = l.Value == r.Value
		}
	case *interpreter.Integer:
		if r, ok := rightObj.(*interpreter.Integer); ok {
			match = l.Value == r.Value
		}
	case *interpreter.Float:
		if r, ok := rightObj.(*interpreter.Float); ok {
			match = l.Value == r.Value
		}
	case *interpreter.Boolean:
		if r, ok := rightObj.(*interpreter.Boolean); ok {
			match = l.Value == r.Value
		}
	default:
		match = leftObj.Inspect() == rightObj.Inspect()
	}
	if meta.kind == exprCmpNeqIdent {
		match = !match
	}
	if match {
		return interpreter.TRUE, nil
	}
	return interpreter.FALSE, nil
}

// hashKeyCache caches computed HashKey values for field names to avoid repeated allocations.
var hashKeyCache sync.Map // string → interpreter.HashKey

func cachedHashKey(field string) interpreter.HashKey {
	if v, ok := hashKeyCache.Load(field); ok {
		return v.(interpreter.HashKey)
	}
	key := &interpreter.String{Value: field}
	hk := key.HashKey()
	hashKeyCache.Store(field, hk)
	return hk
}

// fastDotAccess performs a direct field lookup on an object without going through interpreter.Eval.
func fastDotAccess(obj interpreter.Object, field string) (interpreter.Object, error) {
	switch v := obj.(type) {
	case *interpreter.Hash:
		hk := cachedHashKey(field)
		if pair, exists := v.Pairs[hk]; exists {
			return pair.Value, nil
		}
		return interpreter.NULL, nil
	case *lazyHash:
		if val, exists := v.data[field]; exists {
			return toLazyObject(val), nil
		}
		return interpreter.NULL, nil
	}
	// For non-hash types, data in template context is always Hash or lazyHash
	return interpreter.NULL, nil
}
