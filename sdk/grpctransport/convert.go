package grpctransport

import (
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/sdk/adminclient"
	"github.com/opendecree/decree/sdk/configclient"
)

// --- TypedValue conversion ---

func typedValueFromProto(tv *pb.TypedValue) *configclient.TypedValue {
	if tv == nil {
		return nil
	}
	switch v := tv.Kind.(type) {
	case *pb.TypedValue_IntegerValue:
		return configclient.IntVal(v.IntegerValue)
	case *pb.TypedValue_NumberValue:
		return configclient.FloatVal(v.NumberValue)
	case *pb.TypedValue_StringValue:
		return configclient.StringVal(v.StringValue)
	case *pb.TypedValue_BoolValue:
		return configclient.BoolVal(v.BoolValue)
	case *pb.TypedValue_TimeValue:
		if v.TimeValue != nil {
			return configclient.TimeVal(v.TimeValue.AsTime())
		}
		return nil
	case *pb.TypedValue_DurationValue:
		if v.DurationValue != nil {
			return configclient.DurationVal(v.DurationValue.AsDuration())
		}
		return nil
	case *pb.TypedValue_UrlValue:
		return configclient.URLVal(v.UrlValue)
	case *pb.TypedValue_JsonValue:
		return configclient.JSONVal(v.JsonValue)
	default:
		return nil
	}
}

func typedValueToProto(tv *configclient.TypedValue) *pb.TypedValue {
	if tv == nil {
		return nil
	}
	switch tv.Kind() {
	case configclient.KindInteger:
		return &pb.TypedValue{Kind: &pb.TypedValue_IntegerValue{IntegerValue: tv.MustIntValue()}}
	case configclient.KindNumber:
		return &pb.TypedValue{Kind: &pb.TypedValue_NumberValue{NumberValue: tv.MustFloatValue()}}
	case configclient.KindString:
		return &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: tv.MustStringValue()}}
	case configclient.KindBool:
		return &pb.TypedValue{Kind: &pb.TypedValue_BoolValue{BoolValue: tv.MustBoolValue()}}
	case configclient.KindTime:
		return &pb.TypedValue{Kind: &pb.TypedValue_TimeValue{TimeValue: timestamppb.New(tv.MustTimeValue())}}
	case configclient.KindDuration:
		return &pb.TypedValue{Kind: &pb.TypedValue_DurationValue{DurationValue: durationpb.New(tv.MustDurationValue())}}
	case configclient.KindURL:
		return &pb.TypedValue{Kind: &pb.TypedValue_UrlValue{UrlValue: tv.MustURLValue()}}
	case configclient.KindJSON:
		return &pb.TypedValue{Kind: &pb.TypedValue_JsonValue{JsonValue: tv.MustJSONValue()}}
	default:
		return nil
	}
}

// --- ConfigValue conversion ---

// configValueFromProto converts a proto ConfigValue to the SDK type.
func configValueFromProto(cv *pb.ConfigValue) configclient.ConfigValue {
	return configclient.ConfigValue{
		FieldPath:   cv.GetFieldPath(),
		Value:       typedValueFromProto(cv.GetValue()),
		Checksum:    cv.GetChecksum(),
		Description: cv.GetDescription(),
	}
}

// configVersionFromProto converts a proto ConfigVersion to the SDK configclient
// type. Returns nil when the input is nil so that absent versions stay absent.
func configVersionFromProto(v *pb.ConfigVersion) *configclient.ConfigVersion {
	if v == nil {
		return nil
	}
	result := &configclient.ConfigVersion{
		ID:          v.GetId(),
		TenantID:    v.GetTenantId(),
		Version:     v.GetVersion(),
		Description: v.GetDescription(),
		CreatedBy:   v.GetCreatedBy(),
	}
	if v.GetCreatedAt() != nil {
		result.CreatedAt = v.GetCreatedAt().AsTime()
	}
	return result
}

// --- Schema/Field conversion ---

