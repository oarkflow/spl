package spl

import (
	"fmt"
	"strings"
)

// FragmentSelector specifies which fragment to render within a template.
type FragmentSelector struct {
	// Type selects the fragment strategy: "block", "component", "id", or "expr".
	Type string
	// Value is the fragment name or CSS selector.
	Value string
	// WrapID when true wraps the output in a div with data-spl-fragment="{Value}".
	WrapID bool
}

// findFragment finds a fragment node within a parsed node list.
func findFragment(nodes []Node, sel FragmentSelector) ([]Node, error) {
	switch sel.Type {
	case "block":
		return findBlock(nodes, sel.Value)
	case "component":
		return findComponent(nodes, sel.Value)
	case "id":
		return findElementByID(nodes, sel.Value)
	case "expr":
		return nodes, nil // return all nodes; caller evaluates the expression
	default:
		return nil, fmt.Errorf("unknown fragment type: %s", sel.Type)
	}
}

func findBlock(nodes []Node, name string) ([]Node, error) {
	for _, n := range nodes {
		if b, ok := n.(*BlockNode); ok && b.Name == name {
			return b.Body, nil
		}
		if c, ok := n.(*DefineNode); ok && c.Name == name {
			return c.Body, nil
		}
	}
	return nil, fmt.Errorf("block %q not found", name)
}

func findComponent(nodes []Node, name string) ([]Node, error) {
	for _, n := range nodes {
		if c, ok := n.(*RenderNode); ok && c.Name == name {
			return []Node{n}, nil
		}
	}
	return nil, fmt.Errorf("component render %q not found", name)
}

func findElementByID(nodes []Node, id string) ([]Node, error) {
	for _, n := range nodes {
		if text, ok := n.(*TextNode); ok {
			if strings.Contains(text.Text, `id="`+id+`"`) || strings.Contains(text.Text, `id='`+id+`'`) {
				return []Node{n}, nil
			}
		}
	}
	return nil, fmt.Errorf("element with id %q not found", id)
}

// RenderFragment renders a specific fragment of a template string.
// The fragment is identified by the FragmentSelector.
func (e *Engine) RenderFragment(tmpl string, sel FragmentSelector, data map[string]any) (string, error) {
	compiled, err := e.compileStringTemplate(tmpl)
	if err != nil {
		return "", fmt.Errorf("template parse error: %w", err)
	}

	fragmentNodes, err := findFragment(compiled.Nodes, sel)
	if err != nil {
		return "", err
	}

	out, err := e.renderCompiled(&compiledTemplate{Nodes: fragmentNodes, Components: compiled.Components, Imports: compiled.Imports}, data, e.hydration)
	if err != nil {
		return "", err
	}
	if err := e.ensureSecureRenderedHTML(out); err != nil {
		return "", err
	}
	if sel.WrapID {
		out = fmt.Sprintf(`<div data-spl-fragment="%s">%s</div>`, htmlEscape(sel.Value), out)
	}
	return out, nil
}

// RenderFragmentFile renders a specific fragment from a template file.
func (e *Engine) RenderFragmentFile(path string, sel FragmentSelector, data map[string]any) (string, error) {
	resolved, err := e.resolvePath(path)
	if err != nil {
		return "", fmt.Errorf("template file error (%s): %w", path, err)
	}
	compiled, err := e.compileFileTemplate(resolved)
	if err != nil {
		return "", fmt.Errorf("template file error (%s): %w", path, err)
	}

	fragmentNodes, err := findFragment(compiled.Nodes, sel)
	if err != nil {
		return "", err
	}

	out, err := e.renderCompiled(&compiledTemplate{Nodes: fragmentNodes, Components: compiled.Components, Imports: compiled.Imports}, data, e.hydration)
	if err != nil {
		return "", err
	}
	if err := e.ensureSecureRenderedHTML(out); err != nil {
		return "", err
	}
	if sel.WrapID {
		out = fmt.Sprintf(`<div data-spl-fragment="%s">%s</div>`, htmlEscape(sel.Value), out)
	}
	return out, nil
}

// htmlEscape escapes HTML special characters for fragment ID attributes.
func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, `&`, `&amp;`)
	s = strings.ReplaceAll(s, `"`, `&quot;`)
	s = strings.ReplaceAll(s, `'`, `&apos;`)
	s = strings.ReplaceAll(s, `<`, `&lt;`)
	s = strings.ReplaceAll(s, `>`, `&gt;`)
	return s
}
