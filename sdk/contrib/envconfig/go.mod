module github.com/opendecree/decree/sdk/contrib/envconfig

go 1.22.0

require github.com/opendecree/decree/sdk/configclient v0.12.0-alpha.2

require github.com/opendecree/decree/sdk/retry v0.12.0-alpha.2 // indirect

replace github.com/opendecree/decree/sdk/configclient => ../../../sdk/configclient

replace github.com/opendecree/decree/sdk/retry => ../../../sdk/retry
