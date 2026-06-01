module github.com/opendecree/decree/sdk/configwatcher

go 1.22.0

require github.com/opendecree/decree/sdk/configclient v0.1.2

require github.com/opendecree/decree/sdk/retry v0.0.0 // indirect

replace github.com/opendecree/decree/sdk/retry => ../retry

replace github.com/opendecree/decree/sdk/configclient => ../configclient
