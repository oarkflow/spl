package spl

import (
	"fmt"
	"html"
	"regexp"
	"strings"
)

var forbiddenHTMLPatterns = []struct {
	reason string
	re     *regexp.Regexp
}{
	{reason: "script tags are not allowed in secure mode", re: regexp.MustCompile(`(?i)<script\b`)},
	{reason: "inline event handlers are not allowed in secure mode", re: regexp.MustCompile(`(?i)\son[a-z0-9_-]+\s*=`)},
	{reason: "javascript: URLs are not allowed in secure mode", re: regexp.MustCompile(`(?i)\b(?:href|src|xlink:href|formaction)\s*=\s*(['"])?\s*javascript:`)},
	{reason: "srcdoc is not allowed in secure mode", re: regexp.MustCompile(`(?i)\ssrcdoc\s*=`)},
	{reason: "innerHTML bindings are not allowed in secure mode", re: regexp.MustCompile(`(?i)\bdata-spl-bind-(?:innerhtml|html)\s*=`)},
	{reason: "active embedded content is not allowed in secure mode", re: regexp.MustCompile(`(?i)<(?:iframe|object|embed)\b|<meta\b[^>]*http-equiv\s*=\s*(['"])?refresh\b`)},
}

var dangerousURLAttrRe = regexp.MustCompile(`(?is)\b(?:href|src|xlink:href|formaction)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))`)

func (e *Engine) ensureSecureRenderedHTML(rendered string) error {
	if !e.SecureMode {
		return nil
	}
	for _, pattern := range forbiddenHTMLPatterns {
		if pattern.re.FindStringIndex(rendered) != nil {
			return fmt.Errorf("%s", pattern.reason)
		}
	}
	if dangerousURLAttrRe.FindStringIndex(rendered) != nil {
		for _, match := range dangerousURLAttrRe.FindAllStringSubmatch(rendered, -1) {
			value := ""
			for i := 1; i < len(match); i++ {
				if match[i] != "" {
					value = match[i]
					break
				}
			}
			decoded := strings.ToLower(strings.TrimSpace(html.UnescapeString(value)))
			decoded = strings.Map(func(r rune) rune {
				if r <= 0x20 || r == 0x7f {
					return -1
				}
				return r
			}, decoded)
			if strings.HasPrefix(decoded, "javascript:") {
				return fmt.Errorf("javascript: URLs are not allowed in secure mode")
			}
		}
	}
	return nil
}
