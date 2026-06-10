package spl

import (
	"fmt"
	"strings"
)

// --- Node types ---

type Node interface {
	nodeType() string
}

type TextNode struct {
	Text string
}

func (n *TextNode) nodeType() string { return "text" }

type ExprNode struct {
	Expr    string
	Raw     bool
	Filters []FilterCall
}

func (n *ExprNode) nodeType() string { return "expr" }

type FilterCall struct {
	Name string
	Args []string
}

type IfBranch struct {
	Cond string // SPL expression; empty for @else
	Body []Node
}

type IfNode struct {
	Branches []IfBranch
	Else     []Node
}

func (n *IfNode) nodeType() string { return "if" }

// ConditionalRenderNode supports JSX-like conditional fragments:
// {condition && <div>...</div>} and {condition || <div>...</div>}.
type ConditionalRenderNode struct {
	Cond string
	Op   string // "&&" renders when truthy; "||" renders when falsy
	Body []Node
}

func (n *ConditionalRenderNode) nodeType() string { return "conditional-render" }

type ForNode struct {
	KeyVar  string // "" if only value
	ValVar  string
	Iter    string // SPL expression for the iterable
	KeyExpr string // optional stable key expression for hydrated list metadata
	Body    []Node
	Empty   []Node // rendered when iterable is empty
}

func (n *ForNode) nodeType() string { return "for" }

type SwitchCase struct {
	Values []string // SPL expressions; empty slice means @default
	Body   []Node
}

type SwitchNode struct {
	Expr    string
	Cases   []SwitchCase
	Default []Node
}

func (n *SwitchNode) nodeType() string { return "switch" }

// MatchCase holds a single @case branch inside @match.
type MatchCase struct {
	PatternExpr string // raw pattern text (e.g. "x: integer", "[a, b]", "> 10")
	Guard       string // optional guard expression after "if"
	Body        []Node
}

// MatchNode represents a @match(expr) { @case(pattern) { ... } @default { ... } } directive.
type MatchNode struct {
	Expr    string // the match subject expression
	Cases   []MatchCase
	Default []Node // @default body (nil if none)
}

func (n *MatchNode) nodeType() string { return "match" }

type RawNode struct {
	Text string
}

func (n *RawNode) nodeType() string { return "raw" }

type IncludeNode struct {
	Path     string // file path (string literal)
	DataExpr string // optional SPL expression for include-local data
}

func (n *IncludeNode) nodeType() string { return "include" }

type ImportNode struct {
	Path string
}

func (n *ImportNode) nodeType() string { return "import" }

type HandlerNode struct {
	Name string
	Expr string
	Body string
}

func (n *HandlerNode) nodeType() string { return "handler" }

type ExtendsNode struct {
	Path string
}

func (n *ExtendsNode) nodeType() string { return "extends" }

type BlockNode struct {
	Name string
	Body []Node
}

func (n *BlockNode) nodeType() string { return "block" }

type DefineNode struct {
	Name string
	Body []Node
}

func (n *DefineNode) nodeType() string { return "define" }

// PrependNode adds content before a parent block's default content.
type PrependNode struct {
	Name string
	Body []Node
}

func (n *PrependNode) nodeType() string { return "prepend" }

// AppendNode adds content after a parent block's default content.
type AppendNode struct {
	Name string
	Body []Node
}

func (n *AppendNode) nodeType() string { return "append" }

// HasBlockNode checks if a child has defined content for a named block.
type HasBlockNode struct {
	Name string
	Body []Node
	Else []Node // optional @else content
}

func (n *HasBlockNode) nodeType() string { return "hasBlock" }

// PropDef describes a single declared prop with optional alias, default, and ?-prefix optionality.
type PropDef struct {
	Name     string // external prop name (what the caller passes)
	Alias    string // internal variable name ("" = same as Name)
	Type     string // optional simple type: string, number, bool, array, object
	Default  string // SPL expression for default value ("" = none)
	Optional bool   // true when declared as ?name (NULL if not passed)
}

// ComponentNode defines a reusable component.
type ComponentNode struct {
	Name  string    // component name
	Props []PropDef // declared prop definitions (optional)
	Body  []Node    // component body (template)
}

func (n *ComponentNode) nodeType() string { return "component" }

// RenderNode invokes a component by name.
type RenderNode struct {
	Name      string // component name
	PropsExpr string // optional SPL expression for props (hash literal)
	Children  []Node // body nodes (children/slot fills)
}

func (n *RenderNode) nodeType() string { return "render" }

// SlotNode is a placeholder inside a component body for injected content.
type SlotNode struct {
	Name string // "" = default slot
}

func (n *SlotNode) nodeType() string { return "slot" }

// FillNode provides content for a named slot inside a @render body.
type FillNode struct {
	Name string
	Body []Node
}

func (n *FillNode) nodeType() string { return "fill" }

// LetNode assigns a computed value to a variable at render time.
type LetNode struct {
	VarName string
	Expr    string
}

func (n *LetNode) nodeType() string { return "let" }

// ComputedNode defines a derived value (same semantics as @let, separate for intent).
type ComputedNode struct {
	VarName string
	Expr    string
}

func (n *ComputedNode) nodeType() string { return "computed" }

// WatchNode renders its body only when the watched expression value changes.
type WatchNode struct {
	Expr string
	Body []Node
}

func (n *WatchNode) nodeType() string { return "watch" }

// SchemaFormNode renders a form from a registered JSON schema.
type SchemaFormNode struct {
	SchemaName string // name of the registered schema
	DataExpr   string // SPL expression resolving to data hash
}

func (n *SchemaFormNode) nodeType() string { return "schema_form" }

// SchemaDetailNode renders a detail view from a registered JSON schema.
type SchemaDetailNode struct {
	SchemaName string
	DataExpr   string
}

func (n *SchemaDetailNode) nodeType() string { return "schema_detail" }

// SchemaTableNode renders a table from a registered JSON schema.
type SchemaTableNode struct {
	SchemaName string
	ItemsExpr  string
}

func (n *SchemaTableNode) nodeType() string { return "schema_table" }

// CacheNode represents a @cache directive for fragment caching.
type CacheNode struct {
	Key  string   // cache key expression (evaluated at render time)
	TTL  string   // optional TTL expression in seconds ("0" or "" = use engine CachePolicy.FragmentTTL)
	Deps []string // optional dependency expressions (used as cache key suffix)
	Body []Node
}

func (n *CacheNode) nodeType() string { return "cache" }

// --- Parser ---

type parser struct {
	src       []rune
	pos       int
	delimLeft string // expression open delimiter (default "${")
	delimRight string // expression close delimiter (default "}")
}

// defaultDelimiters returns the default expression delimiters.
func defaultDelimiters() (string, string) {
	return "${", "}"
}

