package main

import (
	"fmt"
	"os"

	"google.golang.org/grpc"

	"github.com/opendecree/decree/sdk/adminclient"
	"github.com/opendecree/decree/sdk/configclient"
	"github.com/opendecree/decree/sdk/grpctransport"
)

func dialServer() (*grpc.ClientConn, error) {
	var opts []grpctransport.DialOption
	if flagInsecure {
		fmt.Fprintln(os.Stderr, "Warning: --insecure flag is set; connection is not encrypted")
		opts = append(opts, grpctransport.WithInsecure())
	}
	conn, err := grpctransport.Dial(flagServer, opts...)
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", flagServer, err)
	}
	return conn, nil
}

func authOptions() []grpctransport.Option {
	var opts []grpctransport.Option
	if flagSubject != "" {
		opts = append(opts, grpctransport.WithSubject(flagSubject))
	}
	if flagRole != "" {
		opts = append(opts, grpctransport.WithRole(flagRole))
	}
	if flagTenantID != "" {
		opts = append(opts, grpctransport.WithTenantID(flagTenantID))
	}
	if flagToken != "" {
		opts = append(opts, grpctransport.WithBearerToken(flagToken))
	}
	return opts
}

func newAdminClient(conn *grpc.ClientConn) (*adminclient.Client, error) {
	return grpctransport.NewAdminClient(conn, authOptions()...)
}

func newConfigClient(conn *grpc.ClientConn) (*configclient.Client, error) {
	return grpctransport.NewConfigClient(conn, authOptions()...)
}

func newConfigTransport(conn *grpc.ClientConn) (*grpctransport.ConfigTransport, error) {
	return grpctransport.NewConfigTransport(conn, authOptions()...)
}
