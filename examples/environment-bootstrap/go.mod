module github.com/opendecree/decree/examples/environment-bootstrap

go 1.24.0

require (
	github.com/opendecree/decree/sdk/grpctransport v0.0.0-00010101000000-000000000000
	github.com/opendecree/decree/sdk/tools v0.3.1
	google.golang.org/grpc v1.79.3
)

require (
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.28.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/opendecree/decree/api v0.3.1 // indirect
	github.com/opendecree/decree/sdk/adminclient v0.3.1 // indirect
	github.com/opendecree/decree/sdk/configclient v0.1.2 // indirect
	github.com/opendecree/decree/sdk/configwatcher v0.1.2 // indirect
	github.com/rogpeppe/go-internal v1.9.0 // indirect
	go.opentelemetry.io/otel/metric v1.41.0 // indirect
	go.opentelemetry.io/otel/sdk v1.41.0 // indirect
	go.opentelemetry.io/otel/trace v1.41.0 // indirect
	golang.org/x/net v0.49.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260209200024-4cfbd4190f57 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260209200024-4cfbd4190f57 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/opendecree/decree/api => ../../api

replace github.com/opendecree/decree/sdk/adminclient => ../../sdk/adminclient

replace github.com/opendecree/decree/sdk/tools => ../../sdk/tools

replace github.com/opendecree/decree/sdk/grpctransport => ../../sdk/grpctransport

replace github.com/opendecree/decree/sdk/configclient => ../../sdk/configclient

replace github.com/opendecree/decree/sdk/configwatcher => ../../sdk/configwatcher