// posInfo returns a "line:col" string for the current parser position for error messages.
func (p *parser) posInfo() string {
	line := 1
	col := 1
	end := p.pos
	if end > len(p.src) {
		end = len(p.src)
	}
	for i := 0; i < end; i++ {
		if p.src[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return fmt.Sprintf("%d:%d", line, col)
}

// errorf returns a formatted error prefixed with the current line:col position.
func (p *parser) errorf(format string, args ...any) error {
	return fmt.Errorf("at %s: %w", p.posInfo(), fmt.Errorf(format, args...))
}

func parse(src string) ([]Node, error) {
	left, right := defaultDelimiters()
	return parseWithDelims(src, left, right)
}

func parseWithDelims(src string, delimLeft, delimRight string) ([]Node, error) {
	p := &parser{src: []rune(src), delimLeft: delimLeft, delimRight: delimRight}
	return p.parseNodes(false)
}

// parseEngineDelims parses a template using the engine's configured delimiters.
// Falls back to "${" / "}" if the engine's delimiters are not set.
func (e *Engine) parseWithEngineDelims(src string) ([]Node, error) {
	left, right := e.DelimLeft, e.DelimRight
	if left == "" {
		left = "${"
	}
	if right == "" {
		right = "}"
	}
	return parseWithDelims(src, left, right)
}

func (p *parser) remaining() int { return len(p.src) - p.pos }
func (p *parser) eof() bool      { return p.pos >= len(p.src) }
func (p *parser) peek() rune {
	if p.eof() {
		return 0
	}
	return p.src[p.pos]
}
func (p *parser) advance() rune {
	ch := p.src[p.pos]
	p.pos++
	return ch
}
func (p *parser) peekAt(offset int) rune {
	i := p.pos + offset
	if i >= len(p.src) || i < 0 {
		return 0
	}
	return p.src[i]
}
func (p *parser) startsWith(s string) bool {
	if p.remaining() < len(s) {
		return false
	}
	// Fast path for ASCII-only strings (all directives are ASCII)
	for i := 0; i < len(s); i++ {
		if int(p.src[p.pos+i]) != int(s[i]) {
			return false
		}
	}
	return true
}
func (p *parser) advanceN(n int) {
	p.pos += n
	if p.pos > len(p.src) {
		p.pos = len(p.src)
	}
}
// startsWithDelimLeft checks if the current position matches the configured left delimiter.
func (p *parser) startsWithDelimLeft() bool {
	return p.startsWith(p.delimLeft)
}

// startsWithDelimRight checks if the current position matches the configured right delimiter.
func (p *parser) startsWithDelimRight() bool {
	return p.startsWith(p.delimRight)
}

func (p *parser) skipWhitespace() {
	for !p.eof() && (p.peek() == ' ' || p.peek() == '\t') {
		p.advance()
	}
}

// parseNodes parses nodes until EOF or a closing '}' (if inBlock is true).
func (p *parser) parseNodes(inBlock bool) ([]Node, error) {
	var nodes []Node
	var textBuf strings.Builder
	var textQuote rune
	var prevTextRune rune
	// textBraceDepth tracks non-directive '{' encountered in plain text (e.g. CSS rules).
	// A closing '}' only terminates the directive block when this depth is zero.
	textBraceDepth := 0

	flushText := func() {
		if textBuf.Len() > 0 {
			nodes = append(nodes, &TextNode{Text: textBuf.String()})
			textBuf.Reset()
		}
	}

	for !p.eof() {
		if textQuote != 0 {
			if p.startsWithDelimLeft() {
				flushText()
				node, err := p.parseExpr()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, node)
				prevTextRune = 0
				continue
			}
			ch := p.advance()
			textBuf.WriteRune(ch)
			if ch == textQuote && prevTextRune != '\\' {
				textQuote = 0
			}
			prevTextRune = ch
			continue
		}

		// Check for closing brace when inside a block
		if inBlock && p.peek() == '}' {
			if textBraceDepth > 0 {
				// This '}' closes a non-directive brace (e.g. CSS rule) — treat as text
				textBraceDepth--
				ch := p.advance()
				textBuf.WriteRune(ch)
				prevTextRune = ch
				continue
			}
			p.advance() // consume '}'
			flushText()
			return nodes, nil
		}

		// Expression: ${...} or custom delimiters
		if p.startsWithDelimLeft() {
			flushText()
			node, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, node)
			continue
		}

		// JSX-like conditional fragment: {condition && <fragment>} or {condition || <fragment>}
		if p.peek() == '{' {
			if node, ok, err := p.tryParseConditionalRender(); ok || err != nil {
				if err != nil {
					return nil, err
				}
				flushText()
				nodes = append(nodes, node)
				prevTextRune = 0
				continue
			}
		}

		// Directive: @keyword
		if p.peek() == '@' {
			// Check what follows
			if p.peekAt(1) == '/' && p.peekAt(2) == '/' {
				// Comment: @// ...
				flushText()
				p.parseComment()
				continue
			}
			keyword := p.peekKeyword()
			switch keyword {
			case "if":
				flushText()
				node, err := p.parseIf()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, node)
				continue
			case "for":
				flushText()
				node, err := p.parseFor()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, node)
				continue
			case "switch":
				flushText()
				node, err := p.parseSwitchDirective()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, node)
				continue
			case "match":
				flushText()
				node, err := p.parseMatchDirective()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, node)
				continue
			case "raw":
				flushText()
				node, err := p.parseRaw()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, node)
				continue
			case "include":
				flushText()
				node, err := p.parseInclude()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, node)
				continue
			case "import":
				flushText()
				node, err := p.parseImportDirective()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, node)
				continue
			case "handler":
				flushText()
				node, err := p.parseHandlerDirective()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, node)
				continue
			case "extends":
				flushText()
				node, err := p.parseExtends()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, node)
				continue
			case "block":
				flushText()
				node, err := p.parseBlock()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, node)
				continue
			case "define":
				flushText()
				node, err := p.parseDefine()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, node)
				continue
			case "prepend":
				flushText()
				node, err := p.parsePrepend()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, node)
				continue
			case "append":
				flushText()
				node, err := p.parseAppend()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, node)
				continue
			case "hasBlock":
				flushText()
				node, err := p.parseHasBlock()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, node)
				continue
			case "component":
				flushText()
				node, err := p.parseComponent()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, node)
				continue
			case "render":
				flushText()
				node, err := p.parseRender()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, node)
				continue
			case "slot":
				flushText()
				node, err := p.parseSlot()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, node)
				continue
			case "fill":
				flushText()
				node, err := p.parseFill()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, node)
				continue
			case "let":
				flushText()
				node, err := p.parseLet()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, node)
				continue
			case "computed":
				flushText()
				node, err := p.parseComputed()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, node)
				continue
			case "computedClient":
				flushText()
				node, err := p.parseComputedClient()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, node)
				continue
			case "watch":
				flushText()
				node, err := p.parseWatch()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, node)
				continue
			case "signal":
				flushText()
				node, err := p.parseSignal()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, node)
				continue
			case "local":
				flushText()
				node, err := p.parseLocal()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, node)
				continue
			case "assets":
				flushText()
				node, err := p.parseAssets()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, node)
				continue
			case "bind":
				flushText()
				node, err := p.parseBind()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, node)
				continue
			case "effect":
				flushText()
				node, err := p.parseEffect()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, node)
				continue
			case "reactive":
				flushText()
				node, err := p.parseReactiveView()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, node)
				continue
			case "click":
				flushText()
				node, err := p.parseClick()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, node)
				continue
			case "stream":
				flushText()
				node, err := p.parseStream()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, node)
				continue
			case "defer":
				flushText()
				node, err := p.parseDefer()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, node)
				continue
			case "lazy":
				flushText()
				node, err := p.parseLazy()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, node)
				continue
			case "translate":
				flushText()
				node, err := p.parseTranslate()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, node)
				continue
			case "cache":
				flushText()
				node, err := p.parseCache()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, node)
				continue
			case "schema_form":
				flushText()
				node, err := p.parseSchemaForm()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, node)
				continue
			case "schema_detail":
				flushText()
				node, err := p.parseSchemaDetail()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, node)
				continue
			case "schema_table":
				flushText()
				node, err := p.parseSchemaTable()
				if err != nil {
					return nil, err
				}
				nodes = append(nodes, node)
				continue
			case "elseif", "else", "empty", "case", "default", "fallback":
				// These are terminators for parent blocks — stop parsing here
				flushText()
				return nodes, nil
			}
		}

		if p.peek() == '"' || p.peek() == '\'' || p.peek() == '`' {
			ch := p.advance()
			textQuote = ch
			textBuf.WriteRune(ch)
			prevTextRune = ch
			continue
		}

		// Regular text
		ch := p.advance()
		// Track non-directive opening braces (e.g. CSS rules) so their matching '}'
		// is not mistaken for the directive block terminator.
		if inBlock && ch == '{' {
			textBraceDepth++
		}
		textBuf.WriteRune(ch)
		prevTextRune = ch
	}

	if inBlock {
		return nil, p.errorf("unexpected end of template: unclosed block")
	}

	flushText()
	return nodes, nil
}

// peekKeyword returns the keyword after '@' without advancing.
func (p *parser) peekKeyword() string {
	i := p.pos + 1 // skip '@'
	start := i
	for i < len(p.src) && isAlpha(p.src[i]) {
		i++
	}
	if start == i {
		return ""
	}
	return string(p.src[start:i])
}

func isAlpha(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_'
}

func (p *parser) tryParseConditionalRender() (*ConditionalRenderNode, bool, error) {
	start := p.pos
	content, ok, err := p.readBalancedSingleBraceContent()
	if err != nil {
		p.pos = start
		return nil, false, err
	}
	if !ok {
		p.pos = start
		return nil, false, nil
	}

	cond, op, fragment, ok := splitConditionalRenderContent(content)
	if !ok {
		p.pos = start
		return nil, false, nil
	}

	body, err := parse(fragment)
	if err != nil {
		return nil, false, p.errorf("conditional fragment: %w", err)
	}
	return &ConditionalRenderNode{Cond: cond, Op: op, Body: body}, true, nil
}