func schemaFromProto(s *pb.Schema) *adminclient.Schema {
	if s == nil {
		return nil
	}
	fields := make([]adminclient.Field, len(s.GetFields()))
	for i, f := range s.GetFields() {
		fields[i] = fieldFromProto(f)
	}
	result := &adminclient.Schema{
		ID:                 s.GetId(),
		Name:               s.GetName(),
		Description:        s.GetDescription(),
		Version:            s.GetVersion(),
		VersionDescription: s.GetVersionDescription(),
		Checksum:           s.GetChecksum(),
		Published:          s.GetPublished(),
		Fields:             fields,
		Info:               schemaInfoFromProto(s.GetInfo()),
	}
	if s.ParentVersion != nil {
		pv := s.GetParentVersion()
		result.ParentVersion = &pv
	}
	if s.GetCreatedAt() != nil {
		result.CreatedAt = s.GetCreatedAt().AsTime()
	}
	return result
}

func schemaInfoFromProto(info *pb.SchemaInfo) *adminclient.SchemaInfo {
	if info == nil {
		return nil
	}
	result := &adminclient.SchemaInfo{
		Title:  info.GetTitle(),
		Author: info.GetAuthor(),
		Labels: info.GetLabels(),
	}
	if info.GetContact() != nil {
		result.Contact = &adminclient.SchemaContact{
			Name:  info.GetContact().GetName(),
			Email: info.GetContact().GetEmail(),
			URL:   info.GetContact().GetUrl(),
		}
	}
	return result
}

func fieldFromProto(f *pb.SchemaField) adminclient.Field {
	if f == nil {
		return adminclient.Field{}
	}
	field := adminclient.Field{
		Path:       f.GetPath(),
		Type:       fieldTypeFromProto(f.GetType()),
		Nullable:   f.GetNullable(),
		Deprecated: f.GetDeprecated(),
		Tags:       f.GetTags(),
		ReadOnly:   f.GetReadOnly(),
		WriteOnce:  f.GetWriteOnce(),
		Sensitive:  f.GetSensitive(),
	}
	if f.RedirectTo != nil {
		field.RedirectTo = f.GetRedirectTo()
	}
	if f.DefaultValue != nil {
		field.Default = f.GetDefaultValue()
	}
	if f.Description != nil {
		field.Description = f.GetDescription()
	}
	if f.Title != nil {
		field.Title = f.GetTitle()
	}
	if f.Example != nil {
		field.Example = f.GetExample()
	}
	if f.Format != nil {
		field.Format = f.GetFormat()
	}
	if f.GetConstraints() != nil {
		field.Constraints = constraintsFromProto(f.GetConstraints())
	}
	if len(f.GetExamples()) > 0 {
		field.Examples = make(map[string]adminclient.FieldExample, len(f.GetExamples()))
		for k, v := range f.GetExamples() {
			field.Examples[k] = adminclient.FieldExample{
				Value:   v.GetValue(),
				Summary: v.GetSummary(),
			}
		}
	}
	if f.GetExternalDocs() != nil {
		field.ExternalDocs = &adminclient.ExternalDocs{
			Description: f.GetExternalDocs().GetDescription(),
			URL:         f.GetExternalDocs().GetUrl(),
		}
	}
	return field
}

func constraintsFromProto(c *pb.FieldConstraints) *adminclient.FieldConstraints {
	if c == nil {
		return nil
	}
	fc := &adminclient.FieldConstraints{
		Enum: c.GetEnumValues(),
	}
	if c.Min != nil {
		v := c.GetMin()
		fc.Min = &v
	}
	if c.Max != nil {
		v := c.GetMax()
		fc.Max = &v
	}
	if c.ExclusiveMin != nil {
		v := c.GetExclusiveMin()
		fc.ExclusiveMin = &v
	}
	if c.ExclusiveMax != nil {
		v := c.GetExclusiveMax()
		fc.ExclusiveMax = &v
	}
	if c.MinLength != nil {
		v := c.GetMinLength()
		fc.MinLength = &v
	}
	if c.MaxLength != nil {
		v := c.GetMaxLength()
		fc.MaxLength = &v
	}
	if c.Regex != nil {
		fc.Pattern = c.GetRegex()
	}
	if c.JsonSchema != nil {
		fc.JSONSchema = c.GetJsonSchema()
	}
	return fc
}

