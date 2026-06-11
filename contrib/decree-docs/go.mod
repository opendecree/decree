module github.com/opendecree/decree/contrib/decree-docs

go 1.24.0

require github.com/spf13/cobra v1.10.2

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
)

// Intra-repo dependencies build against the working tree (pattern shared with
// sdk/contrib and cmd/decree). decree-docs grows require directives for
// sdk/tools and sdk/adminclient when the loaders land; the replaces are inert
// until then.
replace github.com/opendecree/decree/sdk/tools => ../../sdk/tools

replace github.com/opendecree/decree/sdk/adminclient => ../../sdk/adminclient

replace github.com/opendecree/decree/sdk/retry => ../../sdk/retry