func (p *parser) readBalancedSingleBraceContent() (string, bool, error) {
	if p.peek() != '{' || p.peekAt(1) == '{' {
		return "", false, nil
	}
	p.advance()
	var buf strings.Builder
	depth := 1
	for !p.eof() && depth > 0 {
		ch := p.peek()
		if ch == '"' || ch == '\'' || ch == '`' {
			buf.WriteString(p.readStringLiteral())
			continue
		}
		if ch == '/' && p.peekAt(1) == '/' {
			buf.WriteRune(p.advance())
			buf.WriteRune(p.advance())
			for !p.eof() && p.peek() != '\n' {
				buf.WriteRune(p.advance())
			}
			continue
		}
		if ch == '/' && p.peekAt(1) == '*' {
			buf.WriteRune(p.advance())
			buf.WriteRune(p.advance())
			for !p.eof() {
				c := p.advance()
				buf.WriteRune(c)
				if c == '*' && p.peek() == '/' {
					buf.WriteRune(p.advance())
					break
				}
			}
			continue
		}
		if ch == '{' {
			depth++
			buf.WriteRune(p.advance())
			continue
		}
		if ch == '}' {
			depth--
			if depth == 0 {
				p.advance()
				return buf.String(), true, nil
			}
			buf.WriteRune(p.advance())
			continue
		}
		buf.WriteRune(p.advance())
	}
	return "", false, p.errorf("unclosed conditional fragment")
}

func splitConditionalRenderContent(content string) (cond, op, fragment string, ok bool) {
	runes := []rune(content)
	depth := 0
	var quote rune
	escaped := false
	bestIndex := -1
	bestOp := ""

	for i := 0; i < len(runes)-1; i++ {
		ch := runes[i]
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == quote {
				quote = 0
			}
			continue
		}
		switch ch {
		case '"', '\'', '`':
			quote = ch
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		case '&', '|':
			if depth == 0 && runes[i+1] == ch {
				next := i + 2
				for next < len(runes) && (runes[next] == ' ' || runes[next] == '\t' || runes[next] == '\n' || runes[next] == '\r') {
					next++
				}
				if next < len(runes) && (runes[next] == '<' || runes[next] == '(') {
					bestIndex = i
					bestOp = string([]rune{ch, ch})
				}
				i++
			}
		}
	}

	if bestIndex < 0 {
		return "", "", "", false
	}
	cond = strings.TrimSpace(string(runes[:bestIndex]))
	fragment = strings.TrimSpace(string(runes[bestIndex+2:]))
	fragment = strings.TrimSpace(unwrapConditionalFragmentParens(fragment))
	if cond == "" || fragment == "" || !strings.HasPrefix(fragment, "<") {
		return "", "", "", false
	}
	return cond, bestOp, fragment, true
}

func unwrapConditionalFragmentParens(fragment string) string {
	if !strings.HasPrefix(fragment, "(") || !strings.HasSuffix(fragment, ")") {
		return fragment
	}
	runes := []rune(fragment)
	depth := 0
	var quote rune
	escaped := false
	for i, ch := range runes {
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == quote {
				quote = 0
			}
			continue
		}
		switch ch {
		case '"', '\'', '`':
			quote = ch
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 && i != len(runes)-1 {
				return fragment
			}
		}
	}
	if depth != 0 {
		return fragment
	}
	return strings.TrimSpace(string(runes[1 : len(runes)-1]))
}

// parseExpr parses expression with configurable delimiters.
// Default: ${expr}, ${raw expr}, ${expr | filter}
func (p *parser) parseExpr() (*ExprNode, error) {
	p.advanceN(len(p.delimLeft)) // skip left delimiter
	var buf strings.Builder
	delimRight := p.delimRight

	// For single-character right delimiters (like "}"), use depth-tracking.
	// For multi-character delimiters, do exact match.
	if len(delimRight) == 1 {
		right := rune(delimRight[0])
		depth := 1
		for !p.eof() && depth > 0 {
			ch := p.peek()
			if ch == '{' {
				depth++
				buf.WriteRune(p.advance())
			} else if ch == right {
				depth--
				if depth == 0 {
					p.advance() // consume closing delimiter
					break
				}
				buf.WriteRune(p.advance())
			} else if ch == '"' || ch == '\'' || ch == '`' {
				buf.WriteString(p.readStringLiteral())
			} else {
				buf.WriteRune(p.advance())
			}
		}
		if depth != 0 {
			return nil, p.errorf("unclosed expression %s...%s", p.delimLeft, p.delimRight)
		}
	} else {
		// Multi-character right delimiter: look for exact match
		for !p.eof() {
			if p.startsWith(delimRight) {
				p.advanceN(len(delimRight))
				break
			}
			if p.peek() == '"' || p.peek() == '\'' || p.peek() == '`' {
				buf.WriteString(p.readStringLiteral())
			} else {
				buf.WriteRune(p.advance())
			}
		}
	}

	content := buf.String()
	node := &ExprNode{}

	// Check for raw prefix
	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(trimmed, "raw ") {
		node.Raw = true
		content = strings.TrimPrefix(trimmed, "raw ")
	}

	// Split on '|' for filters (but not inside strings/parens)
	parts := splitPipes(content)
	node.Expr = strings.TrimSpace(parts[0])
	for _, fp := range parts[1:] {
		fc := parseFilterCall(strings.TrimSpace(fp))
		node.Filters = append(node.Filters, fc)
	}

	return node, nil
}

// splitPipes splits an expression by top-level '|' characters (not inside strings, parens, or braces).
func splitPipes(s string) []string {
	var parts []string
	var buf strings.Builder
	depth := 0 // tracks (), {}, []
	inStr := rune(0)

	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		ch := runes[i]
		if inStr != 0 {
			buf.WriteRune(ch)
			if ch == '\\' && i+1 < len(runes) {
				i++
				buf.WriteRune(runes[i])
			} else if ch == inStr {
				inStr = 0
			}
			continue
		}
		switch ch {
		case '"', '\'', '`':
			inStr = ch
			buf.WriteRune(ch)
		case '(', '{', '[':
			depth++
			buf.WriteRune(ch)
		case ')', '}', ']':
			depth--
			buf.WriteRune(ch)
		case '|':
			if depth == 0 {
				// Check for || (logical OR) — not a pipe
				if i+1 < len(runes) && runes[i+1] == '|' {
					buf.WriteRune(ch)
					i++
					buf.WriteRune(runes[i])
				} else {
					parts = append(parts, buf.String())
					buf.Reset()
				}
			} else {
				buf.WriteRune(ch)
			}
		default:
			buf.WriteRune(ch)
		}
	}
	parts = append(parts, buf.String())
	return parts
}

// parseFilterCall parses "filterName" or "filterName arg1 arg2"
func parseFilterCall(s string) FilterCall {
	// Format: name or name "arg" or name arg
	parts := splitFilterArgs(s)
	fc := FilterCall{Name: parts[0]}
	if len(parts) > 1 {
		fc.Args = parts[1:]
	}
	return fc
}

// splitFilterArgs splits filter name and arguments, respecting quoted strings.
func splitFilterArgs(s string) []string {
	var parts []string
	runes := []rune(s)
	i := 0
	for i < len(runes) {
		// Skip whitespace
		for i < len(runes) && (runes[i] == ' ' || runes[i] == '\t') {
			i++
		}
		if i >= len(runes) {
			break
		}
		if runes[i] == '"' || runes[i] == '\'' {
			quote := runes[i]
			i++ // skip opening quote
			var buf strings.Builder
			for i < len(runes) && runes[i] != quote {
				if runes[i] == '\\' && i+1 < len(runes) {
					i++
					buf.WriteRune(runes[i])
				} else {
					buf.WriteRune(runes[i])
				}
				i++
			}
			if i < len(runes) {
				i++ // skip closing quote
			}
			parts = append(parts, buf.String())
		} else {
			var buf strings.Builder
			for i < len(runes) && runes[i] != ' ' && runes[i] != '\t' {
				buf.WriteRune(runes[i])
				i++
			}
			parts = append(parts, buf.String())
		}
	}
	return parts
}

