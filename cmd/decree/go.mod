module github.com/opendecree/decree/cmd/decree

go 1.25.0

require (
	github.com/opendecree/decree/api v0.1.2
	github.com/opendecree/decree/sdk/adminclient v0.1.2
	github.com/opendecree/decree/sdk/configclient v0.1.2
	github.com/opendecree/decree/sdk/grpctransport v0.1.0
	github.com/opendecree/decree/sdk/tools v0.1.0
	github.com/spf13/cobra v1.10.2
	google.golang.org/grpc v1.79.3
	google.golang.org/protobuf v1.36.11
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/cpuguy83/go-md2man/v2 v2.0.6 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.28.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/opendecree/decree/sdk/configwatcher v0.1.2 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/net v0.49.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260209200024-4cfbd4190f57 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260209200024-4cfbd4190f57 // indirect
)

replace github.com/opendecree/decree/api => ../../api

replace github.com/opendecree/decree/sdk/adminclient => ../../sdk/adminclient

replace github.com/opendecree/decree/sdk/configclient => ../../sdk/configclient

replace github.com/opendecree/decree/sdk/grpctransport => ../../sdk/grpctransport

replace github.com/opendecree/decree/sdk/configwatcher => ../../sdk/configwatcher

replace github.com/opendecree/decree/sdk/tools => ../../sdk/tools
