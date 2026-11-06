module github.com/opendecree/decree/examples/quickstart

go 1.26.0

require github.com/opendecree/decree/sdk/grpctransport v0.12.0-alpha.5

require (
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.33.0 // indirect
	github.com/opendecree/decree/api v0.12.0-alpha.5 // indirect
	github.com/opendecree/decree/sdk/adminclient v0.12.0-alpha.5 // indirect
	github.com/opendecree/decree/sdk/configclient v0.12.0-alpha.5 // indirect
	github.com/opendecree/decree/sdk/configwatcher v0.12.0-alpha.5 // indirect
	github.com/opendecree/decree/sdk/retry v0.12.0-alpha.5 // indirect
	golang.org/x/net v0.61.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/text v0.43.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20261026165753-2ffab32bce7e // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20261019180522-ce908d29299b // indirect
	google.golang.org/grpc v1.84.0 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
)

replace github.com/opendecree/decree/api => ../../api

replace github.com/opendecree/decree/sdk/configclient => ../../sdk/configclient

replace github.com/opendecree/decree/sdk/grpctransport => ../../sdk/grpctransport

replace github.com/opendecree/decree/sdk/adminclient => ../../sdk/adminclient

replace github.com/opendecree/decree/sdk/configwatcher => ../../sdk/configwatcher

replace github.com/opendecree/decree/sdk/retry => ../../sdk/retry
