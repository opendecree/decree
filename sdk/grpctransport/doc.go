// Package grpctransport provides gRPC [Transport] implementations for the
// OpenDecree SDK clients (configclient, adminclient) and a gRPC-backed
// [configwatcher.Watcher].
//
// Convenience constructors [NewConfigClient], [NewAdminClient], and
// [NewWatcher] wire a [grpc.ClientConnInterface] directly to the high-level
// SDK types. Lower-level transport types (ConfigTransport, SchemaTransport,
// AuditTransport, ServerTransport, AdminConfigTransport) are available for
// custom wiring.
//
// This module pins Go 1.24 because google.golang.org/grpc does.
package grpctransport
