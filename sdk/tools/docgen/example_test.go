package docgen_test

import (
	"fmt"
	"strings"

	"github.com/opendecree/decree/sdk/tools/docgen"
)

func ExampleGenerate() {
	schema := docgen.Schema{
		Name:        "app-config",
		Description: "Application configuration",
		Version:     1,
		Fields: []docgen.Field{
			{Path: "app.env", Type: "string", Description: "Deployment environment"},
			{Path: "app.port", Type: "integer", Description: "HTTP listen port"},
		},
	}

	md := docgen.Generate(schema)
	lines := strings.SplitN(md, "\n", 2)
	fmt.Println(lines[0])
	// Output:
	// # app-config
}

func ExampleGenerate_withoutGrouping() {
	schema := docgen.Schema{
		Name: "flat-config",
		Fields: []docgen.Field{
			{Path: "db.host", Type: "string"},
			{Path: "db.port", Type: "integer"},
		},
	}

	md := docgen.Generate(schema, docgen.WithoutGrouping())
	fmt.Println(strings.Contains(md, "## db"))
	fmt.Println(strings.Contains(md, "### `db.host`"))
	// Output:
	// false
	// true
}
