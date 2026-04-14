package main

import (
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/zeevdr/central-config-service/api/centralconfig/v1"
	"github.com/zeevdr/central-config-service/sdk/adminclient"
	"github.com/zeevdr/central-config-service/sdk/configclient"
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

func newAdminClient(conn *grpc.ClientConn) *adminclient.Client {
	var opts []adminclient.Option
	if flagSubject != "" {
		opts = append(opts, adminclient.WithSubject(flagSubject))
	}
	if flagRole != "" {
		opts = append(opts, adminclient.WithRole(flagRole))
	}
	if flagTenantID != "" {
		opts = append(opts, adminclient.WithTenantID(flagTenantID))
	}
	if flagToken != "" {
		opts = append(opts, adminclient.WithBearerToken(flagToken))
	}
	return adminclient.New(
		pb.NewSchemaServiceClient(conn),
		pb.NewConfigServiceClient(conn),
		pb.NewAuditServiceClient(conn),
		opts...,
	)
}

func newConfigClient(conn *grpc.ClientConn) *configclient.Client {
	var opts []configclient.Option
	if flagSubject != "" {
		opts = append(opts, configclient.WithSubject(flagSubject))
	}
	if flagRole != "" {
		opts = append(opts, configclient.WithRole(flagRole))
	}
	if flagTenantID != "" {
		opts = append(opts, configclient.WithTenantID(flagTenantID))
	}
	if flagToken != "" {
		opts = append(opts, configclient.WithBearerToken(flagToken))
	}
	return configclient.New(pb.NewConfigServiceClient(conn), opts...)
}
