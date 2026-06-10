package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/oarkflow/template"
)

type Country struct {
	Code   string
	Name   string
	Region string
}

type Role struct {
	Value       string
	Label       string
	Permissions []string
}

type Priority string

const (
	PriorityLow      Priority = "low"
	PriorityMedium   Priority = "medium"
	PriorityHigh     Priority = "high"
	PriorityCritical Priority = "critical"
)

type FormConfig struct {
	MaxBioLength int
	MinAge       int
	MaxAge       int
	AllowSignup  bool
}

type SPLViews struct {
	engine    *template.Engine
	directory string
	extension string
	reload    bool
	ssr       bool
}

func New(directory string, extension ...string) *SPLViews {
	ext := ".html"
	if len(extension) > 0 && extension[0] != "" {
		ext = extension[0]
	}
	return &SPLViews{
		engine:    template.New(),
		directory: directory,
		extension: ext,
	}
}

func (v *SPLViews) Reload(enabled bool) *SPLViews {
	v.reload = enabled
	return v
}

func (v *SPLViews) SSR(enabled bool) *SPLViews {
	v.ssr = enabled
	return v
}

func (v *SPLViews) HydrationRuntimeURL(url string) *SPLViews {
	v.engine.HydrationRuntimeURL = url
	return v
}

func (v *SPLViews) Load() error {
	v.engine.BaseDir = v.directory
	v.engine.AutoEscape = true
	return filepath.Walk(v.directory, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		if !strings.HasSuffix(path, v.extension) {
			return nil
		}
		rel, _ := filepath.Rel(v.directory, path)
		_, _ = v.engine.RenderFile(rel, nil)
		return nil
	})
}

func (v *SPLViews) Render(w io.Writer, name string, binding any, layout ...string) error {
	if v.reload {
		v.engine.InvalidateCaches()
	}
	data, ok := binding.(map[string]any)
	if !ok {
		if binding == nil {
			data = make(map[string]any)
		} else if fm, ok := binding.(fiber.Map); ok {
			data = fm
		} else {
			return fmt.Errorf("spl: binding must be map[string]any or fiber.Map, got %T", binding)
		}
	}
	for k, val := range v.engine.Globals {
		if _, exists := data[k]; !exists {
			data[k] = val
		}
	}
	if !strings.HasSuffix(name, v.extension) {
		name += v.extension
	}
	if len(layout) > 0 && layout[0] != "" {
		layoutName := layout[0]
		if !strings.HasSuffix(layoutName, v.extension) {
			layoutName += v.extension
		}
		tmplPath := filepath.Join(v.directory, name)
		content, err := os.ReadFile(tmplPath)
		if err != nil {
			return fmt.Errorf("spl: read %s: %w", name, err)
		}
		wrapped := fmt.Sprintf("@extends(%q)\n%s", layoutName, string(content))
		var out string
		if v.ssr {
			out, err = v.engine.RenderSSR(wrapped, data)
		} else {
			out, err = v.engine.Render(wrapped, data)
		}
		if err != nil {
			return fmt.Errorf("spl: render %s with layout %s: %w", name, layoutName, err)
		}
		_, err = io.WriteString(w, out)
		return err
	}
	var out string
	var err error
	if v.ssr {
		out, err = v.engine.RenderSSRFile(name, data)
	} else {
		out, err = v.engine.RenderFile(name, data)
	}
	if err != nil {
		return fmt.Errorf("spl: render %s: %w", name, err)
	}
	_, err = io.WriteString(w, out)
	return err
}

