// Command features demonstrates all new SPL template engine features.
//
// Usage:
//
//	go run ./cmd/features
package main

import (
	"embed"
	"fmt"
	"log"
	"strings"
	"time"

	template "github.com/oarkflow/spl"
)

//go:embed locales/*.json
var localesFS embed.FS

//go:embed views/fragment.html
var fragmentTmpl string

func main() {
	// -------------------------------------------------------
	// 1. DATE FILTER
	// -------------------------------------------------------
	fmt.Println("=== 1. Date Filter ===")

	engine := template.New()
	now := time.Now()

	out, err := engine.Render(`
Unix timestamp: ${ts | date}
RFC3339:       ${rfc | date "Jan 2, 2006 15:04"}
Simple date:   ${simple | date "Monday, Jan 2, 2006"}
Input/output:  ${rfc | date "2006-01-02" "Jan 2"}
`, map[string]any{
		"ts":     float64(now.Unix()),
		"rfc":    now.Format(time.RFC3339),
		"simple": now.Format("2006-01-02"),
	})
	if err != nil {
		log.Fatalf("date filter: %v", err)
	}
	fmt.Print(out)

	// -------------------------------------------------------
	// 2. REGISTER HELPER
	// -------------------------------------------------------
	fmt.Println("=== 2. RegisterHelper ===")

	engine.RegisterHelper("greet", func(args ...any) any {
		if len(args) == 0 {
			return "Hello, world!"
		}
		name, _ := args[0].(string)
		return "Hello, " + name + "!"
	})

	engine.RegisterHelper("add", func(args ...any) any {
		var sum float64
		for _, a := range args {
			switch v := a.(type) {
			case float64:
				sum += v
			case int:
				sum += float64(v)
			case int64:
				sum += float64(v)
			}
		}
		return sum
	})

	engine.RegisterHelper("upper", func(args ...any) any {
		if len(args) == 0 {
			return ""
		}
		s, _ := args[0].(string)
		return strings.ToUpper(s)
	})

	out, err = engine.Render(`
${greet(name)}
${greet()}
10 + 20 = ${add(x, y)}
${upper("hello world")}
`, map[string]any{"name": "Alice", "x": 10, "y": 20})
	if err != nil {
		log.Fatalf("register helper: %v", err)
	}
	fmt.Print(out)

	// -------------------------------------------------------
	// 3. CUSTOM DELIMITERS
	// -------------------------------------------------------
	fmt.Println("=== 3. Custom Delimiters ===")

	customDelim := template.New()
	customDelim.DelimLeft = "{{"
	customDelim.DelimRight = "}}"

	out, err = customDelim.Render(`
Hello, {{name | upper}}!
Count: {{count}}
`, map[string]any{"name": "world", "count": 42})
	if err != nil {
		log.Fatalf("custom delimiters: %v", err)
	}
	fmt.Print(out)

	// -------------------------------------------------------
	// 4. EMBEDDED FILESYSTEM
	// -------------------------------------------------------
	fmt.Println("=== 4. Embedded FS Support ===")

	fsEngine := template.New()
	fsEngine.FS = localesFS

	// RenderFSFile reads and renders a template from the embedded FS
	out, err = fsEngine.RenderFSFile("locales/en.json", nil)
	if err != nil {
		log.Fatalf("render fs file: %v", err)
	}
	fmt.Println("Locale file content:", out)

	// embedded FS also works with @include, @extends, @import via engine.FS
	fsEngine2 := template.New()
	fsEngine2.FS = localesFS
	out, err = fsEngine2.Render(`<p>FS support active</p>`, nil)
	if err != nil {
		log.Fatalf("fs engine: %v", err)
	}
	fmt.Println(out)

	// -------------------------------------------------------
	// 5. FRAGMENT RENDERING
	// -------------------------------------------------------
	fmt.Println("=== 5. Fragment Rendering ===")

	fragEngine := template.New()

	out, err = fragEngine.RenderFragment(fragmentTmpl,
		template.FragmentSelector{Type: "block", Value: "content"},
		map[string]any{"title": "My Page"},
	)
	if err != nil {
		log.Fatalf("fragment content: %v", err)
	}
	fmt.Println("Content block:")
	fmt.Print(out)

	out, err = fragEngine.RenderFragment(fragmentTmpl,
		template.FragmentSelector{Type: "block", Value: "sidebar"},
		map[string]any{"title": "My Page"},
	)
	if err != nil {
		log.Fatalf("fragment sidebar: %v", err)
	}
	fmt.Println("Sidebar block:")
	fmt.Print(out)

	// -------------------------------------------------------
	// 6. MIDDLEWARE / LIFECYCLE HOOKS
	// -------------------------------------------------------
	fmt.Println("=== 6. Middleware / Lifecycle Hooks ===")

	hookEngine := template.New()

	var renderCount int
	hookEngine.Hooks.BeforeRender = func(ctx *template.RenderContext) error {
		renderCount++
		fmt.Printf("  [hook] before render #%d\n", renderCount)
		return nil
	}
	hookEngine.Hooks.AfterRender = func(ctx *template.RenderContext) error {
		fmt.Printf("  [hook] after render #%d\n", renderCount)
		return nil
	}
	hookEngine.Hooks.OnError = func(ctx *template.RenderContext, err error) {
		fmt.Printf("  [hook] error: %v\n", err)
	}

	out, err = hookEngine.Render(`<p>Hello, ${name}!</p>`, map[string]any{"name": "World"})
	if err != nil {
		log.Fatalf("hook render: %v", err)
	}
	fmt.Print(out)
	fmt.Printf("Total renders: %d\n\n", renderCount)

	// -------------------------------------------------------
	// 7. I18N TRANSLATION
	// -------------------------------------------------------
	fmt.Println("=== 7. i18n Translation Layer ===")

	i18nEngine := template.New()
	i18nEngine.I18n = template.NewI18nConfig("en")

	enBytes, _ := localesFS.ReadFile("locales/en.json")
	if err := i18nEngine.I18n.LoadMessages("en", enBytes); err != nil {
		log.Fatalf("load en locale: %v", err)
	}

	frBytes, _ := localesFS.ReadFile("locales/fr.json")
	if err := i18nEngine.I18n.LoadMessages("fr", frBytes); err != nil {
		log.Fatalf("load fr locale: %v", err)
	}

	out, err = i18nEngine.Render(`
@translate("greeting")
@translate("welcome_user", "en")
@translate("welcome_user", "fr")
@translate("nonexistent.key") {Fallback content}
`, nil)
	if err != nil {
		log.Fatalf("i18n render: %v", err)
	}
	fmt.Print(out)

	// -------------------------------------------------------
	// 8. BUILD-TIME TEMPLATE COMPILATION
	// -------------------------------------------------------
	fmt.Println("=== 8. Build-Time Template Compilation ===")

	compEngine := template.New()
	compiled, err := compEngine.CompileToGo("hello", `<h1>${title}</h1><p>${message}</p>`)
	if err != nil {
		log.Fatalf("compile to go: %v", err)
	}
	fmt.Println("Generated Go source:")
	fmt.Println(string(compiled))

	// -------------------------------------------------------
	fmt.Println("All features demonstrated successfully!")
}
