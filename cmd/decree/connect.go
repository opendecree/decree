package main

import (
	"fmt"
	"os"
	"strings"

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

// resolveToken returns the effective bearer token. --token-file takes
// precedence over --token; the file content is trimmed of surrounding whitespace.
func resolveToken() (string, error) {
	if flagTokenFile != "" {
		data, err := os.ReadFile(flagTokenFile)
		if err != nil {
			return "", fmt.Errorf("read token file: %w", err)
		}
		return strings.TrimSpace(string(data)), nil
	}
	return flagToken, nil
}

func authOptions() ([]grpctransport.Option, error) {
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
	token, err := resolveToken()
	if err != nil {
		return nil, err
	}
	if token != "" {
		opts = append(opts, grpctransport.WithBearerToken(token))
	}
	return opts, nil
}

func newAdminClient(conn *grpc.ClientConn) (*adminclient.Client, error) {
	opts, err := authOptions()
	if err != nil {
		return nil, err
	}
	return grpctransport.NewAdminClient(conn, opts...)
}

func newConfigClient(conn *grpc.ClientConn) (*configclient.Client, error) {
	opts, err := authOptions()
	if err != nil {
		return nil, err
	}
	return grpctransport.NewConfigClient(conn, opts...)
}

func newConfigTransport(conn *grpc.ClientConn) (*grpctransport.ConfigTransport, error) {
	opts, err := authOptions()
	if err != nil {
		return nil, err
	}
	return grpctransport.NewConfigTransport(conn, opts...)
}