func fieldsToProto(fields []adminclient.Field) []*pb.SchemaField {
	result := make([]*pb.SchemaField, len(fields))
	for i, f := range fields {
		sf := &pb.SchemaField{
			Path:       f.Path,
			Type:       fieldTypeToProto(f.Type),
			Nullable:   f.Nullable,
			Deprecated: f.Deprecated,
			Tags:       f.Tags,
			ReadOnly:   f.ReadOnly,
			WriteOnce:  f.WriteOnce,
			Sensitive:  f.Sensitive,
		}
		if f.RedirectTo != "" {
			sf.RedirectTo = &f.RedirectTo
		}
		if f.Default != "" {
			sf.DefaultValue = &f.Default
		}
		if f.Description != "" {
			sf.Description = &f.Description
		}
		if f.Title != "" {
			sf.Title = &f.Title
		}
		if f.Example != "" {
			sf.Example = &f.Example
		}
		if f.Format != "" {
			sf.Format = &f.Format
		}
		if f.Constraints != nil {
			sf.Constraints = constraintsToProto(f.Constraints)
		}
		if len(f.Examples) > 0 {
			sf.Examples = make(map[string]*pb.FieldExample, len(f.Examples))
			for k, v := range f.Examples {
				sf.Examples[k] = &pb.FieldExample{
					Value:   v.Value,
					Summary: v.Summary,
				}
			}
		}
		if f.ExternalDocs != nil {
			sf.ExternalDocs = &pb.ExternalDocs{
				Description: f.ExternalDocs.Description,
				Url:         f.ExternalDocs.URL,
			}
		}
		result[i] = sf
	}
	return result
}

func constraintsToProto(c *adminclient.FieldConstraints) *pb.FieldConstraints {
	if c == nil {
		return nil
	}
	fc := &pb.FieldConstraints{
		EnumValues: c.Enum,
	}
	if c.Min != nil {
		fc.Min = c.Min
	}
	if c.Max != nil {
		fc.Max = c.Max
	}
	if c.ExclusiveMin != nil {
		fc.ExclusiveMin = c.ExclusiveMin
	}
	if c.ExclusiveMax != nil {
		fc.ExclusiveMax = c.ExclusiveMax
	}
	if c.MinLength != nil {
		fc.MinLength = c.MinLength
	}
	if c.MaxLength != nil {
		fc.MaxLength = c.MaxLength
	}
	if c.Pattern != "" {
		fc.Regex = &c.Pattern
	}
	if c.JSONSchema != "" {
		fc.JsonSchema = &c.JSONSchema
	}
	return fc
}

// --- Tenant conversion ---

func tenantFromProto(t *pb.Tenant) *adminclient.Tenant {
	if t == nil {
		return nil
	}
	result := &adminclient.Tenant{
		ID:            t.GetId(),
		Name:          t.GetName(),
		SchemaID:      t.GetSchemaId(),
		SchemaVersion: t.GetSchemaVersion(),
	}
	if t.GetCreatedAt() != nil {
		result.CreatedAt = t.GetCreatedAt().AsTime()
	}
	if t.GetUpdatedAt() != nil {
		result.UpdatedAt = t.GetUpdatedAt().AsTime()
	}
	return result
}

// --- AuditEntry conversion ---

