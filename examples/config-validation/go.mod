module github.com/opendecree/decree/examples/config-validation

go 1.25

require github.com/opendecree/decree/sdk/tools v0.12.0-alpha.5

require (
	github.com/kr/text v0.2.0 // indirect
	github.com/rogpeppe/go-internal v1.15.0 // indirect
)

require (
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.2 // indirect
	golang.org/x/text v0.14.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/opendecree/decree/sdk/tools => ../../sdk/tools

replace github.com/opendecree/decree/api => ../../api

replace github.com/opendecree/decree/sdk/adminclient => ../../sdk/adminclient

replace github.com/opendecree/decree/sdk/retry => ../../sdk/retry