func main() {
	engine := New("./views")
	engine.Reload(true)
	engine.SSR(true)
	engine.engine.SecureMode = true

	runtimeVersion := runtimeAssetVersion(engine.engine.RuntimeJS())
	engine.HydrationRuntimeURL("/static/spl-runtime.min.js?v=" + runtimeVersion)

	engine.engine.Globals["siteName"] = "SPL Fiber Demo"

	app := fiber.New(fiber.Config{Views: engine})

	app.Use(func(c fiber.Ctx) error {
		c.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; object-src 'none'")
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")
		return c.Next()
	})

	app.Get("/static/spl-runtime.min.js", func(c fiber.Ctx) error {
		c.Set("Content-Type", "application/javascript")
		c.Set("Cache-Control", "public, max-age=31536000, immutable")
		return c.SendString(engine.engine.RuntimeJS())
	})

	app.Get("/", func(c fiber.Ctx) error {
		return c.Render("index", fiber.Map{
			"title": "SPL Template Engine &mdash; Fiber Demo",
			"countries": []Country{
				{Code: "us", Name: "United States", Region: "Americas"},
				{Code: "uk", Name: "United Kingdom", Region: "Europe"},
				{Code: "ca", Name: "Canada", Region: "Americas"},
				{Code: "au", Name: "Australia", Region: "Oceania"},
				{Code: "de", Name: "Germany", Region: "Europe"},
				{Code: "jp", Name: "Japan", Region: "Asia"},
				{Code: "in", Name: "India", Region: "Asia"},
			},
			"roles": []Role{
				{Value: "developer", Label: "Developer", Permissions: []string{"read", "write", "deploy"}},
				{Value: "designer", Label: "Designer", Permissions: []string{"read", "write"}},
				{Value: "manager", Label: "Project Manager", Permissions: []string{"read", "write", "admin"}},
				{Value: "devops", Label: "DevOps Engineer", Permissions: []string{"read", "write", "deploy", "admin"}},
			},
			"priorities": []Priority{PriorityLow, PriorityMedium, PriorityHigh, PriorityCritical},
			"config": FormConfig{
				MaxBioLength: 280,
				MinAge:       0,
				MaxAge:       150,
				AllowSignup:  true,
			},
			"regionColors": map[string]string{
				"Americas": "#3b82f6",
				"Europe":   "#22c55e",
				"Asia":     "#f59e0b",
				"Oceania":  "#a855f7",
			},
		})
	})

	app.Post("/api/submit", func(c fiber.Ctx) error {
		var payload map[string]any
		if err := c.Bind().JSON(&payload); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid JSON", "success": false})
		}
		return c.JSON(fiber.Map{
			"success":   true,
			"message":   "Form submitted successfully!",
			"id":        fmt.Sprintf("SUB-%d", 1000+len(payload)),
			"timestamp": time.Now().Format(time.RFC3339),
		})
	})

	var quoteIdx int
	app.Get("/api/quote", func(c fiber.Ctx) error {
		quotes := []map[string]string{
			{"text": "The only way to do great work is to love what you do.", "author": "Steve Jobs"},
			{"text": "Code is like humor. When you have to explain it, it's bad.", "author": "Cory House"},
			{"text": "First, solve the problem. Then, write the code.", "author": "John Johnson"},
			{"text": "Simplicity is the soul of efficiency.", "author": "Austin Freeman"},
			{"text": "Make it work, make it right, make it fast.", "author": "Kent Beck"},
		}
		idx := quoteIdx % len(quotes)
		quoteIdx++
		return c.JSON(quotes[idx])
	})

	var (
		todoMu     sync.Mutex
		todos      []map[string]any
		todoNextID int
	)

	app.Get("/api/todos", func(c fiber.Ctx) error {
		todoMu.Lock()
		list := todos
		todoMu.Unlock()
		if list == nil {
			list = []map[string]any{}
		}
		return c.JSON(list)
	})

	app.Post("/api/todos", func(c fiber.Ctx) error {
		var form map[string]any
		if err := c.Bind().JSON(&form); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid JSON"})
		}
		todoMu.Lock()
		todoNextID++
		todos = append(todos, map[string]any{
			"id":       todoNextID,
			"title":    form["title"],
			"priority": form["priority"],
			"notes":    form["notes"],
		})
		list := make([]map[string]any, len(todos))
		copy(list, todos)
		todoMu.Unlock()
		return c.Status(201).JSON(list)
	})

	log.Println("SPL Fiber demo on http://localhost:3000")
	log.Fatal(app.Listen(":3000"))
}

func runtimeAssetVersion(src string) string {
	sum := sha256.Sum256([]byte(src))
	return hex.EncodeToString(sum[:8])
}