func auditEntryFromProto(e *pb.AuditEntry) *adminclient.AuditEntry {
	if e == nil {
		return nil
	}
	result := &adminclient.AuditEntry{
		ID:         e.GetId(),
		TenantID:   e.GetTenantId(),
		Actor:      e.GetActor(),
		Action:     e.GetAction(),
		ChainEpoch: e.GetChainEpoch(),
	}
	if e.FieldPath != nil {
		result.FieldPath = e.GetFieldPath()
	}
	if e.OldValue != nil {
		result.OldValue = e.GetOldValue()
	}
	if e.NewValue != nil {
		result.NewValue = e.GetNewValue()
	}
	if e.ConfigVersion != nil {
		v := e.GetConfigVersion()
		result.ConfigVersion = &v
	}
	if e.GetCreatedAt() != nil {
		result.CreatedAt = e.GetCreatedAt().AsTime()
	}
	result.ObjectKind = e.GetObjectKind()
	result.EntryHash = e.GetEntryHash()
	result.PreviousHash = e.GetPreviousHash()
	if len(e.GetMetadata()) > 0 {
		result.Metadata = e.GetMetadata()
	}
	return result
}

// --- UsageStats conversion ---

func usageStatsFromProto(s *pb.UsageStats) *adminclient.UsageStats {
	if s == nil {
		return nil
	}
	result := &adminclient.UsageStats{
		TenantID:  s.GetTenantId(),
		FieldPath: s.GetFieldPath(),
		ReadCount: s.GetReadCount(),
	}
	if s.LastReadBy != nil {
		result.LastReadBy = s.GetLastReadBy()
	}
	if s.GetLastReadAt() != nil {
		t := s.GetLastReadAt().AsTime()
		result.LastReadAt = &t
	}
	return result
}

// --- Version conversion ---

func versionFromProto(v *pb.ConfigVersion) *adminclient.Version {
	if v == nil {
		return nil
	}
	result := &adminclient.Version{
		ID:          v.GetId(),
		TenantID:    v.GetTenantId(),
		Version:     v.GetVersion(),
		Description: v.GetDescription(),
		CreatedBy:   v.GetCreatedBy(),
	}
	if v.GetCreatedAt() != nil {
		result.CreatedAt = v.GetCreatedAt().AsTime()
	}
	return result
}

// --- Helpers ---

func timeToProto(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}

func fieldTypeFromProto(ft pb.FieldType) adminclient.FieldType {
	switch ft {
	case pb.FieldType_FIELD_TYPE_INT:
		return adminclient.FieldTypeInteger
	case pb.FieldType_FIELD_TYPE_STRING:
		return adminclient.FieldTypeString
	case pb.FieldType_FIELD_TYPE_TIME:
		return adminclient.FieldTypeTime
	case pb.FieldType_FIELD_TYPE_DURATION:
		return adminclient.FieldTypeDuration
	case pb.FieldType_FIELD_TYPE_URL:
		return adminclient.FieldTypeURL
	case pb.FieldType_FIELD_TYPE_JSON:
		return adminclient.FieldTypeJSON
	case pb.FieldType_FIELD_TYPE_NUMBER:
		return adminclient.FieldTypeNumber
	case pb.FieldType_FIELD_TYPE_BOOL:
		return adminclient.FieldTypeBool
	default:
		return ""
	}
}

func fieldTypeToProto(ft adminclient.FieldType) pb.FieldType {
	switch ft {
	case adminclient.FieldTypeInteger:
		return pb.FieldType_FIELD_TYPE_INT
	case adminclient.FieldTypeString:
		return pb.FieldType_FIELD_TYPE_STRING
	case adminclient.FieldTypeTime:
		return pb.FieldType_FIELD_TYPE_TIME
	case adminclient.FieldTypeDuration:
		return pb.FieldType_FIELD_TYPE_DURATION
	case adminclient.FieldTypeURL:
		return pb.FieldType_FIELD_TYPE_URL
	case adminclient.FieldTypeJSON:
		return pb.FieldType_FIELD_TYPE_JSON
	case adminclient.FieldTypeNumber:
		return pb.FieldType_FIELD_TYPE_NUMBER
	case adminclient.FieldTypeBool:
		return pb.FieldType_FIELD_TYPE_BOOL
	default:
		return pb.FieldType_FIELD_TYPE_UNSPECIFIED
	}
}
