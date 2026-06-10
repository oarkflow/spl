package spl

import (
	"fmt"
	"hash/fnv"
	"regexp"
	"strings"
)

var (
	styleTagRe      = regexp.MustCompile(`(?is)<style\b([^>]*)>(.*?)</style>`)
	scriptTagRe     = regexp.MustCompile(`(?is)<script\b([^>]*)>(.*?)</script>`)
	linkTagRe       = regexp.MustCompile(`(?is)<link\b[^>]*>`)
	classSelectorRe = regexp.MustCompile(`\.([A-Za-z_][A-Za-z0-9_-]*)`)
	idSelectorRe    = regexp.MustCompile(`#([A-Za-z_][A-Za-z0-9_-]*)`)
	classAttrRe     = regexp.MustCompile(`(?is)\bclass\s*=\s*("([^"]*)"|'([^']*)')`)
	idAttrRe        = regexp.MustCompile(`(?is)\bid\s*=\s*("([^"]*)"|'([^']*)')`)
)

func optimizeRenderedAssets(html string) string {
	if html == "" {
		return html
	}
	html = dedupeAssetTags(html, styleTagRe, "style")
	html = dedupeAssetTags(html, scriptTagRe, "script")
	html = dedupeLinkTags(html)
	html = normalizeHTMLWhitespace(html)
	return html
}

var blankLineRe = regexp.MustCompile(`\n{3,}`)

func normalizeHTMLWhitespace(html string) string {
	return blankLineRe.ReplaceAllString(html, "\n\n")
}

func dedupeAssetTags(html string, re *regexp.Regexp, tag string) string {
	seen := make(map[string]struct{})
	return re.ReplaceAllStringFunc(html, func(match string) string {
		attrs, body, ok := splitPairedAsset(match, tag)
		if !ok || hasUniqueAssetAttr(attrs) {
			return match
		}
		if tag == "script" && !isJavaScriptAsset(attrs) {
			return match
		}
		key := tag + ":" + compactAssetWhitespace(attrs) + ":" + strings.TrimSpace(body)
		if _, exists := seen[key]; exists {
			return ""
		}
		seen[key] = struct{}{}
		return match
	})
}

func dedupeLinkTags(html string) string {
	seen := make(map[string]struct{})
	return linkTagRe.ReplaceAllStringFunc(html, func(match string) string {
		attrs := trimTagShell(match, "link")
		if hasUniqueAssetAttr(attrs) {
			return match
		}
		if !isStylesheetLink(attrs) {
			return match
		}
		key := "link:" + compactAssetWhitespace(attrs)
		if _, exists := seen[key]; exists {
			return ""
		}
		seen[key] = struct{}{}
		return match
	})
}

func splitPairedAsset(tagHTML, tag string) (attrs, body string, ok bool) {
	lower := strings.ToLower(tagHTML)
	openPrefix := "<" + tag
	if !strings.HasPrefix(lower, openPrefix) {
		return "", "", false
	}
	openEnd := strings.Index(tagHTML, ">")
	if openEnd < 0 {
		return "", "", false
	}
	closeStart := strings.LastIndex(lower, "</"+tag+">")
	if closeStart < openEnd {
		return "", "", false
	}
	return tagHTML[len(openPrefix):openEnd], tagHTML[openEnd+1 : closeStart], true
}

func trimTagShell(tagHTML, tag string) string {
	start := len("<" + tag)
	end := strings.LastIndex(tagHTML, ">")
	if end < start {
		return ""
	}
	return tagHTML[start:end]
}

