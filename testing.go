package spl

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// AssertRender verifies that rendering tmpl with data produces the expected string.
// It calls t.Fatal on mismatch with a diff-style message.
func AssertRender(t *testing.T, engine *Engine, tmpl string, data map[string]any, expected string) {
	t.Helper()
	result, err := engine.Render(tmpl, data)
	if err != nil {
		t.Fatalf("render error:\n  template: %q\n  error:    %v", tmpl, err)
	}
	if result != expected {
		t.Fatalf("render mismatch:\n  template: %q\n  expected: %q\n  got:      %q", tmpl, expected, result)
	}
}

// AssertRenderSub is like AssertRender but calls t.Errorf instead of t.Fatalf.
func AssertRenderSub(t *testing.T, engine *Engine, tmpl string, data map[string]any, expected string) {
	t.Helper()
	result, err := engine.Render(tmpl, data)
	if err != nil {
		t.Errorf("render error:\n  template: %q\n  error:    %v", tmpl, err)
		return
	}
	if result != expected {
		t.Errorf("render mismatch:\n  template: %q\n  expected: %q\n  got:      %q", tmpl, expected, result)
	}
}

// AssertRenderError verifies that rendering tmpl with data produces an error.
func AssertRenderError(t *testing.T, engine *Engine, tmpl string, data map[string]any) {
	t.Helper()
	_, err := engine.Render(tmpl, data)
	if err == nil {
		t.Fatalf("expected error for template:\n  template: %q\n  data:     %v", tmpl, data)
	}
}

// AssertSnapshot verifies that rendering tmpl with data matches a golden file.
// The golden file is stored at <testdata>/<name>.golden relative to the test's directory.
// Pass "update" to force-update golden files.
func AssertSnapshot(t *testing.T, engine *Engine, name, tmpl string, data map[string]any) {
	t.Helper()
	result, err := engine.Render(tmpl, data)
	if err != nil {
		t.Fatalf("snapshot %q render error: %v", name, err)
	}

	goldenPath := filepath.Join("testdata", name+".golden")

	// Check for update flag via -update flag or SNAPSHOT_UPDATE env var
	update := false
	for _, arg := range os.Args[1:] {
		if arg == "-update" || arg == "--update" || arg == "-u" {
			update = true
			break
		}
	}
	if !update {
		if v := os.Getenv("SNAPSHOT_UPDATE"); v == "1" || strings.EqualFold(v, "true") {
			update = true
		}
	}

	if update {
		dir := filepath.Dir(goldenPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("snapshot %q mkdir: %v", name, err)
		}
		if err := os.WriteFile(goldenPath, []byte(result), 0644); err != nil {
			t.Fatalf("snapshot %q write: %v", name, err)
		}
		return
	}

	expected, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("snapshot %q: golden file not found at %s (use -update to create)", name, goldenPath)
	}

	expectedStr := string(expected)
	if result != expectedStr {
		t.Fatalf("snapshot %q mismatch:\n  expected: %q\n  got:      %q", name, expectedStr, result)
	}
}

// FormatDiff returns a human-readable diff of two strings for test output.
func FormatDiff(expected, got string) string {
	expLines := strings.Split(expected, "\n")
	gotLines := strings.Split(got, "\n")
	var b strings.Builder
	max := len(expLines)
	if len(gotLines) > max {
		max = len(gotLines)
	}
	b.WriteString("--- expected\n+++ got\n")
	for i := 0; i < max; i++ {
		var e, g string
		if i < len(expLines) {
			e = expLines[i]
		}
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if e != g {
			if e != "" {
				fmt.Fprintf(&b, "-%s\n", e)
			}
			if g != "" {
				fmt.Fprintf(&b, "+%s\n", g)
			}
		}
	}
	return b.String()
}
