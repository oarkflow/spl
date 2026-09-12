package spl

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// These tests compare rendering a template against rendering the VS Code
// extension's formatted copy of the same template. Set SPL_FMT_DIR to a
// directory holding formatted copies of testdata/templates; the extension's
// `npm run verify:go` script produces one.

var (
	fmtWsRe   = regexp.MustCompile(`(\s|\\n|\\t|\\r)+`)
	fmtTagRe  = regexp.MustCompile(`<[^>]*>`)
	fmtGapRe  = regexp.MustCompile(`\s+(/?>)`)
	fmtPairRe = regexp.MustCompile(`^(.*)=(.*)$`)
)

func fmtDir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("SPL_FMT_DIR")
	if dir == "" {
		t.Skip("SPL_FMT_DIR not set")
	}
	return dir
}

// textOf is the visible text of a rendered page with whitespace collapsed the
// way a browser collapses it. It catches words being joined or split.
func textOf(s string) string {
	return strings.TrimSpace(fmtWsRe.ReplaceAllString(fmtTagRe.ReplaceAllString(s, " "), " "))
}

// tagsOf is the tag sequence with insignificant whitespace inside each tag
// removed. It catches any change to element order, nesting, or attributes.
func tagsOf(s string) string {
	var b strings.Builder
	for _, tag := range fmtTagRe.FindAllString(s, -1) {
		b.WriteString(fmtGapRe.ReplaceAllString(fmtWsRe.ReplaceAllString(tag, " "), "$1"))
		b.WriteByte('\n')
	}
	return b.String()
}

// unescapeEmbedded turns the JSON-escaped markup inside the hydration payload
// back into real tags so it is compared as HTML rather than as opaque text.
func unescapeEmbedded(s string) string {
	r := strings.NewReplacer(`\u003c`, "<", `\u003e`, ">", `\u0026`, "&", `\n`, " ", `\t`, " ", `\"`, `"`)
	return r.Replace(s)
}

func assertEquivalent(t *testing.T, name, a, b string) {
	t.Helper()
	a, b = unescapeEmbedded(a), unescapeEmbedded(b)
	if ta, tb := textOf(a), textOf(b); ta != tb {
		t.Errorf("%s: visible text differs\n orig: %s\n fmt:  %s", name, firstDiff(ta, tb), firstDiff(tb, ta))
	}
	if ga, gb := tagsOf(a), tagsOf(b); ga != gb {
		t.Errorf("%s: tag sequence differs\n orig: %s\n fmt:  %s", name, firstDiff(ga, gb), firstDiff(gb, ga))
	}
}

func firstDiff(a, b string) string {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	lo, hi := i-80, i+160
	if lo < 0 {
		lo = 0
	}
	if hi > len(a) {
		hi = len(a)
	}
	return a[lo:hi]
}

// TestFormattedTemplatesParseTheSame asserts the formatter never turns a
// template the engine accepts into one it rejects (or vice versa).
func TestFormattedTemplatesParseTheSame(t *testing.T) {
	dir := fmtDir(t)
	entries, err := filepath.Glob(filepath.Join("testdata", "templates", "*.html"))
	if err != nil || len(entries) == 0 {
		t.Fatalf("no templates found: %v", err)
	}
	for _, orig := range entries {
		name := filepath.Base(orig)
		t.Run(name, func(t *testing.T) {
			a, err := os.ReadFile(orig)
			if err != nil {
				t.Fatal(err)
			}
			b, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			_, errA := parseWithDelims(string(a), "${", "}")
			_, errB := parseWithDelims(string(b), "${", "}")
			if (errA == nil) != (errB == nil) {
				t.Fatalf("parse result changed: original=%v formatted=%v", errA, errB)
			}
		})
	}
}

func renderBoth(t *testing.T, file string, data map[string]any, ssr bool) (string, string) {
	t.Helper()
	render := func(dir string) string {
		e := New()
		e.BaseDir = dir
		var out string
		var err error
		if ssr {
			out, err = e.RenderSSRFile(file, data)
		} else {
			out, err = e.RenderFile(file, data)
		}
		if err != nil {
			t.Fatalf("%s in %s: %v", file, dir, err)
		}
		return out
	}
	return render(filepath.Join("testdata", "templates")), render(fmtDir(t))
}

func TestFormattedReactiveShowcaseRendersIdentically(t *testing.T) {
	data := map[string]any{
		"siteTitle": "SPL UI", "pageTitle": "Complete Reactive Showcase",
		"description": "all major template features working together", "footerText": "footer",
		"counter": 2, "panelOpen": false, "apiBase": "http://127.0.0.1:3020",
		"userName": "sujit", "featureCount": 1, "searchQuery": "spl reactive templates",
		"summary":  "summary text for truncation",
		"navLinks": []any{map[string]any{"href": "/complete", "label": "Showcase"}},
		"features": []any{map[string]any{"name": "signals", "status": "working"}},
	}
	a, b := renderBoth(t, "complete_reactive_showcase.html", data, true)
	assertEquivalent(t, "complete_reactive_showcase.html", a, b)
}

func TestFormattedCardRendersIdentically(t *testing.T) {
	data := map[string]any{"title": "Card title", "body": "Some body text", "badge": "new"}
	a, b := renderBoth(t, "card.html", data, false)
	assertEquivalent(t, "card.html", a, b)
}
