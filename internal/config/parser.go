package config

import (
	"fmt"
	"sort"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/opendecree/decree/internal/storage/domain"
)

// Parser converts a single spec_version of decree.config.yaml to and from
// the internal interchange shape. New versions of the config format
// register an implementation of this interface via init() in their own
// file under internal/config/ — see parser_v1.go for the v1
// implementation. The service layer dispatches through DispatchImport
// on import and MarshalConfigAt on export, so adding v2 support means
// landing a single new parser_v2.go file with init() and nothing else.
//
// Parser.Parse takes the schema's field types as input so it can coerce
// YAML-native primitives (numbers, bools, durations) to the canonical
// string representation the storage layer expects. Layer-2 semantic
// checks (field locks, per-field validation, dependentRequired) run at
// the service layer on the parser's output — not inside Parse — so
// different versions can share the same enforcement.
type Parser interface {
	// SpecVersion is the literal `spec_version:` value this parser handles
	// (e.g. "v1"). Used as the registry key.
	SpecVersion() string

	// Parse converts YAML bytes to a parsed import. fieldTypes maps each
	// field path to its declared schema type so the parser can coerce
	// typed YAML primitives to the canonical string representation.
	// Returns an error on malformed YAML or type-mismatch between a YAML
	// value and its declared field type.
	Parse(data []byte, fieldTypes map[string]domain.FieldType) (ParsedImport, error)

	// Marshal converts a list of stored config rows back to YAML bytes for
	// export. fieldTypes is needed so the parser can render strings as
	// their typed YAML form.
	Marshal(version int32, description string, rows []configRow, fieldTypes map[string]domain.FieldType) ([]byte, error)
}

// ParsedImport carries everything the service layer needs from a parsed
// import: the values to write, plus the document's top-level description
// (used as the audit description if the request did not supply one).
// Future fields — e.g. an explicit `version` declared by the YAML — slot
// in here without churning the Parser interface signature.
type ParsedImport struct {
	Description string
	Values      []configValueImport
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
		panic(fmt.Sprintf("config: duplicate parser registered for spec_version %q", v))
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
// as the default when ExportConfig is called without an explicit version.
func LatestVersion() string {
	versions := SupportedVersions()
	if len(versions) == 0 {
		return ""
	}
	return versions[len(versions)-1]
}

// peekHeader is a minimal struct used to extract spec_version from a YAML
// document before handing the bytes to the version-specific parser.
type peekHeader struct {
	SpecVersion string `yaml:"spec_version"`
}

// DispatchImport routes an incoming config YAML document to the parser
// registered for its spec_version. Returns an error naming the supported
// versions if spec_version is missing or unrecognized; otherwise delegates
// to the matching parser.
func DispatchImport(data []byte, fieldTypes map[string]domain.FieldType) (ParsedImport, error) {
	var hdr peekHeader
	if err := yaml.Unmarshal(data, &hdr); err != nil {
		return ParsedImport{}, fmt.Errorf("invalid YAML: %w", err)
	}
	if hdr.SpecVersion == "" {
		return ParsedImport{}, fmt.Errorf("spec_version is required (supported: %v)", SupportedVersions())
	}
	parsersMu.RLock()
	p, ok := parsers[hdr.SpecVersion]
	parsersMu.RUnlock()
	if !ok {
		return ParsedImport{}, fmt.Errorf("unsupported spec_version %q (supported: %v)", hdr.SpecVersion, SupportedVersions())
	}
	return p.Parse(data, fieldTypes)
}

// MarshalConfigAt selects the parser for the requested spec_version and
// emits the config in that version's wire format. version may be empty —
// in which case LatestVersion is used. Returns an error if no parser is
// registered for the requested version.
func MarshalConfigAt(version int32, description string, rows []configRow, fieldTypes map[string]domain.FieldType, specVersion string) ([]byte, error) {
	if specVersion == "" {
		specVersion = LatestVersion()
	}
	parsersMu.RLock()
	p, ok := parsers[specVersion]
	parsersMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unsupported spec_version %q (supported: %v)", specVersion, SupportedVersions())
	}
	return p.Marshal(version, description, rows, fieldTypes)
}