func (p *parser) readStringLiteral() string {
	var buf strings.Builder
	quote := p.advance()
	buf.WriteRune(quote)
	for !p.eof() {
		ch := p.advance()
		buf.WriteRune(ch)
		if ch == '\\' && !p.eof() {
			buf.WriteRune(p.advance())
		} else if ch == quote {
			break
		}
	}
	return buf.String()
}

// parseComment consumes @// ... until end of line.
func (p *parser) parseComment() {
	p.advanceN(3) // skip '@//'
	for !p.eof() && p.peek() != '\n' {
		p.advance()
	}
	if !p.eof() {
		p.advance() // consume '\n'
	}
}

// parseIf parses @if(cond) { ... } @elseif(cond) { ... } @else { ... }
func (p *parser) parseIf() (*IfNode, error) {
	node := &IfNode{}

	// Parse @if(cond) { body }
	p.advanceN(3) // skip '@if'
	cond, err := p.readParenExpr()
	if err != nil {
		return nil, p.errorf("@if: %w", err)
	}
	p.skipWhitespaceAndNewlines()
	if p.peek() != '{' {
		return nil, p.errorf("@if: expected '{' after condition")
	}
	p.advance() // skip '{'
	body, err := p.parseNodes(true)
	if err != nil {
		return nil, p.errorf("@if body: %w", err)
	}
	node.Branches = append(node.Branches, IfBranch{Cond: cond, Body: body})

	// Parse optional @elseif / @else
	for {
		p.skipWhitespaceAndNewlines()
		if p.startsWith("@elseif") {
			p.advanceN(7) // skip '@elseif'
			cond, err := p.readParenExpr()
			if err != nil {
				return nil, p.errorf("@elseif: %w", err)
			}
			p.skipWhitespaceAndNewlines()
			if p.peek() != '{' {
				return nil, p.errorf("@elseif: expected '{'")
			}
			p.advance()
			body, err := p.parseNodes(true)
			if err != nil {
				return nil, p.errorf("@elseif body: %w", err)
			}
			node.Branches = append(node.Branches, IfBranch{Cond: cond, Body: body})
		} else if p.startsWith("@else") && !p.startsWith("@elseif") {
			p.advanceN(5) // skip '@else'
			p.skipWhitespaceAndNewlines()
			if p.peek() != '{' {
				return nil, p.errorf("@else: expected '{'")
			}
			p.advance()
			body, err := p.parseNodes(true)
			if err != nil {
				return nil, p.errorf("@else body: %w", err)
			}
			node.Else = body
			break
		} else {
			break
		}
	}

	return node, nil
}

// parseFor parses @for(item in items) { ... } @empty { ... }
func (p *parser) parseFor() (*ForNode, error) {
	p.advanceN(4) // skip '@for'
	inner, err := p.readParenExpr()
	if err != nil {
		return nil, p.errorf("@for: %w", err)
	}

	node := &ForNode{}
	// Parse "item in items", "i, item in items", or "item in items; key item.id".
	inner = strings.TrimSpace(inner)
	if parts := splitTopLevelSemicolons(inner); len(parts) > 1 {
		inner = strings.TrimSpace(parts[0])
		for _, opt := range parts[1:] {
			opt = strings.TrimSpace(opt)
			if strings.HasPrefix(opt, "key ") {
				node.KeyExpr = strings.TrimSpace(opt[len("key "):])
			} else if strings.HasPrefix(opt, "key=") {
				node.KeyExpr = strings.TrimSpace(opt[len("key="):])
			} else {
				return nil, p.errorf("@for: unknown option %q", opt)
			}
		}
	}
	inIdx := strings.Index(inner, " in ")
	if inIdx < 0 {
		return nil, p.errorf("@for: expected 'VAR in EXPR' syntax, got: %s", inner)
	}
	vars := strings.TrimSpace(inner[:inIdx])
	node.Iter = strings.TrimSpace(inner[inIdx+4:])

	if strings.Contains(vars, ",") {
		parts := strings.SplitN(vars, ",", 2)
		node.KeyVar = strings.TrimSpace(parts[0])
		node.ValVar = strings.TrimSpace(parts[1])
	} else {
		node.ValVar = vars
	}

	p.skipWhitespaceAndNewlines()
	if p.peek() != '{' {
		return nil, p.errorf("@for: expected '{'")
	}
	p.advance()
	body, err := p.parseNodes(true)
	if err != nil {
		return nil, p.errorf("@for body: %w", err)
	}
	node.Body = body

	// Optional @empty { ... }
	p.skipWhitespaceAndNewlines()
	if p.startsWith("@empty") {
		p.advanceN(6)
		p.skipWhitespaceAndNewlines()
		if p.peek() != '{' {
			return nil, p.errorf("@empty: expected '{'")
		}
		p.advance()
		empty, err := p.parseNodes(true)
		if err != nil {
			return nil, p.errorf("@empty body: %w", err)
		}
		node.Empty = empty
	}

	return node, nil
}

// parseSwitchDirective parses @switch(expr) { @case(...) { ... } @default { ... } }
func (p *parser) parseSwitchDirective() (*SwitchNode, error) {
	p.advanceN(7) // skip '@switch'
	expr, err := p.readParenExpr()
	if err != nil {
		return nil, p.errorf("@switch: %w", err)
	}
	node := &SwitchNode{Expr: expr}

	p.skipWhitespaceAndNewlines()
	if p.peek() != '{' {
		return nil, p.errorf("@switch: expected '{'")
	}
	p.advance() // skip outer '{'

	// Parse @case and @default blocks until '}'
	for {
		p.skipWhitespaceAndNewlines()
		if p.eof() {
			return nil, p.errorf("@switch: unclosed block")
		}
		if p.peek() == '}' {
			p.advance()
			break
		}
		if p.startsWith("@case") {
			p.advanceN(5)
			values, err := p.readParenExpr()
			if err != nil {
				return nil, p.errorf("@case: %w", err)
			}
			// values can be comma-separated
			valParts := splitCaseValues(values)
			p.skipWhitespaceAndNewlines()
			if p.peek() != '{' {
				return nil, p.errorf("@case: expected '{'")
			}
			p.advance()
			body, err := p.parseNodes(true)
			if err != nil {
				return nil, p.errorf("@case body: %w", err)
			}
			node.Cases = append(node.Cases, SwitchCase{Values: valParts, Body: body})
		} else if p.startsWith("@default") {
			p.advanceN(8)
			p.skipWhitespaceAndNewlines()
			if p.peek() != '{' {
				return nil, p.errorf("@default: expected '{'")
			}
			p.advance()
			body, err := p.parseNodes(true)
			if err != nil {
				return nil, p.errorf("@default body: %w", err)
			}
			node.Default = body
		} else {
			return nil, p.errorf("@switch: unexpected content, expected @case or @default")
		}
	}

	return node, nil
}

// parseMatchDirective parses @match(expr) { @case(pattern) { ... } @case(pattern if guard) { ... } @default { ... } }
func (p *parser) parseMatchDirective() (*MatchNode, error) {
	p.advanceN(6) // skip '@match'
	expr, err := p.readParenExpr()
	if err != nil {
		return nil, p.errorf("@match: %w", err)
	}
	node := &MatchNode{Expr: expr}

	p.skipWhitespaceAndNewlines()
	if p.peek() != '{' {
		return nil, p.errorf("@match: expected '{'")
	}
	p.advance() // skip outer '{'

	for {
		p.skipWhitespaceAndNewlines()
		if p.eof() {
			return nil, p.errorf("@match: unclosed block")
		}
		if p.peek() == '}' {
			p.advance()
			break
		}
		if p.startsWith("@case") {
			p.advanceN(5)
			patternStr, err := p.readParenExpr()
			if err != nil {
				return nil, p.errorf("@match @case: %w", err)
			}
			// Split pattern and guard on " if " (but not inside strings/parens)
			pattern, guard := splitPatternGuard(patternStr)
			p.skipWhitespaceAndNewlines()
			if p.peek() != '{' {
				return nil, p.errorf("@match @case: expected '{'")
			}
			p.advance()
			body, err := p.parseNodes(true)
			if err != nil {
				return nil, p.errorf("@match @case body: %w", err)
			}
			node.Cases = append(node.Cases, MatchCase{PatternExpr: pattern, Guard: guard, Body: body})
		} else if p.startsWith("@default") {
			p.advanceN(8)
			p.skipWhitespaceAndNewlines()
			if p.peek() != '{' {
				return nil, p.errorf("@match @default: expected '{'")
			}
			p.advance()
			body, err := p.parseNodes(true)
			if err != nil {
				return nil, p.errorf("@match @default body: %w", err)
			}
			node.Default = body
		} else {
			return nil, p.errorf("@match: unexpected content, expected @case or @default")
		}
	}

	return node, nil
}

