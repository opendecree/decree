package schema

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"

	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/internal/storage/domain"
)

// schemaToProto converts domain schema + version + fields to a proto Schema.
func schemaToProto(s domain.Schema, v domain.SchemaVersion, fields []domain.SchemaField) *pb.Schema {
	pbFields := make([]*pb.SchemaField, 0, len(fields))
	for _, f := range fields {
		pbFields = append(pbFields, fieldToProto(f))
	}

	result := &pb.Schema{
		Id:        s.ID,
		Name:      s.Name,
		Version:   v.Version,
		Checksum:  v.Checksum,
		Published: v.Published,
		Fields:    pbFields,
		CreatedAt: timestamppb.New(v.CreatedAt),
	}
	if s.Description != nil {
		result.Description = *s.Description
	}
	if v.ParentVersion != nil {
		result.ParentVersion = v.ParentVersion
	}
	if v.Description != nil {
		result.VersionDescription = *v.Description
	}
	return result
}

// fieldToProto converts a domain schema field to a proto SchemaField.
func fieldToProto(f domain.SchemaField) *pb.SchemaField {
	result := &pb.SchemaField{
		Path:       f.Path,
		Type:       f.FieldType.ToProto(),
		Nullable:   f.Nullable,
		Deprecated: f.Deprecated,
		Tags:       f.Tags,
		ReadOnly:   f.ReadOnly,
		WriteOnce:  f.WriteOnce,
		Sensitive:  f.Sensitive,
	}

	if f.Constraints != nil {
		var c pb.FieldConstraints
		if err := json.Unmarshal(f.Constraints, &c); err == nil {
			result.Constraints = &c
		}
	}
	if f.RedirectTo != nil {
		result.RedirectTo = f.RedirectTo
	}
	if f.DefaultValue != nil {
		result.DefaultValue = f.DefaultValue
	}
	if f.Description != nil {
		result.Description = f.Description
	}
	if f.Title != nil {
		result.Title = f.Title
	}
	if f.Example != nil {
		result.Example = f.Example
	}
	if f.Format != nil {
		result.Format = f.Format
	}
	if f.Examples != nil {
		var examples map[string]*pb.FieldExample
		if err := json.Unmarshal(f.Examples, &examples); err == nil {
			result.Examples = examples
		}
	}
	if f.ExternalDocs != nil {
		var docs pb.ExternalDocs
		if err := json.Unmarshal(f.ExternalDocs, &docs); err == nil {
			result.ExternalDocs = &docs
		}
	}
	return result
}

// tenantToProto converts a domain tenant to a proto Tenant.
func tenantToProto(t domain.Tenant) *pb.Tenant {
	return &pb.Tenant{
		Id:            t.ID,
		Name:          t.Name,
		SchemaId:      t.SchemaID,
		SchemaVersion: t.SchemaVersion,
		CreatedAt:     timestamppb.New(t.CreatedAt),
		UpdatedAt:     timestamppb.New(t.UpdatedAt),
	}
}

// fieldLockToProto converts a domain field lock to a proto FieldLock.
func fieldLockToProto(fl domain.TenantFieldLock) *pb.FieldLock {
	result := &pb.FieldLock{
		TenantId:  fl.TenantID,
		FieldPath: fl.FieldPath,
	}
	if fl.LockedValues != nil {
		var values []string
		if err := json.Unmarshal(fl.LockedValues, &values); err == nil {
			result.LockedValues = values
		}
	}
	return result
}

// computeChecksum computes a deterministic checksum for a set of schema fields.
func computeChecksum(fields []*pb.SchemaField) string {
	sorted := make([]*pb.SchemaField, len(fields))
	copy(sorted, fields)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Path < sorted[j].Path
	})

	h := sha256.New()
	for _, f := range sorted {
		_, _ = fmt.Fprintf(h, "%s:%s:%v:%v", f.Path, f.Type.String(), f.Nullable, f.Deprecated)
		if f.Constraints != nil {
			data, _ := json.Marshal(f.Constraints)
			h.Write(data)
		}
	}
	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}

var slugRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

func isValidSlug(name string) bool {
	return len(name) >= 1 && len(name) <= 63 && slugRe.MatchString(name)
}

func ptrString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