func compactAssetWhitespace(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

func hasUniqueAssetAttr(attrs string) bool {
	return strings.EqualFold(attrValue(attrs, "data-spl-unique"), "true")
}

func hasScopeAttr(attrs string) bool {
	return attrValue(attrs, "data-spl-scope") != ""
}

func isStylesheetLink(attrs string) bool {
	rel := strings.ToLower(attrValue(attrs, "rel"))
	as := strings.ToLower(attrValue(attrs, "as"))
	return strings.Contains(rel, "stylesheet") || (strings.Contains(rel, "preload") && as == "style")
}

func isJavaScriptAsset(attrs string) bool {
	typ := strings.ToLower(strings.TrimSpace(attrValue(attrs, "type")))
	if typ == "" {
		return true
	}
	switch typ {
	case "module", "text/javascript", "application/javascript", "application/ecmascript", "text/ecmascript":
		return true
	default:
		return false
	}
}

func attrValue(attrs, name string) string {
	attrs = strings.TrimSpace(attrs)
	name = strings.ToLower(name)
	for i := 0; i < len(attrs); {
		for i < len(attrs) && isHTMLSpace(attrs[i]) {
			i++
		}
		start := i
		for i < len(attrs) && isAttrNameChar(attrs[i]) {
			i++
		}
		if start == i {
			i++
			continue
		}
		key := strings.ToLower(attrs[start:i])
		for i < len(attrs) && isHTMLSpace(attrs[i]) {
			i++
		}
		value := ""
		if i < len(attrs) && attrs[i] == '=' {
			i++
			for i < len(attrs) && isHTMLSpace(attrs[i]) {
				i++
			}
			if i < len(attrs) && (attrs[i] == '"' || attrs[i] == '\'') {
				quote := attrs[i]
				i++
				valueStart := i
				for i < len(attrs) && attrs[i] != quote {
					i++
				}
				value = attrs[valueStart:i]
				if i < len(attrs) {
					i++
				}
			} else {
				valueStart := i
				for i < len(attrs) && !isHTMLSpace(attrs[i]) {
					i++
				}
				value = attrs[valueStart:i]
			}
		}
		if key == name {
			return value
		}
	}
	return ""
}

func isHTMLSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\f'
}

