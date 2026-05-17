//go:build e2e

package e2e

// TestRPCMatrixCoverage enumerates every service method from the proto file
// descriptors and fails if any non-bypassed RPC is absent from the
// role×action matrix declared in role_action_matrix_test.go.
//
// When you add a new RPC to the proto:
//  1. Add an rpcSpec entry to allRPCs() in role_action_matrix_test.go.
//  2. If the RPC intentionally skips authentication (e.g. health checks,
//     unauthenticated discovery endpoints), add its full path to
//     matrixBypassList below instead of to allRPCs().
//
// The bypass list uses the gRPC full method path format:
//
//	/package.ServiceName/MethodName

import (
	"testing"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// matrixBypassList contains RPCs that intentionally skip authentication and
// must NOT appear in the role×action matrix. Add an entry here (not to
// allRPCs) whenever a new unauthenticated endpoint is introduced.
//
// Format: "/package.ServiceName/MethodName"
var matrixBypassList = map[string]string{
	// ServerService.GetServerInfo skips auth — see internal/auth skipAuth.
	"/centralconfig.v1.ServerService/GetServerInfo": "no-auth endpoint (server metadata discovery)",
}

// protoFileDescriptors is the set of proto file descriptors to scan.
// Add a new entry here whenever a new .proto file with service definitions
// is introduced.
var protoFileDescriptors = []protoreflect.FileDescriptor{
	pb.File_centralconfig_v1_schema_service_proto,
	pb.File_centralconfig_v1_config_service_proto,
	pb.File_centralconfig_v1_audit_service_proto,
	pb.File_centralconfig_v1_server_service_proto,
}

func TestRPCMatrixCoverage(t *testing.T) {
	// Build a set of RPC names already covered by the matrix.
	covered := make(map[string]bool, len(allRPCs()))
	for _, spec := range allRPCs() {
		covered[spec.name] = true
	}

	// Walk every service method in every proto file descriptor and verify
	// it is either in the matrix or explicitly bypassed.
	for _, fd := range protoFileDescriptors {
		services := fd.Services()
		for i := 0; i < services.Len(); i++ {
			svc := services.Get(i)
			methods := svc.Methods()
			for j := 0; j < methods.Len(); j++ {
				method := methods.Get(j)
				fullPath := "/" + string(svc.FullName()) + "/" + string(method.Name())
				methodName := string(method.Name())

				if _, bypassed := matrixBypassList[fullPath]; bypassed {
					// Explicitly excluded from auth coverage — skip.
					continue
				}

				if !covered[methodName] {
					t.Errorf(
						"RPC %s is not in the role×action matrix and is not bypassed.\n"+
							"Add an rpcSpec entry to allRPCs() in role_action_matrix_test.go,\n"+
							"or add %q to matrixBypassList in rpc_matrix_coverage_test.go\n"+
							"if this RPC intentionally skips authentication.",
						fullPath, fullPath,
					)
				}
			}
		}
	}
}
