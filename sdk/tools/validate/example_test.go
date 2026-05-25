package validate_test

import (
	"fmt"

	"github.com/opendecree/decree/sdk/tools/validate"
)

func ExampleValidate() {
	schemaYAML := []byte(`spec_version: "v1"
name: app-config
fields:
  app.env:
    type: string
  app.port:
    type: integer
`)
	configYAML := []byte(`spec_version: "v1"
values:
  app.env:
    value: production
  app.port:
    value: 8080
`)

	result, err := validate.Validate(schemaYAML, configYAML)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(result.IsValid())
	// Output:
	// true
}

func ExampleValidate_violations() {
	schemaYAML := []byte(`spec_version: "v1"
name: app-config
fields:
  app.port:
    type: integer
    constraints:
      minimum: 1024
      maximum: 65535
`)
	configYAML := []byte(`spec_version: "v1"
values:
  app.port:
    value: 80
`)

	result, err := validate.Validate(schemaYAML, configYAML)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(result.IsValid())
	fmt.Println(len(result.Violations) > 0)
	// Output:
	// false
	// true
}
