// Command render demonstrates parsing and rendering an SPL template string.
package main

import (
	"fmt"
	"log"

	template "github.com/oarkflow/spl"
)

func main() {
	engine := template.New()
	source := `<h1>${title}</h1>
@if(items) {
<ul>
@for(item in items) {
  <li>${item | title}</li>
}
</ul>
} @else {
<p>No items found.</p>
}`

	// Render parses the source, evaluates it with the supplied data, and caches
	// the parsed template so repeated renders avoid parsing it again.
	output, err := engine.Render(source, map[string]any{
		"title": "Template parsing and rendering",
		"items": []string{"parse template", "provide data", "render html"},
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(output)
}
