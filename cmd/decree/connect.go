package main

import (
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/opendecree/decree/sdk/adminclient"
	"github.com/opendecree/decree/sdk/configclient"
	"github.com/opendecree/decree/sdk/grpctransport"
)

func dialServer() (*grpc.ClientConn, error) {
	var opts []grpc.DialOption
	if flagInsecure {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	conn, err := grpc.NewClient(flagServer, opts...)
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

func newAdminClient(conn *grpc.ClientConn) *adminclient.Client {
	return grpctransport.NewAdminClient(conn, authOptions()...)
}

func newConfigClient(conn *grpc.ClientConn) *configclient.Client {
	return grpctransport.NewConfigClient(conn, authOptions()...)
}