// splitPatternGuard splits a pattern string on top-level " if " to separate guard from pattern.
func splitPatternGuard(s string) (pattern, guard string) {
	depth := 0
	inStr := rune(0)
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		ch := runes[i]
		if inStr != 0 {
			if ch == inStr && (i == 0 || runes[i-1] != '\\') {
				inStr = 0
			}
			continue
		}
		if ch == '"' || ch == '\'' || ch == '`' {
			inStr = ch
			continue
		}
		if ch == '(' || ch == '[' || ch == '{' {
			depth++
		} else if ch == ')' || ch == ']' || ch == '}' {
			depth--
		}
		if depth == 0 && ch == ' ' && i+4 <= len(runes) && string(runes[i:i+4]) == " if " {
			return strings.TrimSpace(string(runes[:i])), strings.TrimSpace(string(runes[i+4:]))
		}
	}
	return strings.TrimSpace(s), ""
}

// splitCaseValues splits case values by commas, respecting strings.
func splitCaseValues(s string) []string {
	var parts []string
	var buf strings.Builder
	depth := 0
	inStr := rune(0)
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		ch := runes[i]
		if inStr != 0 {
			buf.WriteRune(ch)
			if ch == '\\' && i+1 < len(runes) {
				i++
				buf.WriteRune(runes[i])
			} else if ch == inStr {
				inStr = 0
			}
			continue
		}
		switch ch {
		case '"', '\'':
			inStr = ch
			buf.WriteRune(ch)
		case '(', '{', '[':
			depth++
			buf.WriteRune(ch)
		case ')', '}', ']':
			depth--
			buf.WriteRune(ch)
		case ',':
			if depth == 0 {
				parts = append(parts, strings.TrimSpace(buf.String()))
				buf.Reset()
			} else {
				buf.WriteRune(ch)
			}
		default:
			buf.WriteRune(ch)
		}
	}
	if buf.Len() > 0 {
		parts = append(parts, strings.TrimSpace(buf.String()))
	}
	return parts
}

func splitTopLevelSemicolons(s string) []string {
	var parts []string
	var buf strings.Builder
	depth := 0
	inStr := rune(0)
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		ch := runes[i]
		if inStr != 0 {
			buf.WriteRune(ch)
			if ch == '\\' && i+1 < len(runes) {
				i++
				buf.WriteRune(runes[i])
			} else if ch == inStr {
				inStr = 0
			}
			continue
		}
		switch ch {
		case '"', '\'', '`':
			inStr = ch
			buf.WriteRune(ch)
		case '(', '{', '[':
			depth++
			buf.WriteRune(ch)
		case ')', '}', ']':
			if depth > 0 {
				depth--
			}
			buf.WriteRune(ch)
		case ';':
			if depth == 0 {
				parts = append(parts, strings.TrimSpace(buf.String()))
				buf.Reset()
				continue
			}
			buf.WriteRune(ch)
		default:
			buf.WriteRune(ch)
		}
	}
	parts = append(parts, strings.TrimSpace(buf.String()))
	return parts
}

// parseRaw parses @raw { ... } — everything inside is literal text.
func (p *parser) parseRaw() (*RawNode, error) {
	p.advanceN(4) // skip '@raw'
	p.skipWhitespaceAndNewlines()
	if p.peek() != '{' {
		return nil, p.errorf("@raw: expected '{'")
	}
	p.advance() // skip '{'
	var buf strings.Builder
	depth := 1
	for !p.eof() && depth > 0 {
		ch := p.peek()
		if ch == '{' {
			depth++
			buf.WriteRune(p.advance())
		} else if ch == '}' {
			depth--
			if depth == 0 {
				p.advance()
				break
			}
			buf.WriteRune(p.advance())
		} else {
			buf.WriteRune(p.advance())
		}
	}
	if depth != 0 {
		return nil, p.errorf("@raw: unclosed block")
	}
	return &RawNode{Text: buf.String()}, nil
}

// parseInclude parses @include("path") or @include("path", dataExpr)
func (p *parser) parseInclude() (*IncludeNode, error) {
	p.advanceN(8) // skip '@include'
	inner, err := p.readParenExpr()
	if err != nil {
		return nil, p.errorf("@include: %w", err)
	}
	parts := splitCaseValues(inner)
	if len(parts) == 0 {
		return nil, p.errorf("@include: path is required")
	}
	path := unquote(strings.TrimSpace(parts[0]))
	node := &IncludeNode{Path: path}
	if len(parts) > 1 {
		node.DataExpr = strings.TrimSpace(parts[1])
	}
	return node, nil
}

// parseImportDirective parses @import("components.html")
func (p *parser) parseImportDirective() (*ImportNode, error) {
	p.advanceN(7) // skip '@import'
	inner, err := p.readParenExpr()
	if err != nil {
		return nil, p.errorf("@import: %w", err)
	}
	path := unquote(strings.TrimSpace(inner))
	if path == "" {
		return nil, p.errorf("@import: path is required")
	}
	return &ImportNode{Path: path}, nil
}

// parseHandlerDirective parses @handler(name = expr)
func (p *parser) parseHandlerDirective() (*HandlerNode, error) {
	p.advanceN(8) // skip '@handler'
	inner, err := p.readParenExpr()
	if err != nil {
		return nil, p.errorf("@handler: %w", err)
	}
	trimmed := strings.TrimSpace(inner)
	if trimmed == "" {
		return nil, p.errorf("@handler: handler name is required")
	}
	if idx := findFirstAssignEquals(inner); idx >= 0 {
		name := strings.TrimSpace(inner[:idx])
		expr := strings.TrimSpace(inner[idx+1:])
		if name == "" || expr == "" {
			return nil, p.errorf("@handler: name and expression are required")
		}
		return &HandlerNode{Name: name, Expr: expr}, nil
	}
	p.skipWhitespaceAndNewlines()
	if p.peek() != '{' {
		return nil, p.errorf("@handler: expected '{' for multiline handler body")
	}
	body, err := p.readRawBlock()
	if err != nil {
		return nil, p.errorf("@handler body: %w", err)
	}
	return &HandlerNode{Name: trimmed, Body: strings.TrimSpace(body)}, nil
}

func (p *parser) readRawBlock() (string, error) {
	if p.peek() != '{' {
		return "", p.errorf("expected '{'")
	}
	p.advance()
	var buf strings.Builder
	depth := 1
	for !p.eof() && depth > 0 {
		ch := p.peek()
		if ch == '{' {
			depth++
			buf.WriteRune(p.advance())
			continue
		}
		if ch == '}' {
			depth--
			if depth == 0 {
				p.advance()
				break
			}
			buf.WriteRune(p.advance())
			continue
		}
		// Skip // line comments — don't parse quotes inside them
		if ch == '/' && p.peekAt(1) == '/' {
			for !p.eof() && p.peek() != '\n' {
				buf.WriteRune(p.advance())
			}
			continue
		}
		if ch == '"' || ch == '\'' || ch == '`' {
			buf.WriteString(p.readStringLiteral())
			continue
		}
		buf.WriteRune(p.advance())
	}
	if depth != 0 {
		return "", p.errorf("unclosed handler block")
	}
	return buf.String(), nil
}

// parseExtends parses @extends("layout.html")
func (p *parser) parseExtends() (*ExtendsNode, error) {
	p.advanceN(8) // skip '@extends'
	inner, err := p.readParenExpr()
	if err != nil {
		return nil, p.errorf("@extends: %w", err)
	}
	return &ExtendsNode{Path: unquote(strings.TrimSpace(inner))}, nil
}

// parseBlock parses @block("name") { ... }
func (p *parser) parseBlock() (*BlockNode, error) {
	p.advanceN(6) // skip '@block'
	inner, err := p.readParenExpr()
	if err != nil {
		return nil, p.errorf("@block: %w", err)
	}
	name := unquote(strings.TrimSpace(inner))
	p.skipWhitespaceAndNewlines()
	if p.peek() != '{' {
		return nil, p.errorf("@block: expected '{'")
	}
	p.advance()
	body, err := p.parseNodes(true)
	if err != nil {
		return nil, p.errorf("@block body: %w", err)
	}
	return &BlockNode{Name: name, Body: body}, nil
}

