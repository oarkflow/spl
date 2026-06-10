package spl

import (
	"fmt"
	"regexp"
	"strings"
)

type TemplateDiagnostic struct {
	Severity string
	Message  string
	Line     int
	Column   int
}

type TemplateAnalysis struct {
	Diagnostics []TemplateDiagnostic
	Signals     []string
	Components  []string
	Filters     []string
	Includes    []string
}

func (e *Engine) ValidateTemplate(tmpl string) []TemplateDiagnostic {
	if _, err := e.parseWithEngineDelims(tmpl); err != nil {
		line, col := diagnosticPosition(err.Error())
		return []TemplateDiagnostic{{
			Severity: "error",
			Message:  err.Error(),
			Line:     line,
			Column:   col,
		}}
	}
	return nil
}

func (e *Engine) AnalyzeTemplate(tmpl string) TemplateAnalysis {
	nodes, err := e.parseWithEngineDelims(tmpl)
	if err != nil {
		return TemplateAnalysis{Diagnostics: e.ValidateTemplate(tmpl)}
	}
	var analysis TemplateAnalysis
	seenSignals := make(map[string]struct{})
	seenComponents := make(map[string]struct{})
	seenFilters := make(map[string]struct{})
	seenIncludes := make(map[string]struct{})
	walkNodes(nodes, func(n Node) {
		switch v := n.(type) {
		case *SignalNode:
			addUnique(&analysis.Signals, seenSignals, v.Name)
		case *LocalNode:
			addUnique(&analysis.Signals, seenSignals, v.Name)
		case *ComponentNode:
			addUnique(&analysis.Components, seenComponents, v.Name)
		case *RenderNode:
			addUnique(&analysis.Components, seenComponents, v.Name)
		case *IncludeNode:
			addUnique(&analysis.Includes, seenIncludes, v.Path)
		case *ImportNode:
			addUnique(&analysis.Includes, seenIncludes, v.Path)
		case *ExprNode:
			for _, fc := range v.Filters {
				addUnique(&analysis.Filters, seenFilters, fc.Name)
				if _, ok := e.Filters[fc.Name]; !ok {
					analysis.Diagnostics = append(analysis.Diagnostics, TemplateDiagnostic{
						Severity: "warning",
						Message:  fmt.Sprintf("unknown filter %q", fc.Name),
					})
				}
			}
		}
	})
	return analysis
}

func FormatTemplate(tmpl string) string {
	lines := strings.Split(tmpl, "\n")
	indent := 0
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			lines[i] = ""
			continue
		}
		if strings.HasPrefix(trimmed, "}") || strings.HasPrefix(trimmed, "@else") || strings.HasPrefix(trimmed, "@elseif") || strings.HasPrefix(trimmed, "@empty") {
			if indent > 0 {
				indent--
			}
		}
		lines[i] = strings.Repeat("  ", indent) + trimmed
		if strings.HasSuffix(trimmed, "{") && !strings.HasPrefix(trimmed, "}") {
			indent++
		}
		if strings.HasSuffix(trimmed, "}") && !strings.HasPrefix(trimmed, "}") && indent > 0 {
			indent--
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func walkNodes(nodes []Node, fn func(Node)) {
	for _, n := range nodes {
		fn(n)
		switch v := n.(type) {
		case *IfNode:
			for _, branch := range v.Branches {
				walkNodes(branch.Body, fn)
			}
			walkNodes(v.Else, fn)
		case *ConditionalRenderNode:
			walkNodes(v.Body, fn)
		case *ForNode:
			walkNodes(v.Body, fn)
			walkNodes(v.Empty, fn)
		case *ComponentNode:
			walkNodes(v.Body, fn)
		case *RenderNode:
			walkNodes(v.Children, fn)
		case *FillNode:
			walkNodes(v.Body, fn)
		case *BlockNode:
			walkNodes(v.Body, fn)
		case *DefineNode:
			walkNodes(v.Body, fn)
		case *WatchNode:
			walkNodes(v.Body, fn)
		case *EffectNode:
			walkNodes(v.Body, fn)
		case *ReactiveViewNode:
			walkNodes(v.Body, fn)
		}
	}
}

func addUnique(dst *[]string, seen map[string]struct{}, value string) {
	if value == "" {
		return
	}
	if _, ok := seen[value]; ok {
		return
	}
	seen[value] = struct{}{}
	*dst = append(*dst, value)
}

var diagnosticPosRe = regexp.MustCompile(`at ([0-9]+):([0-9]+):`)

func diagnosticPosition(message string) (int, int) {
	matches := diagnosticPosRe.FindStringSubmatch(message)
	if len(matches) != 3 {
		return 0, 0
	}
	var line, col int
	fmt.Sscanf(matches[1], "%d", &line)
	fmt.Sscanf(matches[2], "%d", &col)
	return line, col
}
