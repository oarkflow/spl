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
	"time"

	template "github.com/oarkflow/spl"
)

//go:embed locales/*.json
var localesFS embed.FS

//go:embed views/fragment.html
var fragmentTmpl string

//go:embed views/layout.html views/child.html
var viewsFS embed.FS

func main() {
	// -------------------------------------------------------
	// 1. DATE FILTER
	// -------------------------------------------------------
	fmt.Println("=== 1. Date Filter ===")

	now := time.Now()
	out, err := template.New().Render(`
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

	eng := template.New()
	eng.RegisterHelper("greet", func(args ...any) any {
		if len(args) == 0 {
			return "Hello, world!"
		}
		name, _ := args[0].(string)
		return "Hello, " + name + "!"
	})
	eng.RegisterHelper("add", func(args ...any) any {
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

	out, err = eng.Render(`
${greet(name)}
${greet()}
10 + 20 = ${add(x, y)}
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

	out, err = fsEngine.RenderFSFile("locales/en.json", nil)
	if err != nil {
		log.Fatalf("render fs file: %v", err)
	}
	fmt.Println("Locale file content:", out)

	// -------------------------------------------------------
	// 5. CACHE TTL & CONFIGURABLE CACHE POLICY
	// -------------------------------------------------------
	fmt.Println("=== 5. Cache TTL ===")

	cacheEngine := template.New()
	cacheEngine.CachePolicy.CompiledTextTTL = 60 // 60 seconds

	out, err = cacheEngine.Render(`<p>Cached with TTL: ${msg}</p>`, map[string]any{"msg": "hello"})
	if err != nil {
		log.Fatalf("cache ttl: %v", err)
	}
	fmt.Print(out)
	fmt.Println("  (CompiledTextTTL set to 60s — entry will expire in 1 minute)")

	// -------------------------------------------------------
	// 6. @cache DIRECTIVE (FRAGMENT CACHING)
	// -------------------------------------------------------
	fmt.Println("=== 6. @cache Directive ===")

	cacheDirEngine := template.New()
	cacheDirEngine.CachePolicy.FragmentTTL = 30

	out, err = cacheDirEngine.Render(`
@cache("my-fragment", 10) {
  <div class="cached-fragment">
    <p>This fragment is cached for 10 seconds.</p>
    <p>Generated at: ${now("15:04:05")}</p>
  </div>
}
`, nil)
	if err != nil {
		log.Fatalf("@cache directive: %v", err)
	}
	fmt.Print(out)
	fmt.Println("  (Run again within 10s to see cached output)")

	// -------------------------------------------------------
	// 7. BUILT-IN EXPRESSION HELPERS
	// -------------------------------------------------------
	fmt.Println("=== 7. Built-in Helpers ===")

	out, err = template.New().Render(`
slice:    ${slice(items, 0, 2)}
first:    ${first(items)}
last:     ${last(items)}
length:   ${len(items)}
has red:  ${has(items, "red")}
join:     ${join(items, ", ")}
upper:    ${upper("hello")}
lower:    ${lower("WORLD")}
defaults:  ${defaults(name, "Guest")}
json:     ${json(info)}
orElse:   ${orElse(empty, name, "fallback")}
`, map[string]any{
		"items": []any{"red", "green", "blue"},
		"name":  "Alice",
		"info":  map[string]any{"role": "admin", "active": true},
		"empty": "",
	})
	if err != nil {
		log.Fatalf("built-in helpers: %v", err)
	}
	fmt.Print(out)

	// -------------------------------------------------------
	// 8. ADVANCED INHERITANCE (@prepend / @append / @hasBlock)
	// -------------------------------------------------------
	fmt.Println("=== 8. Advanced Inheritance ===")

	inheritEngine := template.New()
	inheritEngine.FS = viewsFS
	inheritEngine.Globals = map[string]any{
		"year": time.Now().Year(),
	}

	out, err = inheritEngine.RenderFSFile("views/child.html", map[string]any{
		"title": "My Page",
		"items": []any{"Apple", "Banana", "Cherry"},
	})
	if err != nil {
		log.Fatalf("advanced inheritance: %v", err)
	}
	fmt.Print(out)

	// -------------------------------------------------------
	// 9. FRAGMENT RENDERING
	// -------------------------------------------------------
	fmt.Println("=== 9. Fragment Rendering ===")

	fragEngine := template.New()
	out, err = fragEngine.RenderFragment(fragmentTmpl,
		template.FragmentSelector{Type: "block", Value: "content"},
		map[string]any{"title": "My Page"},
	)
	if err != nil {
		log.Fatalf("fragment content: %v", err)
	}
	fmt.Println("Content block:", out)

	out, err = fragEngine.RenderFragment(fragmentTmpl,
		template.FragmentSelector{Type: "block", Value: "sidebar"},
		map[string]any{"title": "My Page"},
	)
	if err != nil {
		log.Fatalf("fragment sidebar: %v", err)
	}
	fmt.Println("Sidebar block:", out)

	// -------------------------------------------------------
	// 10. MIDDLEWARE / LIFECYCLE HOOKS
	// -------------------------------------------------------
	fmt.Println("=== 10. Middleware / Lifecycle Hooks ===")

	hookEngine := template.New()
	hookEngine.Hooks.BeforeRender = func(ctx *template.RenderContext) error {
		fmt.Println("  [hook] before render")
		return nil
	}
	hookEngine.Hooks.AfterRender = func(ctx *template.RenderContext) error {
		fmt.Println("  [hook] after render")
		return nil
	}

	out, err = hookEngine.Render(`<p>Hello, ${name}!</p>`, map[string]any{"name": "World"})
	if err != nil {
		log.Fatalf("hook render: %v", err)
	}
	fmt.Print(out)

	// -------------------------------------------------------
	// 11. I18N TRANSLATION
	// -------------------------------------------------------
	fmt.Println("=== 11. i18n Translation Layer ===")

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
	// 12. DEBUG FILTER
	// -------------------------------------------------------
	fmt.Println("=== 12. Debug Filter ===")

	dubugOut, err := template.New().Render(`
${name | debug}
${name | debug "user"}
`, map[string]any{"name": "Alice"})
	if err != nil {
		log.Fatalf("debug filter: %v", err)
	}
	fmt.Print(dubugOut)

	// -------------------------------------------------------
	// 13. BUILD-TIME TEMPLATE COMPILATION
	// -------------------------------------------------------
	fmt.Println("=== 13. Build-Time Template Compilation ===")

	compiled, err := template.New().CompileToGo("hello", `<h1>${title}</h1><p>${message}</p>`)
	if err != nil {
		log.Fatalf("compile to go: %v", err)
	}
	fmt.Println("Generated Go source:")
	fmt.Println(string(compiled))

	// -------------------------------------------------------
	fmt.Println("\nAll 13 features demonstrated successfully!")
}