// parseDefine parses @define("name") { ... }
func (p *parser) parseDefine() (*DefineNode, error) {
	p.advanceN(7) // skip '@define'
	inner, err := p.readParenExpr()
	if err != nil {
		return nil, p.errorf("@define: %w", err)
	}
	name := unquote(strings.TrimSpace(inner))
	p.skipWhitespaceAndNewlines()
	if p.peek() != '{' {
		return nil, p.errorf("@define: expected '{'")
	}
	p.advance()
	body, err := p.parseNodes(true)
	if err != nil {
		return nil, p.errorf("@define body: %w", err)
	}
	return &DefineNode{Name: name, Body: body}, nil
}

// parsePrepend parses @prepend("name") { ... }
func (p *parser) parsePrepend() (*PrependNode, error) {
	p.advanceN(8) // skip '@prepend'
	inner, err := p.readParenExpr()
	if err != nil {
		return nil, p.errorf("@prepend: %w", err)
	}
	name := unquote(strings.TrimSpace(inner))
	p.skipWhitespaceAndNewlines()
	if p.peek() != '{' {
		return nil, p.errorf("@prepend: expected '{'")
	}
	p.advance()
	body, err := p.parseNodes(true)
	if err != nil {
		return nil, p.errorf("@prepend body: %w", err)
	}
	return &PrependNode{Name: name, Body: body}, nil
}

// parseAppend parses @append("name") { ... }
func (p *parser) parseAppend() (*AppendNode, error) {
	p.advanceN(7) // skip '@append'
	inner, err := p.readParenExpr()
	if err != nil {
		return nil, p.errorf("@append: %w", err)
	}
	name := unquote(strings.TrimSpace(inner))
	p.skipWhitespaceAndNewlines()
	if p.peek() != '{' {
		return nil, p.errorf("@append: expected '{'")
	}
	p.advance()
	body, err := p.parseNodes(true)
	if err != nil {
		return nil, p.errorf("@append body: %w", err)
	}
	return &AppendNode{Name: name, Body: body}, nil
}

// parseHasBlock parses @hasBlock("name") { ... } @else { ... }
func (p *parser) parseHasBlock() (*HasBlockNode, error) {
	p.advanceN(9) // skip '@hasBlock'
	inner, err := p.readParenExpr()
	if err != nil {
		return nil, p.errorf("@hasBlock: %w", err)
	}
	name := unquote(strings.TrimSpace(inner))
	p.skipWhitespaceAndNewlines()
	if p.peek() != '{' {
		return nil, p.errorf("@hasBlock: expected '{'")
	}
	p.advance()
	body, err := p.parseNodes(true)
	if err != nil {
		return nil, p.errorf("@hasBlock body: %w", err)
	}
	node := &HasBlockNode{Name: name, Body: body}
	// Optional @else
	p.skipWhitespaceAndNewlines()
	if p.startsWith("@else") {
		p.advanceN(5)
		p.skipWhitespaceAndNewlines()
		if p.peek() != '{' {
			return nil, p.errorf("@hasBlock @else: expected '{'")
		}
		p.advance()
		elseBody, err := p.parseNodes(true)
		if err != nil {
			return nil, p.errorf("@hasBlock @else body: %w", err)
		}
		node.Else = elseBody
	}
	return node, nil
}

// parseComponent parses @component("Name") { ... } or @component("Name", prop1, prop2 as alias = default) { ... }
func (p *parser) parseComponent() (*ComponentNode, error) {
	p.advanceN(10) // skip '@component'
	inner, err := p.readParenExpr()
	if err != nil {
		return nil, p.errorf("@component: %w", err)
	}

	// Parse name and optional prop declarations
	trimmed := strings.TrimSpace(inner)
	var name string
	var props []PropDef

	if len(trimmed) > 0 && (trimmed[0] == '"' || trimmed[0] == '\'') {
		// Find end of quoted name string
		quote := trimmed[0]
		end := 1
		for end < len(trimmed) {
			if trimmed[end] == '\\' {
				end += 2
				continue
			}
			if trimmed[end] == quote {
				end++
				break
			}
			end++
		}
		name = unquote(trimmed[:end])
		rest := strings.TrimSpace(trimmed[end:])
		// Parse comma-separated prop definitions after the name
		if len(rest) > 0 && rest[0] == ',' {
			propsPart := rest[1:]
			propTokens := splitCaseValues(propsPart)
			for _, tok := range propTokens {
				pd, err := parsePropDef(tok)
				if err != nil {
					return nil, p.errorf("@component %q: %w", name, err)
				}
				props = append(props, pd)
			}
		}
	} else {
		name = trimmed
	}

	p.skipWhitespaceAndNewlines()
	if p.peek() != '{' {
		return nil, p.errorf("@component: expected '{'")
	}
	p.advance()
	body, err := p.parseNodes(true)
	if err != nil {
		return nil, p.errorf("@component body: %w", err)
	}
	return &ComponentNode{Name: name, Props: props, Body: body}, nil
}

// parsePropDef parses a single prop definition token: "name", "name as alias", "name = default", "name as alias = default",
// with optional "?" prefix marking the prop as optional (NULL when not passed).
func parsePropDef(token string) (PropDef, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return PropDef{}, fmt.Errorf("empty prop definition")
	}

	pd := PropDef{}

	// Check for optional "?" prefix
	if strings.HasPrefix(token, "?") {
		pd.Optional = true
		token = strings.TrimSpace(token[1:])
	}

	// Split on first assignment '=' to separate name/alias from default
	eqIdx := findFirstAssignEquals(token)
	var namePart string
	if eqIdx >= 0 {
		namePart = strings.TrimSpace(token[:eqIdx])
		pd.Default = strings.TrimSpace(token[eqIdx+1:])
	} else {
		namePart = token
	}

	// Check for " as " alias
	asIdx := strings.Index(namePart, " as ")
	if asIdx >= 0 {
		pd.Name = strings.TrimSpace(namePart[:asIdx])
		pd.Alias = strings.TrimSpace(namePart[asIdx+4:])
	} else {
		pd.Name = strings.TrimSpace(namePart)
	}

	nameFields := strings.Fields(pd.Name)
	if len(nameFields) > 1 {
		pd.Name = nameFields[0]
		pd.Type = nameFields[1]
	}
	if pd.Alias != "" {
		aliasFields := strings.Fields(pd.Alias)
		if len(aliasFields) > 1 {
			pd.Alias = aliasFields[0]
			if pd.Type == "" {
				pd.Type = aliasFields[1]
			}
		}
	}

	if pd.Name == "" {
		return PropDef{}, fmt.Errorf("prop name is required")
	}
	switch pd.Type {
	case "", "string", "number", "bool", "boolean", "array", "object", "any":
	default:
		return PropDef{}, fmt.Errorf("unsupported prop type %q", pd.Type)
	}
	return pd, nil
}

// findFirstAssignEquals finds the index of the first assignment '=' that is not part of ==, !=, <=, >=.
// Returns -1 if not found. Respects string literals and bracket depth.
func findFirstAssignEquals(s string) int {
	runes := []rune(s)
	inStr := rune(0)
	depth := 0
	for i := 0; i < len(runes); i++ {
		ch := runes[i]
		if inStr != 0 {
			if ch == '\\' && i+1 < len(runes) {
				i++
			} else if ch == inStr {
				inStr = 0
			}
			continue
		}
		switch ch {
		case '"', '\'', '`':
			inStr = ch
		case '(', '{', '[':
			depth++
		case ')', '}', ']':
			depth--
		case '!':
			if i+1 < len(runes) && runes[i+1] == '=' {
				i++ // skip !=
			}
		case '<', '>':
			if i+1 < len(runes) && runes[i+1] == '=' {
				i++ // skip <=, >=
			}
		case '=':
			if depth == 0 {
				if i+1 < len(runes) && runes[i+1] == '=' {
					i++ // skip ==
					continue
				}
				return i
			}
		}
	}
	return -1
}

