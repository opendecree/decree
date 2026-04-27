package schema

import (
	"fmt"
	"sort"
	"sync"

	"gopkg.in/yaml.v3"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
)

// Parser converts a single spec_version of decree.schema.yaml to and from
// the internal proto domain. New versions of the schema format register an
// implementation of this interface via init() in their own file under
// internal/schema/ — see parser_v1.go for the v1 implementation. The
// service layer dispatches through Registry on import and Marshal on
// export, so adding v2 support means landing a single new parser_v2.go
// file with init() and nothing else.
//
// Parse should return the proto Schema with its fields, info,
// DependentRequired, and Validations populated. Layer-2 semantic checks
// (referential integrity, prefix-overlap, range sanity) run at the
// service layer on the proto value — not inside Parse — so different
// versions can share the same semantic enforcement.
type Parser interface {
	// SpecVersion is the literal `spec_version:` value this parser handles
	// (e.g. "v1"). Used as the registry key.
	SpecVersion() string

	// Parse converts YAML bytes to a proto Schema. Returns an error if the
	// document is malformed at the layer-1 structural level (unknown keys,
	// invalid types, bad regexes).
	Parse(data []byte) (*pb.Schema, error)

	// Marshal converts a proto Schema back to YAML bytes for export. The
	// caller has already chosen which parser to dispatch to based on the
	// requested spec_version; Marshal does not re-check the value.
	Marshal(s *pb.Schema) ([]byte, error)
}

var (
	parsersMu sync.RWMutex
	parsers   = map[string]Parser{}
)

// Register adds a parser to the registry. Called from each parser's init()
// function. Panics on duplicate registration — that is a programming error
// at compile time, not a runtime input.
func Register(p Parser) {
	parsersMu.Lock()
	defer parsersMu.Unlock()
	v := p.SpecVersion()
	if _, exists := parsers[v]; exists {
		panic(fmt.Sprintf("schema: duplicate parser registered for spec_version %q", v))
	}
	parsers[v] = p
}

// SupportedVersions returns the registered spec_version values in sorted
// order. Used in error messages so users see which versions the server
// accepts.
func SupportedVersions() []string {
	parsersMu.RLock()
	defer parsersMu.RUnlock()
	out := make([]string, 0, len(parsers))
	for v := range parsers {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

// LatestVersion returns the highest registered spec_version (lexicographic
// order — sufficient while versions follow a "v1", "v2", … pattern). Used
// as the default when ExportSchema is called without an explicit version.
// Returns "" if no parsers are registered, which is a server
// misconfiguration the caller surfaces as Internal.
func LatestVersion() string {
	versions := SupportedVersions()
	if len(versions) == 0 {
		return ""
	}
	return versions[len(versions)-1]
}

// peekHeader is a minimal struct used to extract spec_version from a YAML
// document before handing the bytes to the version-specific parser. Any
// fields outside spec_version are ignored at this stage so a malformed
// document still produces a clear "unsupported spec_version" error
// before falling into the per-parser shape lint.
type peekHeader struct {
	SpecVersion string `yaml:"spec_version"`
}

// Dispatch routes an incoming schema YAML document to the parser
// registered for its spec_version. Returns an error naming the supported
// versions if spec_version is missing or unrecognized; otherwise delegates
// to the matching parser.
func Dispatch(data []byte) (*pb.Schema, error) {
	var hdr peekHeader
	if err := yaml.Unmarshal(data, &hdr); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}
	if hdr.SpecVersion == "" {
		return nil, fmt.Errorf("spec_version is required (supported: %v)", SupportedVersions())
	}
	parsersMu.RLock()
	p, ok := parsers[hdr.SpecVersion]
	parsersMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unsupported spec_version %q (supported: %v)", hdr.SpecVersion, SupportedVersions())
	}
	return p.Parse(data)
}

// MarshalSchemaAt selects the parser for the requested spec_version and
// emits the schema in that version's wire format. version may be empty —
// in which case LatestVersion is used. Returns an error if no parser is
// registered for the requested version.
func MarshalSchemaAt(s *pb.Schema, version string) ([]byte, error) {
	if version == "" {
		version = LatestVersion()
	}
	parsersMu.RLock()
	p, ok := parsers[version]
	parsersMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unsupported spec_version %q (supported: %v)", version, SupportedVersions())
	}
	return p.Marshal(s)
}
