module github.com/opendecree/decree/examples/config-validation

go 1.24.0

require github.com/opendecree/decree/sdk/tools v0.3.1

require (
	github.com/kr/text v0.2.0 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/opendecree/decree/sdk/tools => ../../sdk/tools

replace github.com/opendecree/decree/api => ../../api

replace github.com/opendecree/decree/sdk/adminclient => ../../sdk/adminclient
