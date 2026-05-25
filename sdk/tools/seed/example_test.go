package seed_test

import (
	"fmt"

	"github.com/opendecree/decree/sdk/tools/seed"
)

func ExampleParseFile() {
	data := []byte(`spec_version: "v1"
schema:
  name: app-config
  fields:
    app.env:
      type: string
tenant:
  name: acme
config:
  description: initial setup
  values:
    app.env:
      value: production
`)

	f, err := seed.ParseFile(data)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(f.Schema.Name)
	fmt.Println(f.Tenant.Name)
	// Output:
	// app-config
	// acme
}

func ExampleMarshal() {
	f := &seed.File{
		SpecVersion: "v1",
		Schema: seed.SchemaDef{
			Name: "app-config",
			Fields: map[string]seed.FieldDef{
				"app.env": {Type: "string"},
			},
		},
		Tenant: seed.TenantDef{Name: "acme"},
		Config: seed.ConfigDef{
			Description: "initial setup",
			Values: map[string]seed.ConfigValueDef{
				"app.env": {Value: "production"},
			},
		},
	}

	data, err := seed.Marshal(f)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(len(data) > 0)
	// Output:
	// true
}