func isAttrNameChar(b byte) bool {
	return b == '-' || b == ':' || b == '_' || (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9')
}

func (e *Engine) nextAssetScope() string {
	e.assetScopeSeq++
	return fmt.Sprintf("spl-u-%d", e.assetScopeSeq)
}

// contentScope produces a deterministic scope string from CSS content.
// Identical CSS always yields the same scope, enabling deduplication.
func contentScope(css string) string {
	h := fnv.New32a()
	h.Write([]byte(css))
	return fmt.Sprintf("spl-%x", h.Sum32())
}

// scopeComponentAssets scopes <style> tags by rewriting CSS selectors and HTML
// class/id attributes. Uses a content-hash scope so identical CSS produces the
// same scope ID, allowing deduplication. Does NOT inject data-spl-* attributes.
func (e *Engine) scopeComponentAssets(html string) string {
	classNames := make(map[string]string)
	idNames := make(map[string]string)

	html = styleTagRe.ReplaceAllStringFunc(html, func(match string) string {
		attrs, css, ok := splitPairedAsset(match, "style")
		if !ok || hasScopeAttr(attrs) {
			return match
		}
		scope := contentScope(css)
		css = scopeCSS(css, scope, classNames, idNames)
		// Return style tag WITHOUT data-spl-* attributes
		if strings.TrimSpace(attrs) == "" {
			return "<style>" + css + "</style>"
		}
		return "<style" + attrs + ">" + css + "</style>"
	})

	if len(classNames) > 0 {
		html = rewriteClassAttributes(html, classNames)
	}
	if len(idNames) > 0 {
		html = rewriteIDAttributes(html, idNames)
	}
	return html
}

func (e *Engine) scopeUniqueComponentAssets(html string) string {
	scope := e.nextAssetScope()
	classNames := make(map[string]string)
	idNames := make(map[string]string)

	html = styleTagRe.ReplaceAllStringFunc(html, func(match string) string {
		attrs, css, ok := splitPairedAsset(match, "style")
		if !ok || !hasUniqueAssetAttr(attrs) || hasScopeAttr(attrs) {
			return match
		}
		css = scopeCSS(css, scope, classNames, idNames)
		attrs = ensureAssetScopeAttr(attrs, scope)
		return "<style" + attrs + ">" + css + "</style>"
	})

	if len(classNames) > 0 {
		html = rewriteClassAttributes(html, classNames)
	}
	if len(idNames) > 0 {
		html = rewriteIDAttributes(html, idNames)
	}

	html = scriptTagRe.ReplaceAllStringFunc(html, func(match string) string {
		attrs, js, ok := splitPairedAsset(match, "script")
		if !ok || !hasUniqueAssetAttr(attrs) || !isJavaScriptAsset(attrs) {
			return match
		}
		attrs = ensureAssetScopeAttr(attrs, scope)
		return "<script" + attrs + ">(function(){\nconst SPL_UNIQUE_SCOPE = " + quoteJS(scope) + ";\n" + js + "\n})();</script>"
	})

	html = linkTagRe.ReplaceAllStringFunc(html, func(match string) string {
		attrs := trimTagShell(match, "link")
		if !hasUniqueAssetAttr(attrs) {
			return match
		}
		return "<link" + ensureAssetScopeAttr(attrs, scope) + ">"
	})

	return html
}

func scopeCSS(css, scope string, classNames, idNames map[string]string) string {
	var out strings.Builder
	out.Grow(len(css) + 32)
	var quote byte
	comment := false
	urlDepth := 0
	for i := 0; i < len(css); {
		if comment {
			if i+1 < len(css) && css[i] == '*' && css[i+1] == '/' {
				comment = false
				out.WriteString("*/")
				i += 2
				continue
			}
			out.WriteByte(css[i])
			i++
			continue
		}
		if quote != 0 {
			out.WriteByte(css[i])
			if css[i] == '\\' && i+1 < len(css) {
				i++
				out.WriteByte(css[i])
			} else if css[i] == quote {
				quote = 0
			}
			i++
			continue
		}
		if i+1 < len(css) && css[i] == '/' && css[i+1] == '*' {
			comment = true
			out.WriteString("/*")
			i += 2
			continue
		}
		if css[i] == '"' || css[i] == '\'' {
			quote = css[i]
			out.WriteByte(css[i])
			i++
			continue
		}
		if startsCSSURL(css, i) {
			urlDepth = 1
			out.WriteString(css[i : i+4])
			i += 4
			continue
		}
		if urlDepth > 0 {
			if css[i] == '(' {
				urlDepth++
			} else if css[i] == ')' {
				urlDepth--
			}
			out.WriteByte(css[i])
			i++
			continue
		}
		if (css[i] == '.' || css[i] == '#') && looksLikeSelectorPrefix(css, i) && i+1 < len(css) && isCSSIdentStart(css[i+1]) {
			prefix := css[i]
			j := i + 2
			for j < len(css) && isCSSIdentChar(css[j]) {
				j++
			}
			name := css[i+1 : j]
			if strings.HasPrefix(name, "spl-u-") {
				out.WriteString(css[i:j])
				i = j
				continue
			}
			scoped := scopedAssetName(name, scope)
			if prefix == '.' {
				classNames[name] = scoped
			} else {
				idNames[name] = scoped
			}
			out.WriteByte(prefix)
			out.WriteString(scoped)
			i = j
			continue
		}
		out.WriteByte(css[i])
		i++
	}
	return out.String()
}

func startsCSSURL(css string, i int) bool {
	return i+4 <= len(css) &&
		(css[i] == 'u' || css[i] == 'U') &&
		(css[i+1] == 'r' || css[i+1] == 'R') &&
		(css[i+2] == 'l' || css[i+2] == 'L') &&
		css[i+3] == '('
}

func looksLikeSelectorPrefix(css string, marker int) bool {
	for i := marker - 1; i >= 0; i-- {
		switch css[i] {
		case ' ', '\t', '\n', '\r':
			continue
		case ',', '>', '+', '~', '(', '{', '}', '[':
			return true
		case ':', ';':
			return false
		default:
			return false
		}
	}
	return true
}

func isCSSIdentStart(b byte) bool {
	return b == '_' || b == '-' || (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}

func isCSSIdentChar(b byte) bool {
	return isCSSIdentStart(b) || (b >= '0' && b <= '9')
}

func scopedAssetName(name, scope string) string {
	return name + "--" + scope
}

func ensureAssetScopeAttr(attrs, scope string) string {
	if strings.TrimSpace(attrs) == "" {
		return ` data-spl-scope="` + scope + `"`
	}
	if strings.Contains(strings.ToLower(attrs), "data-spl-scope") {
		return attrs
	}
	return attrs + ` data-spl-scope="` + scope + `"`
}

func rewriteClassAttributes(html string, names map[string]string) string {
	return classAttrRe.ReplaceAllStringFunc(html, func(match string) string {
		parts := classAttrRe.FindStringSubmatch(match)
		if len(parts) < 4 {
			return match
		}
		quote := `"`
		value := parts[2]
		if parts[3] != "" {
			quote = `'`
			value = parts[3]
		}
		tokens := strings.Fields(value)
		for i, token := range tokens {
			if scoped, ok := names[token]; ok {
				tokens[i] = scoped
			}
		}
		return "class=" + quote + strings.Join(tokens, " ") + quote
	})
}

func rewriteIDAttributes(html string, names map[string]string) string {
	return idAttrRe.ReplaceAllStringFunc(html, func(match string) string {
		parts := idAttrRe.FindStringSubmatch(match)
		if len(parts) < 4 {
			return match
		}
		quote := `"`
		value := parts[2]
		if parts[3] != "" {
			quote = `'`
			value = parts[3]
		}
		if scoped, ok := names[value]; ok {
			value = scoped
		}
		return "id=" + quote + value + quote
	})
}

func quoteJS(s string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\r", `\r`)
	return `"` + replacer.Replace(s) + `"`
}
