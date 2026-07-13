module github.com/opendecree/decree/e2e

go 1.25.0

require (
	github.com/opendecree/decree/api v0.12.0-alpha.5
	github.com/opendecree/decree/sdk/adminclient v0.12.0-alpha.5
	github.com/opendecree/decree/sdk/configclient v0.12.0-alpha.5
	github.com/opendecree/decree/sdk/grpctransport v0.12.0-alpha.5
	github.com/stretchr/testify v1.11.1
	google.golang.org/grpc v1.81.1
	google.golang.org/protobuf v1.36.11
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.29.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/opendecree/decree/sdk/configwatcher v0.12.0-alpha.5 // indirect
	github.com/opendecree/decree/sdk/retry v0.12.0-alpha.5 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	golang.org/x/text v0.37.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260414002931-afd174a4e478 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260414002931-afd174a4e478 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/opendecree/decree/api => ../api

replace github.com/opendecree/decree/sdk/adminclient => ../sdk/adminclient

replace github.com/opendecree/decree/sdk/configclient => ../sdk/configclient

replace github.com/opendecree/decree/sdk/grpctransport => ../sdk/grpctransport

replace github.com/opendecree/decree/sdk/configwatcher => ../sdk/configwatcher

replace github.com/opendecree/decree/sdk/retry => ../sdk/retry
