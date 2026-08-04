module github.com/opendecree/decree/cmd/decree

go 1.25.7

require (
	github.com/lib/pq v1.12.3
	github.com/opendecree/decree/sdk/adminclient v0.12.0-alpha.5
	github.com/opendecree/decree/sdk/configclient v0.12.0-alpha.5
	github.com/opendecree/decree/sdk/grpctransport v0.12.0-alpha.5
	github.com/opendecree/decree/sdk/tools v0.12.0-alpha.5
	github.com/pressly/goose/v3 v3.27.3
	github.com/spf13/cobra v1.10.2
	google.golang.org/grpc v1.82.1
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/cpuguy83/go-md2man/v2 v2.0.6 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.28.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/mfridman/interpolate v0.0.2 // indirect
	github.com/opendecree/decree/api v0.12.0-alpha.5 // indirect
	github.com/opendecree/decree/sdk/configwatcher v0.12.0-alpha.5 // indirect
	github.com/opendecree/decree/sdk/retry v0.12.0-alpha.5 // indirect
	github.com/rogpeppe/go-internal v1.15.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.2 // indirect
	github.com/sethvargo/go-retry v0.4.0 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/net v0.57.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.40.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260414002931-afd174a4e478 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260720211330-0afa2a65878a // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

replace github.com/opendecree/decree/api => ../../api

replace github.com/opendecree/decree/sdk/retry => ../../sdk/retry

replace github.com/opendecree/decree/sdk/adminclient => ../../sdk/adminclient

replace github.com/opendecree/decree/sdk/configclient => ../../sdk/configclient

replace github.com/opendecree/decree/sdk/grpctransport => ../../sdk/grpctransport

replace github.com/opendecree/decree/sdk/configwatcher => ../../sdk/configwatcher

replace github.com/opendecree/decree/sdk/tools => ../../sdk/tools