// parseRender parses @render("Name") { ... } or @render("Name", propsExpr) { ... }
func (p *parser) parseRender() (*RenderNode, error) {
	p.advanceN(7) // skip '@render'
	inner, err := p.readParenExpr()
	if err != nil {
		return nil, p.errorf("@render: %w", err)
	}

	// Split into name and optional props expression
	// Use splitCaseValues-like logic but only split on first comma after the name string
	trimmed := strings.TrimSpace(inner)
	var name, propsExpr string

	if len(trimmed) > 0 && (trimmed[0] == '"' || trimmed[0] == '\'') {
		// Find end of quoted string
		quote := trimmed[0]
		end := 1
		for end < len(trimmed) {
			if trimmed[end] == '\\' {
				end += 2
				continue
			}
			if trimmed[end] == quote {
				end++
				break
			}
			end++
		}
		name = unquote(trimmed[:end])
		rest := strings.TrimSpace(trimmed[end:])
		if len(rest) > 0 && rest[0] == ',' {
			propsExpr = strings.TrimSpace(rest[1:])
		}
	} else {
		// Unquoted — treat as name (no props)
		name = trimmed
	}

	p.skipWhitespaceAndNewlines()
	var children []Node
	if p.peek() == '{' {
		p.advance()
		children, err = p.parseNodes(true)
		if err != nil {
			return nil, p.errorf("@render body: %w", err)
		}
	}

	return &RenderNode{Name: name, PropsExpr: propsExpr, Children: children}, nil
}

// parseSlot parses @slot or @slot("name")
func (p *parser) parseSlot() (*SlotNode, error) {
	p.advanceN(5) // skip '@slot'
	p.skipWhitespace()
	var name string
	if p.peek() == '(' {
		inner, err := p.readParenExpr()
		if err != nil {
			return nil, p.errorf("@slot: %w", err)
		}
		name = unquote(strings.TrimSpace(inner))
	}
	return &SlotNode{Name: name}, nil
}

// parseFill parses @fill("name") { ... }
func (p *parser) parseFill() (*FillNode, error) {
	p.advanceN(5) // skip '@fill'
	inner, err := p.readParenExpr()
	if err != nil {
		return nil, p.errorf("@fill: %w", err)
	}
	name := unquote(strings.TrimSpace(inner))
	p.skipWhitespaceAndNewlines()
	if p.peek() != '{' {
		return nil, p.errorf("@fill: expected '{'")
	}
	p.advance()
	body, err := p.parseNodes(true)
	if err != nil {
		return nil, p.errorf("@fill body: %w", err)
	}
	return &FillNode{Name: name, Body: body}, nil
}

// parseLet parses @let(varName = expr)
func (p *parser) parseLet() (*LetNode, error) {
	p.advanceN(4) // skip '@let'
	inner, err := p.readParenExpr()
	if err != nil {
		return nil, p.errorf("@let: %w", err)
	}
	idx := findFirstAssignEquals(inner)
	if idx < 0 {
		return nil, p.errorf("@let: expected 'varName = expr' syntax")
	}
	varName := strings.TrimSpace(inner[:idx])
	expr := strings.TrimSpace(inner[idx+1:])
	if varName == "" || expr == "" {
		return nil, p.errorf("@let: variable name and expression are required")
	}
	return &LetNode{VarName: varName, Expr: expr}, nil
}

// parseComputed parses @computed(varName = expr)
func (p *parser) parseComputed() (*ComputedNode, error) {
	p.advanceN(9) // skip '@computed'
	inner, err := p.readParenExpr()
	if err != nil {
		return nil, p.errorf("@computed: %w", err)
	}
	idx := findFirstAssignEquals(inner)
	if idx < 0 {
		return nil, p.errorf("@computed: expected 'varName = expr' syntax")
	}
	varName := strings.TrimSpace(inner[:idx])
	expr := strings.TrimSpace(inner[idx+1:])
	if varName == "" || expr == "" {
		return nil, p.errorf("@computed: variable name and expression are required")
	}
	return &ComputedNode{VarName: varName, Expr: expr}, nil
}

// parseComputedClient parses @computedClient(name = expr, dep1, dep2).
func (p *parser) parseComputedClient() (*ComputedClientNode, error) {
	p.advanceN(15) // skip '@computedClient'
	inner, err := p.readParenExpr()
	if err != nil {
		return nil, p.errorf("@computedClient: %w", err)
	}
	parts := splitCaseValues(inner)
	if len(parts) == 0 {
		return nil, p.errorf("@computedClient: expected 'name = expr' syntax")
	}
	idx := findFirstAssignEquals(parts[0])
	if idx < 0 {
		return nil, p.errorf("@computedClient: expected 'name = expr' syntax")
	}
	name := strings.TrimSpace(parts[0][:idx])
	expr := strings.TrimSpace(parts[0][idx+1:])
	if name == "" || expr == "" {
		return nil, p.errorf("@computedClient: name and expression are required")
	}
	var deps []string
	for _, dep := range parts[1:] {
		if dep = strings.TrimSpace(dep); dep != "" {
			deps = append(deps, dep)
		}
	}
	return &ComputedClientNode{Name: name, Expr: expr, Deps: deps}, nil
}

// parseWatch parses @watch(expr) { body }
func (p *parser) parseWatch() (*WatchNode, error) {
	p.advanceN(6) // skip '@watch'
	expr, err := p.readParenExpr()
	if err != nil {
		return nil, p.errorf("@watch: %w", err)
	}
	p.skipWhitespaceAndNewlines()
	if p.peek() != '{' {
		return nil, p.errorf("@watch: expected '{'")
	}
	p.advance()
	body, err := p.parseNodes(true)
	if err != nil {
		return nil, p.errorf("@watch body: %w", err)
	}
	return &WatchNode{Expr: strings.TrimSpace(expr), Body: body}, nil
}

// parseSignal parses @signal(name = expr)
func (p *parser) parseSignal() (*SignalNode, error) {
	p.advanceN(7) // skip '@signal'
	inner, err := p.readParenExpr()
	if err != nil {
		return nil, p.errorf("@signal: %w", err)
	}
	idx := findFirstAssignEquals(inner)
	if idx < 0 {
		return nil, p.errorf("@signal: expected 'name = expr' syntax")
	}
	name := strings.TrimSpace(inner[:idx])
	expr := strings.TrimSpace(inner[idx+1:])
	if name == "" || expr == "" {
		return nil, p.errorf("@signal: name and expression are required")
	}
	return &SignalNode{Name: name, InitialExpr: expr}, nil
}

// parseLocal parses @local(name = expr), a component-instance signal.
func (p *parser) parseLocal() (*LocalNode, error) {
	p.advanceN(6) // skip '@local'
	inner, err := p.readParenExpr()
	if err != nil {
		return nil, p.errorf("@local: %w", err)
	}
	idx := findFirstAssignEquals(inner)
	if idx < 0 {
		return nil, p.errorf("@local: expected 'name = expr' syntax")
	}
	name := strings.TrimSpace(inner[:idx])
	expr := strings.TrimSpace(inner[idx+1:])
	if name == "" || expr == "" {
		return nil, p.errorf("@local: name and expression are required")
	}
	return &LocalNode{Name: name, InitialExpr: expr}, nil
}

// parseAssets parses @assets("dedupe") or @assets(mode = "raw").
func (p *parser) parseAssets() (*AssetsNode, error) {
	p.advanceN(7) // skip '@assets'
	inner, err := p.readParenExpr()
	if err != nil {
		return nil, p.errorf("@assets: %w", err)
	}
	mode := strings.TrimSpace(inner)
	if idx := findFirstAssignEquals(mode); idx >= 0 {
		key := strings.TrimSpace(mode[:idx])
		if key != "mode" {
			return nil, p.errorf("@assets: unknown option %q", key)
		}
		mode = strings.TrimSpace(mode[idx+1:])
	}
	mode = unquote(mode)
	switch mode {
	case "", "dedupe", "raw":
		return &AssetsNode{Mode: mode}, nil
	default:
		return nil, p.errorf("@assets: unsupported mode %q", mode)
	}
}

// parseBind parses @bind(signal) or @bind(signal, attr)
func (p *parser) parseBind() (*BindNode, error) {
	p.advanceN(5) // skip '@bind'
	inner, err := p.readParenExpr()
	if err != nil {
		return nil, p.errorf("@bind: %w", err)
	}
	parts := splitCaseValues(inner)
	if len(parts) == 0 {
		return nil, p.errorf("@bind: signal is required")
	}
	signal := strings.TrimSpace(parts[0])
	attr := "textContent"
	if len(parts) > 1 {
		attr = unquote(strings.TrimSpace(parts[1]))
	}
	if signal == "" {
		return nil, p.errorf("@bind: signal is required")
	}
	return &BindNode{Signal: signal, Attr: attr}, nil
}

