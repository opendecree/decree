package dump_test

import (
	"fmt"

	"github.com/opendecree/decree/sdk/tools/dump"
	"github.com/opendecree/decree/sdk/tools/seed"
)

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
			Description: "snapshot",
			Values: map[string]seed.ConfigValueDef{
				"app.env": {Value: "production"},
			},
		},
	}

	data, err := dump.Marshal(f)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(len(data) > 0)
	// Output:
	// true
}
