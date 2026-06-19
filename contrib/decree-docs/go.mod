module github.com/opendecree/decree/contrib/decree-docs

go 1.24.0

require (
	github.com/opendecree/decree/sdk/adminclient v0.12.0-alpha.3
	github.com/opendecree/decree/sdk/tools v0.12.0-alpha.3
	github.com/spf13/cobra v1.10.2
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/opendecree/decree/sdk/retry v0.12.0-alpha.3 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.2 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	golang.org/x/text v0.14.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

// Intra-repo dependencies build against the working tree (pattern shared with
// sdk/contrib and cmd/decree).
replace github.com/opendecree/decree/sdk/tools => ../../sdk/tools

replace github.com/opendecree/decree/sdk/adminclient => ../../sdk/adminclient

replace github.com/opendecree/decree/sdk/retry => ../../sdk/retry
