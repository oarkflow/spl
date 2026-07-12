package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func demoRenderData() map[string]any {
	return map[string]any{
		"title": "SPL Template Engine &mdash; Fiber Demo",
		"countries": []Country{
			{Code: "us", Name: "United States", Region: "Americas"},
			{Code: "uk", Name: "United Kingdom", Region: "Europe"},
			{Code: "ca", Name: "Canada", Region: "Americas"},
		},
		"roles": []Role{
			{Value: "developer", Label: "Developer", Permissions: []string{"read", "write", "deploy"}},
			{Value: "designer", Label: "Designer", Permissions: []string{"read", "write"}},
		},
		"priorities": []Priority{PriorityLow, PriorityMedium, PriorityHigh},
		"config": FormConfig{
			MaxBioLength: 280,
			MinAge:       0,
			MaxAge:       150,
			AllowSignup:  true,
		},
		"regionColors": map[string]string{
			"Americas": "#3b82f6",
			"Europe":   "#22c55e",
		},
	}
}

func TestFiberDemoRendersContent(t *testing.T) {
	engine := New("./views")
	engine.SSR(true)
	engine.engine.Globals["siteName"] = "SPL Fiber Demo"
	if err := engine.Load(); err != nil {
		t.Fatalf("load views: %v", err)
	}
	var out strings.Builder
	if err := engine.Render(&out, "index", demoRenderData()); err != nil {
		t.Fatalf("render index: %v", err)
	}
	html := out.String()
	if !strings.Contains(html, "Forms") {
		t.Fatalf("expected tab content, got %q", html)
	}
	if !strings.Contains(html, "Load Quote") {
		t.Fatalf("expected quote section, got %q", html)
	}
	if !strings.Contains(html, "data-spl-hydration") {
		t.Fatalf("expected hydration payload, got %q", html)
	}
}

func TestFiberDemoRendersContentInSecureMode(t *testing.T) {
	engine := New("./views")
	engine.SSR(true)
	engine.engine.Globals["siteName"] = "SPL Fiber Demo"
	engine.engine.SecureMode = true
	if err := engine.Load(); err != nil {
		t.Fatalf("load views: %v", err)
	}
	var out strings.Builder
	if err := engine.Render(&out, "index", demoRenderData()); err != nil {
		t.Fatalf("render index in secure mode: %v", err)
	}
	html := out.String()
	if !strings.Contains(html, "data-spl-hydration") {
		t.Fatalf("expected hydration payload in secure mode, got %q", html)
	}
}

func TestFiberDemoRendersCacheableHydrationAsset(t *testing.T) {
	engine := New("./views")
	engine.SSR(true).HydrationRuntimeURL("/static/spl-runtime.js").HydrationAssets("/static")
	engine.engine.Globals["siteName"] = "SPL Fiber Demo"
	if err := engine.Load(); err != nil {
		t.Fatalf("load views: %v", err)
	}
	var out strings.Builder
	if err := engine.Render(&out, "index", demoRenderData()); err != nil {
		t.Fatalf("render index: %v", err)
	}
	html := out.String()
	prefix := `src="/static/spl-hydration.`
	start := strings.Index(html, prefix)
	if start < 0 || strings.Contains(html, `type="application/json" data-spl-hydration`) {
		t.Fatalf("expected external hydration asset, got %q", html)
	}
	start += len(`src="/static/`)
	end := strings.Index(html[start:], `"`)
	assetName := html[start : start+end]
	asset, ok := engine.HydrationAsset(assetName)
	if !ok || !strings.HasPrefix(asset, `window.__SPL_HYDRATE__({`) {
		t.Fatalf("expected stored executable hydration JS, got %q", asset)
	}
	if strings.Index(html, `data-spl-runtime`) > strings.Index(html, `data-spl-hydration`) {
		t.Fatal("runtime must load before executable hydration")
	}
}

func TestRenderReportsTemplateParseErrors(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "broken.html"), []byte("@if(true) {"), 0o600); err != nil {
		t.Fatal(err)
	}

	engine := New(dir)
	if err := engine.Load(); err != nil {
		t.Fatal(err)
	}
	err := engine.Render(&strings.Builder{}, "broken", nil)
	if err == nil || !strings.Contains(err.Error(), "render broken.html") {
		t.Fatalf("expected a template parse error, got %v", err)
	}
}
