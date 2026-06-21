module github.com/opendecree/decree/sdk/configwatcher

go 1.22.0

require github.com/opendecree/decree/sdk/configclient v0.12.0-alpha.4

require github.com/opendecree/decree/sdk/retry v0.12.0-alpha.4 // indirect

replace github.com/opendecree/decree/sdk/retry => ../retry

replace github.com/opendecree/decree/sdk/configclient => ../configclient