// parseEffect parses @effect(dep1, dep2) { ... }
func (p *parser) parseEffect() (*EffectNode, error) {
	p.advanceN(7) // skip '@effect'
	inner, err := p.readParenExpr()
	if err != nil {
		return nil, p.errorf("@effect: %w", err)
	}
	deps := splitCaseValues(inner)
	for i := range deps {
		deps[i] = strings.TrimSpace(deps[i])
	}
	p.skipWhitespaceAndNewlines()
	if p.peek() != '{' {
		return nil, p.errorf("@effect: expected '{'")
	}
	p.advance()
	body, err := p.parseNodes(true)
	if err != nil {
		return nil, p.errorf("@effect body: %w", err)
	}
	return &EffectNode{Deps: deps, Body: body}, nil
}

// parseReactiveView parses @reactive(dep1, dep2) { ... }
func (p *parser) parseReactiveView() (*ReactiveViewNode, error) {
	p.advanceN(9) // skip '@reactive'
	inner, err := p.readParenExpr()
	if err != nil {
		return nil, p.errorf("@reactive: %w", err)
	}
	deps := splitCaseValues(inner)
	for i := range deps {
		deps[i] = strings.TrimSpace(deps[i])
	}
	p.skipWhitespaceAndNewlines()
	if p.peek() != '{' {
		return nil, p.errorf("@reactive: expected '{'")
	}
	p.advance()
	body, err := p.parseNodes(true)
	if err != nil {
		return nil, p.errorf("@reactive body: %w", err)
	}
	return &ReactiveViewNode{Deps: deps, Body: body}, nil
}

// parseClick parses @click(label, signal, action, value?)
func (p *parser) parseClick() (*ClickNode, error) {
	p.advanceN(6) // skip '@click'
	inner, err := p.readParenExpr()
	if err != nil {
		return nil, p.errorf("@click: %w", err)
	}
	parts := splitCaseValues(inner)
	if len(parts) < 3 {
		return nil, p.errorf("@click: expected label, signal, action")
	}
	node := &ClickNode{
		Label:  unquote(strings.TrimSpace(parts[0])),
		Signal: strings.TrimSpace(parts[1]),
		Action: unquote(strings.TrimSpace(parts[2])),
	}
	if len(parts) > 3 {
		node.Value = unquote(strings.TrimSpace(parts[3]))
	}
	if node.Label == "" || node.Signal == "" || node.Action == "" {
		return nil, p.errorf("@click: label, signal, and action are required")
	}
	return node, nil
}

// parseStream parses @stream { ... }
func (p *parser) parseStream() (*StreamNode, error) {
	p.advanceN(7) // skip '@stream'
	p.skipWhitespaceAndNewlines()
	if p.peek() != '{' {
		return nil, p.errorf("@stream: expected '{'")
	}
	p.advance()
	body, err := p.parseNodes(true)
	if err != nil {
		return nil, p.errorf("@stream body: %w", err)
	}
	return &StreamNode{Body: body}, nil
}

// parseDefer parses @defer { ... } [@fallback { ... }]
func (p *parser) parseDefer() (*DeferNode, error) {
	p.advanceN(6) // skip '@defer'
	p.skipWhitespaceAndNewlines()
	if p.peek() != '{' {
		return nil, p.errorf("@defer: expected '{'")
	}
	p.advance()
	body, err := p.parseNodes(true)
	if err != nil {
		return nil, p.errorf("@defer body: %w", err)
	}
	p.skipWhitespaceAndNewlines()
	var fallback []Node
	if p.startsWith("@fallback") {
		p.advanceN(9)
		p.skipWhitespaceAndNewlines()
		if p.peek() != '{' {
			return nil, p.errorf("@fallback: expected '{'")
		}
		p.advance()
		fallback, err = p.parseNodes(true)
		if err != nil {
			return nil, p.errorf("@fallback body: %w", err)
		}
	}
	return &DeferNode{Body: body, Fallback: fallback}, nil
}

// parseLazy parses @lazy(expr) { ... } [@fallback { ... }]
func (p *parser) parseLazy() (*LazyNode, error) {
	p.advanceN(5) // skip '@lazy'
	expr, err := p.readParenExpr()
	if err != nil {
		return nil, p.errorf("@lazy: %w", err)
	}
	p.skipWhitespaceAndNewlines()
	if p.peek() != '{' {
		return nil, p.errorf("@lazy: expected '{'")
	}
	p.advance()
	body, err := p.parseNodes(true)
	if err != nil {
		return nil, p.errorf("@lazy body: %w", err)
	}
	p.skipWhitespaceAndNewlines()
	var fallback []Node
	if p.startsWith("@fallback") {
		p.advanceN(9)
		p.skipWhitespaceAndNewlines()
		if p.peek() != '{' {
			return nil, p.errorf("@fallback: expected '{'")
		}
		p.advance()
		fallback, err = p.parseNodes(true)
		if err != nil {
			return nil, p.errorf("@fallback body: %w", err)
		}
	}
	return &LazyNode{Expr: strings.TrimSpace(expr), Body: body, Fallback: fallback}, nil
}

// readParenExpr reads content between matching '(' and ')'.
func (p *parser) readParenExpr() (string, error) {
	p.skipWhitespaceAndNewlines()
	if p.peek() != '(' {
		return "", p.errorf("expected '('")
	}
	p.advance() // skip '('
	var buf strings.Builder
	depth := 1
	for !p.eof() && depth > 0 {
		ch := p.peek()
		if ch == '(' {
			depth++
			buf.WriteRune(p.advance())
		} else if ch == ')' {
			depth--
			if depth == 0 {
				p.advance()
				break
			}
			buf.WriteRune(p.advance())
		} else if ch == '"' || ch == '\'' {
			buf.WriteString(p.readStringLiteral())
		} else {
			buf.WriteRune(p.advance())
		}
	}
	if depth != 0 {
		return "", p.errorf("unclosed '('")
	}
	return buf.String(), nil
}

func (p *parser) skipWhitespaceAndNewlines() {
	for !p.eof() {
		ch := p.peek()
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			p.advance()
		} else {
			break
		}
	}
}

// unquote removes surrounding quotes from a string if present.
func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// parseSchemaForm parses @schema_form("schemaName", dataExpr)
func (p *parser) parseSchemaForm() (*SchemaFormNode, error) {
	p.advanceN(12) // skip '@schema_form'
	inner, err := p.readParenExpr()
	if err != nil {
		return nil, p.errorf("@schema_form: %w", err)
	}
	parts := splitCaseValues(inner)
	if len(parts) < 1 {
		return nil, p.errorf("@schema_form: schema name is required")
	}
	name := unquote(strings.TrimSpace(parts[0]))
	node := &SchemaFormNode{SchemaName: name}
	if len(parts) > 1 {
		node.DataExpr = strings.TrimSpace(parts[1])
	}
	return node, nil
}

// parseSchemaDetail parses @schema_detail("schemaName", dataExpr)
func (p *parser) parseSchemaDetail() (*SchemaDetailNode, error) {
	p.advanceN(14) // skip '@schema_detail'
	inner, err := p.readParenExpr()
	if err != nil {
		return nil, p.errorf("@schema_detail: %w", err)
	}
	parts := splitCaseValues(inner)
	if len(parts) < 1 {
		return nil, p.errorf("@schema_detail: schema name is required")
	}
	name := unquote(strings.TrimSpace(parts[0]))
	node := &SchemaDetailNode{SchemaName: name}
	if len(parts) > 1 {
		node.DataExpr = strings.TrimSpace(parts[1])
	}
	return node, nil
}

// parseSchemaTable parses @schema_table("schemaName", itemsExpr)
func (p *parser) parseSchemaTable() (*SchemaTableNode, error) {
	p.advanceN(13) // skip '@schema_table'
	inner, err := p.readParenExpr()
	if err != nil {
		return nil, p.errorf("@schema_table: %w", err)
	}
	parts := splitCaseValues(inner)
	if len(parts) < 1 {
		return nil, p.errorf("@schema_table: schema name is required")
	}
	name := unquote(strings.TrimSpace(parts[0]))
	node := &SchemaTableNode{SchemaName: name}
	if len(parts) > 1 {
		node.ItemsExpr = strings.TrimSpace(parts[1])
	}
	return node, nil
}
