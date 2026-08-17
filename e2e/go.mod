module github.com/opendecree/decree/e2e

go 1.25.0

require (
	github.com/opendecree/decree/api v0.12.0-alpha.5
	github.com/opendecree/decree/sdk/adminclient v0.12.0-alpha.5
	github.com/opendecree/decree/sdk/configclient v0.12.0-alpha.5
	github.com/opendecree/decree/sdk/grpctransport v0.12.0-alpha.5
	github.com/stretchr/testify v1.11.1
	google.golang.org/grpc v1.83.0
	google.golang.org/protobuf v1.36.12
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.30.0 // indirect
	github.com/opendecree/decree/sdk/configwatcher v0.12.0-alpha.5 // indirect
	github.com/opendecree/decree/sdk/retry v0.12.0-alpha.5 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/net v0.57.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.40.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260803160001-6ac0973c030d // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260803160001-6ac0973c030d // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/opendecree/decree/api => ../api

replace github.com/opendecree/decree/sdk/adminclient => ../sdk/adminclient

replace github.com/opendecree/decree/sdk/configclient => ../sdk/configclient

replace github.com/opendecree/decree/sdk/grpctransport => ../sdk/grpctransport

replace github.com/opendecree/decree/sdk/configwatcher => ../sdk/configwatcher

replace github.com/opendecree/decree/sdk/retry => ../sdk/retry
