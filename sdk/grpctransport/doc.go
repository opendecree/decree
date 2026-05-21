// Package grpctransport provides gRPC [Transport] implementations for the
// OpenDecree SDK clients (configclient, adminclient) and a gRPC-backed
// [configwatcher.Watcher].
//
// [Dial] opens a connection with TLS and system roots by default.
// Pass [WithInsecure] only for local development or testing.
//
// Convenience constructors [NewConfigClient], [NewAdminClient], and
// [NewWatcher] wire a [grpc.ClientConnInterface] directly to the high-level
// SDK types. Lower-level transport types (ConfigTransport, SchemaTransport,
// AuditTransport, ServerTransport, AdminConfigTransport) are available for
// custom wiring.
//
// This module pins Go 1.24 because google.golang.org/grpc does.
package grpctransport
