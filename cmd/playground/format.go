package main

import "strings"

var playgroundVoidTags = map[string]bool{
	"area": true, "base": true, "br": true, "col": true, "embed": true,
	"hr": true, "img": true, "input": true, "link": true, "meta": true,
	"param": true, "source": true, "track": true, "wbr": true,
}

var playgroundCompactTags = map[string]bool{
	"a": true, "abbr": true, "b": true, "button": true, "code": true,
	"em": true, "h1": true, "h2": true, "h3": true, "h4": true,
	"h5": true, "h6": true, "label": true, "li": true, "option": true,
	"p": true, "small": true, "span": true, "strong": true, "td": true,
	"th": true, "title": true,
}

var playgroundInlineTags = map[string]bool{
	"a": true, "abbr": true, "b": true, "code": true, "em": true,
	"small": true, "span": true, "strong": true,
}

type htmlToken struct {
	kind string
	raw  string
	name string
}

func formatAPIHTML(src string) string {
	if strings.TrimSpace(src) == "" {
		return src
	}
	lower := strings.ToLower(src)
	scriptAt := strings.Index(lower, "<script")
	if scriptAt >= 0 {
		prefix := strings.TrimRight(formatHTMLFragment(src[:scriptAt]), "\n")
		suffix := strings.TrimLeft(src[scriptAt:], "\n")
		if prefix == "" {
			return suffix
		}
		return prefix + "\n" + suffix
	}
	return strings.TrimRight(formatHTMLFragment(src), "\n")
}

func formatHTMLFragment(src string) string {
	tokens := tokenizeHTML(src)
	var out strings.Builder
	indent := 0

	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		switch tok.kind {
		case "text":
			writeTextLines(&out, tok.raw, indent)
		case "comment", "doctype", "void":
			writeIndentedLine(&out, indent, strings.TrimSpace(tok.raw))
		case "close":
			if indent > 0 {
				indent--
			}
			writeIndentedLine(&out, indent, strings.TrimSpace(tok.raw))
		case "open":
			if inline, next := inlineLine(tokens, i); inline {
				writeIndentedLine(&out, indent, next)
				i = nextInlineIndex(tokens, i)
				continue
			}
			if compact, next := compactElement(tokens, i); compact {
				writeIndentedLine(&out, indent, next)
				i += 2
				continue
			}
			writeIndentedLine(&out, indent, strings.TrimSpace(tok.raw))
			if !playgroundVoidTags[tok.name] {
				indent++
			}
		}
	}
	return out.String()
}

func inlineLine(tokens []htmlToken, i int) (bool, string) {
	if i >= len(tokens) || tokens[i].kind != "open" || !playgroundInlineTags[tokens[i].name] {
		return false, ""
	}
	var line strings.Builder
	for i < len(tokens) {
		tok := tokens[i]
		switch tok.kind {
		case "open":
			if !playgroundInlineTags[tok.name] {
				return line.Len() > 0, strings.TrimSpace(line.String())
			}
			compact, text := compactElement(tokens, i)
			if !compact {
				return line.Len() > 0, strings.TrimSpace(line.String())
			}
			line.WriteString(text)
			i += 3
		case "text":
			text := tok.raw
			if idx := strings.IndexByte(text, '\n'); idx >= 0 {
				line.WriteString(text[:idx])
				return line.Len() > 0, strings.TrimSpace(line.String())
			}
			line.WriteString(text)
			i++
		default:
			return line.Len() > 0, strings.TrimSpace(line.String())
		}
	}
	return line.Len() > 0, strings.TrimSpace(line.String())
}

func nextInlineIndex(tokens []htmlToken, i int) int {
	for i < len(tokens) {
		tok := tokens[i]
		switch tok.kind {
		case "open":
			if !playgroundInlineTags[tok.name] {
				return i - 1
			}
			if compact, _ := compactElement(tokens, i); !compact {
				return i - 1
			}
			i += 3
		case "text":
			if strings.Contains(tok.raw, "\n") {
				return i
			}
			i++
		default:
			return i - 1
		}
	}
	return len(tokens) - 1
}

func compactElement(tokens []htmlToken, i int) (bool, string) {
	if i+2 >= len(tokens) {
		return false, ""
	}
	open := tokens[i]
	text := tokens[i+1]
	close := tokens[i+2]
	if open.kind != "open" || text.kind != "text" || close.kind != "close" || open.name != close.name {
		return false, ""
	}
	body := strings.TrimSpace(text.raw)
	if body == "" || strings.Contains(body, "\n") || !playgroundCompactTags[open.name] {
		return false, ""
	}
	return true, strings.TrimSpace(open.raw) + body + strings.TrimSpace(close.raw)
}

func tokenizeHTML(src string) []htmlToken {
	var tokens []htmlToken
	for len(src) > 0 {
		start := strings.IndexByte(src, '<')
		if start < 0 {
			tokens = append(tokens, htmlToken{kind: "text", raw: src})
			break
		}
		if start > 0 {
			tokens = append(tokens, htmlToken{kind: "text", raw: src[:start]})
			src = src[start:]
		}
		end := strings.IndexByte(src, '>')
		if end < 0 {
			tokens = append(tokens, htmlToken{kind: "text", raw: src})
			break
		}
		raw := src[:end+1]
		tokens = append(tokens, classifyHTMLTag(raw))
		src = src[end+1:]
	}
	return tokens
}

func classifyHTMLTag(raw string) htmlToken {
	trimmed := strings.TrimSpace(raw)
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "<!doctype") {
		return htmlToken{kind: "doctype", raw: raw}
	}
	if strings.HasPrefix(lower, "<!--") {
		return htmlToken{kind: "comment", raw: raw}
	}
	if strings.HasPrefix(lower, "</") {
		return htmlToken{kind: "close", raw: raw, name: tagName(trimmed[2:])}
	}
	name := tagName(trimmed[1:])
	if playgroundVoidTags[name] || strings.HasSuffix(trimmed, "/>") {
		return htmlToken{kind: "void", raw: raw, name: name}
	}
	return htmlToken{kind: "open", raw: raw, name: name}
}

func tagName(s string) string {
	s = strings.TrimSpace(s)
	for i, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '/' || r == '>' {
			return strings.ToLower(s[:i])
		}
	}
	return strings.ToLower(strings.TrimSuffix(s, ">"))
}

func writeTextLines(out *strings.Builder, text string, indent int) {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		writeIndentedLine(out, indent, line)
	}
}

func writeIndentedLine(out *strings.Builder, indent int, line string) {
	if line == "" {
		return
	}
	for i := 0; i < indent; i++ {
		out.WriteString("  ")
	}
	out.WriteString(line)
	out.WriteByte('\n')
}
