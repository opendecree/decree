module github.com/opendecree/decree/sdk/tools

go 1.25.0

require (
	github.com/opendecree/decree/sdk/adminclient v0.1.2
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/kr/pretty v0.3.1 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
)

replace github.com/opendecree/decree/api => ../../api

replace github.com/opendecree/decree/sdk/adminclient => ../adminclient
