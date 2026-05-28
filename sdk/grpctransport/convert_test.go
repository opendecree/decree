package grpctransport

import (
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/sdk/adminclient"
	"github.com/opendecree/decree/sdk/configclient"
)

func ptrF64(f float64) *float64 { return &f }
func ptrI32(i int32) *int32     { return &i }

// --- TypedValue round-trips ---

func TestTypedValueRoundTrip_Integer(t *testing.T) {
	in := configclient.IntVal(42)
	proto := typedValueToProto(in)
	out := typedValueFromProto(proto)
	if out == nil || out.Kind() != configclient.KindInteger || out.MustIntValue() != 42 {
		t.Errorf("integer round-trip: got %v", out)
	}
}

func TestTypedValueRoundTrip_Number(t *testing.T) {
	in := configclient.FloatVal(3.14)
	proto := typedValueToProto(in)
	out := typedValueFromProto(proto)
	if out == nil || out.Kind() != configclient.KindNumber || out.MustFloatValue() != 3.14 {
		t.Errorf("number round-trip: got %v", out)
	}
}

func TestTypedValueRoundTrip_String(t *testing.T) {
	in := configclient.StringVal("hello")
	proto := typedValueToProto(in)
	out := typedValueFromProto(proto)
	if out == nil || out.Kind() != configclient.KindString || out.MustStringValue() != "hello" {
		t.Errorf("string round-trip: got %v", out)
	}
}

func TestTypedValueRoundTrip_Bool(t *testing.T) {
	in := configclient.BoolVal(true)
	proto := typedValueToProto(in)
	out := typedValueFromProto(proto)
	if out == nil || out.Kind() != configclient.KindBool || !out.MustBoolValue() {
		t.Errorf("bool round-trip: got %v", out)
	}
}

func TestTypedValueRoundTrip_Time(t *testing.T) {
	ts := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	in := configclient.TimeVal(ts)
	proto := typedValueToProto(in)
	out := typedValueFromProto(proto)
	if out == nil || out.Kind() != configclient.KindTime || !out.MustTimeValue().Equal(ts) {
		t.Errorf("time round-trip: got %v", out)
	}
}

func TestTypedValueRoundTrip_Duration(t *testing.T) {
	d := 5 * time.Second
	in := configclient.DurationVal(d)
	proto := typedValueToProto(in)
	out := typedValueFromProto(proto)
	if out == nil || out.Kind() != configclient.KindDuration || out.MustDurationValue() != d {
		t.Errorf("duration round-trip: got %v", out)
	}
}

func TestTypedValueRoundTrip_URL(t *testing.T) {
	in := configclient.URLVal("https://example.com")
	proto := typedValueToProto(in)
	out := typedValueFromProto(proto)
	if out == nil || out.Kind() != configclient.KindURL || out.MustURLValue() != "https://example.com" {
		t.Errorf("url round-trip: got %v", out)
	}
}

func TestTypedValueRoundTrip_JSON(t *testing.T) {
	in := configclient.JSONVal(`{"key":"val"}`)
	proto := typedValueToProto(in)
	out := typedValueFromProto(proto)
	if out == nil || out.Kind() != configclient.KindJSON || out.MustJSONValue() != `{"key":"val"}` {
		t.Errorf("json round-trip: got %v", out)
	}
}

func TestTypedValueFromProto_Nil(t *testing.T) {
	if typedValueFromProto(nil) != nil {
		t.Error("typedValueFromProto(nil) must return nil")
	}
}

func TestTypedValueToProto_Nil(t *testing.T) {
	if typedValueToProto(nil) != nil {
		t.Error("typedValueToProto(nil) must return nil")
	}
}

func TestTypedValueFromProto_NilTimeValue(t *testing.T) {
	proto := &pb.TypedValue{Kind: &pb.TypedValue_TimeValue{TimeValue: nil}}
	if typedValueFromProto(proto) != nil {
		t.Error("nil TimeValue field must return nil TypedValue")
	}
}

func TestTypedValueFromProto_NilDurationValue(t *testing.T) {
	proto := &pb.TypedValue{Kind: &pb.TypedValue_DurationValue{DurationValue: nil}}
	if typedValueFromProto(proto) != nil {
		t.Error("nil DurationValue field must return nil TypedValue")
	}
}

func TestTypedValueFromProto_UnknownKind(t *testing.T) {
	if typedValueFromProto(&pb.TypedValue{}) != nil {
		t.Error("TypedValue with nil kind must return nil")
	}
}

func TestTypedValueFromProto_TimeValue_WithTimestamp(t *testing.T) {
	ts := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	proto := &pb.TypedValue{Kind: &pb.TypedValue_TimeValue{TimeValue: timestamppb.New(ts)}}
	out := typedValueFromProto(proto)
	if out == nil || !out.MustTimeValue().Equal(ts) {
		t.Errorf("got %v, want %v", out, ts)
	}
}

func TestTypedValueFromProto_DurationValue_WithDuration(t *testing.T) {
	d := 2*time.Minute + 30*time.Second
	proto := &pb.TypedValue{Kind: &pb.TypedValue_DurationValue{DurationValue: durationpb.New(d)}}
	out := typedValueFromProto(proto)
	if out == nil || out.MustDurationValue() != d {
		t.Errorf("got %v, want %v", out, d)
	}
}

// --- fieldTypeFromProto ---

func TestFieldTypeFromProto(t *testing.T) {
	cases := []struct {
		ft   pb.FieldType
		want adminclient.FieldType
	}{
		{pb.FieldType_FIELD_TYPE_INT, adminclient.FieldTypeInteger},
		{pb.FieldType_FIELD_TYPE_STRING, adminclient.FieldTypeString},
		{pb.FieldType_FIELD_TYPE_TIME, adminclient.FieldTypeTime},
		{pb.FieldType_FIELD_TYPE_DURATION, adminclient.FieldTypeDuration},
		{pb.FieldType_FIELD_TYPE_URL, adminclient.FieldTypeURL},
		{pb.FieldType_FIELD_TYPE_JSON, adminclient.FieldTypeJSON},
		{pb.FieldType_FIELD_TYPE_NUMBER, adminclient.FieldTypeNumber},
		{pb.FieldType_FIELD_TYPE_BOOL, adminclient.FieldTypeBool},
		{pb.FieldType_FIELD_TYPE_UNSPECIFIED, ""},
	}
	for _, c := range cases {
		if got := fieldTypeFromProto(c.ft); got != c.want {
			t.Errorf("fieldTypeFromProto(%v) = %q, want %q", c.ft, got, c.want)
		}
	}
}

// --- fieldTypeToProto ---

func TestFieldTypeToProto(t *testing.T) {
	cases := []struct {
		ft   adminclient.FieldType
		want pb.FieldType
	}{
		{adminclient.FieldTypeInteger, pb.FieldType_FIELD_TYPE_INT},
		{adminclient.FieldTypeString, pb.FieldType_FIELD_TYPE_STRING},
		{adminclient.FieldTypeTime, pb.FieldType_FIELD_TYPE_TIME},
		{adminclient.FieldTypeDuration, pb.FieldType_FIELD_TYPE_DURATION},
		{adminclient.FieldTypeURL, pb.FieldType_FIELD_TYPE_URL},
		{adminclient.FieldTypeJSON, pb.FieldType_FIELD_TYPE_JSON},
		{adminclient.FieldTypeNumber, pb.FieldType_FIELD_TYPE_NUMBER},
		{adminclient.FieldTypeBool, pb.FieldType_FIELD_TYPE_BOOL},
		{"unknown", pb.FieldType_FIELD_TYPE_UNSPECIFIED},
		{"", pb.FieldType_FIELD_TYPE_UNSPECIFIED},
	}
	for _, c := range cases {
		if got := fieldTypeToProto(c.ft); got != c.want {
			t.Errorf("fieldTypeToProto(%q) = %v, want %v", c.ft, got, c.want)
		}
	}
}

func TestFieldTypeRoundTrip(t *testing.T) {
	types := []pb.FieldType{
		pb.FieldType_FIELD_TYPE_INT,
		pb.FieldType_FIELD_TYPE_STRING,
		pb.FieldType_FIELD_TYPE_TIME,
		pb.FieldType_FIELD_TYPE_DURATION,
		pb.FieldType_FIELD_TYPE_URL,
		pb.FieldType_FIELD_TYPE_JSON,
		pb.FieldType_FIELD_TYPE_NUMBER,
		pb.FieldType_FIELD_TYPE_BOOL,
	}
	for _, ft := range types {
		sdk := fieldTypeFromProto(ft)
		back := fieldTypeToProto(sdk)
		if back != ft {
			t.Errorf("round-trip %v: sdk=%q, back=%v", ft, sdk, back)
		}
	}
}

// --- fieldsToProto / fieldFromProto round-trips ---

func TestFieldsRoundTrip_Basic(t *testing.T) {
	fields := []adminclient.Field{
		{
			Path:        "app.name",
			Type:        adminclient.FieldTypeString,
			Nullable:    true,
			Deprecated:  false,
			Tags:        []string{"core"},
			ReadOnly:    false,
			WriteOnce:   true,
			Sensitive:   false,
			Default:     "default-name",
			Description: "the app name",
			Title:       "App Name",
			Example:     "MyApp",
			Format:      "hostname",
		},
	}
	protos := fieldsToProto(fields)
	if len(protos) != 1 {
		t.Fatalf("fieldsToProto returned %d results", len(protos))
	}
	back := fieldFromProto(protos[0])
	f := fields[0]
	if back.Path != f.Path {
		t.Errorf("Path: got %q, want %q", back.Path, f.Path)
	}
	if back.Type != f.Type {
		t.Errorf("Type: got %q, want %q", back.Type, f.Type)
	}
	if back.Nullable != f.Nullable {
		t.Errorf("Nullable: got %v, want %v", back.Nullable, f.Nullable)
	}
	if back.WriteOnce != f.WriteOnce {
		t.Errorf("WriteOnce: got %v, want %v", back.WriteOnce, f.WriteOnce)
	}
	if back.Default != f.Default {
		t.Errorf("Default: got %q, want %q", back.Default, f.Default)
	}
	if back.Description != f.Description {
		t.Errorf("Description: got %q, want %q", back.Description, f.Description)
	}
	if back.Title != f.Title {
		t.Errorf("Title: got %q, want %q", back.Title, f.Title)
	}
	if back.Format != f.Format {
		t.Errorf("Format: got %q, want %q", back.Format, f.Format)
	}
}

func TestFieldsRoundTrip_WithConstraints(t *testing.T) {
	min := 1.0
	max := 100.0
	minLen := int32(2)
	maxLen := int32(64)
	fields := []adminclient.Field{
		{
			Path: "rate",
			Type: "number",
			Constraints: &adminclient.FieldConstraints{
				Min:        &min,
				Max:        &max,
				MinLength:  &minLen,
				MaxLength:  &maxLen,
				Enum:       []string{"low", "high"},
				Pattern:    `^\d+$`,
				JSONSchema: `{"type":"number"}`,
			},
		},
	}
	back := fieldFromProto(fieldsToProto(fields)[0])
	c := back.Constraints
	if c == nil {
		t.Fatal("constraints nil after round-trip")
	}
	if c.Min == nil || *c.Min != min {
		t.Errorf("Min: got %v, want %v", c.Min, min)
	}
	if c.Max == nil || *c.Max != max {
		t.Errorf("Max: got %v, want %v", c.Max, max)
	}
	if c.MinLength == nil || *c.MinLength != minLen {
		t.Errorf("MinLength: got %v, want %v", c.MinLength, minLen)
	}
	if c.MaxLength == nil || *c.MaxLength != maxLen {
		t.Errorf("MaxLength: got %v, want %v", c.MaxLength, maxLen)
	}
	if len(c.Enum) != 2 || c.Enum[0] != "low" {
		t.Errorf("Enum: got %v", c.Enum)
	}
	if c.Pattern != `^\d+$` {
		t.Errorf("Pattern: got %q", c.Pattern)
	}
	if c.JSONSchema != `{"type":"number"}` {
		t.Errorf("JSONSchema: got %q", c.JSONSchema)
	}
}

func TestConstraintsRoundTrip_ExclusiveMinMax(t *testing.T) {
	fields := []adminclient.Field{
		{
			Path: "score",
			Type: "number",
			Constraints: &adminclient.FieldConstraints{
				ExclusiveMin: ptrF64(0.0),
				ExclusiveMax: ptrF64(1.0),
			},
		},
	}
	back := fieldFromProto(fieldsToProto(fields)[0])
	c := back.Constraints
	if c == nil {
		t.Fatal("constraints nil")
	}
	if c.ExclusiveMin == nil || *c.ExclusiveMin != 0.0 {
		t.Errorf("ExclusiveMin: got %v, want 0.0", c.ExclusiveMin)
	}
	if c.ExclusiveMax == nil || *c.ExclusiveMax != 1.0 {
		t.Errorf("ExclusiveMax: got %v, want 1.0", c.ExclusiveMax)
	}
}

func TestFieldsRoundTrip_OptionalFields(t *testing.T) {
	fields := []adminclient.Field{
		{
			Path:       "alias",
			Type:       adminclient.FieldTypeString,
			RedirectTo: "canonical.path",
			ExternalDocs: &adminclient.ExternalDocs{
				Description: "See docs",
				URL:         "https://docs.example.com",
			},
			Examples: map[string]adminclient.FieldExample{
				"ex1": {Value: "foo", Summary: "a foo example"},
			},
		},
	}
	back := fieldFromProto(fieldsToProto(fields)[0])
	if back.RedirectTo != "canonical.path" {
		t.Errorf("RedirectTo: got %q, want %q", back.RedirectTo, "canonical.path")
	}
	if back.ExternalDocs == nil || back.ExternalDocs.URL != "https://docs.example.com" {
		t.Errorf("ExternalDocs.URL: got %v", back.ExternalDocs)
	}
	if len(back.Examples) != 1 || back.Examples["ex1"].Value != "foo" {
		t.Errorf("Examples: got %v", back.Examples)
	}
}

func TestFieldFromProto_Nil(t *testing.T) {
	f := fieldFromProto(nil)
	if f.Path != "" {
		t.Error("fieldFromProto(nil) should return zero Field")
	}
}

func TestConstraintsFromProto_Nil(t *testing.T) {
	if constraintsFromProto(nil) != nil {
		t.Error("constraintsFromProto(nil) must return nil")
	}
}

func TestConstraintsToProto_Nil(t *testing.T) {
	if constraintsToProto(nil) != nil {
		t.Error("constraintsToProto(nil) must return nil")
	}
}

// --- configValueFromProto ---

func TestConfigValueFromProto(t *testing.T) {
	cv := &pb.ConfigValue{
		FieldPath: "app.name",
		Value:     &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "prod"}},
		Checksum:  "abc123",
	}
	out := configValueFromProto(cv)
	if out.FieldPath != "app.name" {
		t.Errorf("FieldPath: got %q", out.FieldPath)
	}
	if out.Value == nil || out.Value.MustStringValue() != "prod" {
		t.Errorf("Value: got %v", out.Value)
	}
	if out.Checksum != "abc123" {
		t.Errorf("Checksum: got %q", out.Checksum)
	}
}

// --- tenantFromProto ---

func TestTenantFromProto_Nil(t *testing.T) {
	if tenantFromProto(nil) != nil {
		t.Error("tenantFromProto(nil) must return nil")
	}
}

func TestTenantFromProto_WithTimestamps(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	proto := &pb.Tenant{
		Id:            "t1",
		Name:          "acme",
		SchemaId:      "s1",
		SchemaVersion: 3,
		CreatedAt:     timestamppb.New(now),
		UpdatedAt:     timestamppb.New(now.Add(time.Hour)),
	}
	out := tenantFromProto(proto)
	if out.ID != "t1" || out.Name != "acme" || out.SchemaID != "s1" || out.SchemaVersion != 3 {
		t.Errorf("basic fields: %+v", out)
	}
	if !out.CreatedAt.Equal(now) {
		t.Errorf("CreatedAt: got %v, want %v", out.CreatedAt, now)
	}
	if !out.UpdatedAt.Equal(now.Add(time.Hour)) {
		t.Errorf("UpdatedAt: got %v", out.UpdatedAt)
	}
}

// --- versionFromProto ---

func TestVersionFromProto_Nil(t *testing.T) {
	if versionFromProto(nil) != nil {
		t.Error("versionFromProto(nil) must return nil")
	}
}

func TestVersionFromProto_WithTimestamp(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	proto := &pb.ConfigVersion{
		Id:          "v1",
		TenantId:    "t1",
		Version:     5,
		Description: "bump",
		CreatedBy:   "alice",
		CreatedAt:   timestamppb.New(now),
	}
	out := versionFromProto(proto)
	if out.ID != "v1" || out.TenantID != "t1" || out.Version != 5 {
		t.Errorf("basic fields: %+v", out)
	}
	if !out.CreatedAt.Equal(now) {
		t.Errorf("CreatedAt: got %v, want %v", out.CreatedAt, now)
	}
}

// --- usageStatsFromProto ---

func TestUsageStatsFromProto_Nil(t *testing.T) {
	if usageStatsFromProto(nil) != nil {
		t.Error("usageStatsFromProto(nil) must return nil")
	}
}

func TestUsageStatsFromProto_WithOptionals(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	reader := "bob"
	proto := &pb.UsageStats{
		TenantId:   "t1",
		FieldPath:  "app.name",
		ReadCount:  42,
		LastReadBy: &reader,
		LastReadAt: timestamppb.New(now),
	}
	out := usageStatsFromProto(proto)
	if out.TenantID != "t1" || out.FieldPath != "app.name" || out.ReadCount != 42 {
		t.Errorf("basic fields: %+v", out)
	}
	if out.LastReadBy != "bob" {
		t.Errorf("LastReadBy: got %q", out.LastReadBy)
	}
	if out.LastReadAt == nil || !out.LastReadAt.Equal(now) {
		t.Errorf("LastReadAt: got %v, want %v", out.LastReadAt, now)
	}
}

// --- auditEntryFromProto ---

func TestAuditEntryFromProto_Nil(t *testing.T) {
	if auditEntryFromProto(nil) != nil {
		t.Error("auditEntryFromProto(nil) must return nil")
	}
}

func TestAuditEntryFromProto_WithOptionals(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	fp := "app.name"
	oldV := "old"
	newV := "new"
	cv := int32(7)
	proto := &pb.AuditEntry{
		Id:            "e1",
		TenantId:      "t1",
		Actor:         "alice",
		Action:        "set_field",
		FieldPath:     &fp,
		OldValue:      &oldV,
		NewValue:      &newV,
		ConfigVersion: &cv,
		CreatedAt:     timestamppb.New(now),
		ObjectKind:    "field",
		EntryHash:     "hash1",
		PreviousHash:  "hash0",
	}
	out := auditEntryFromProto(proto)
	if out.ID != "e1" || out.TenantID != "t1" || out.Actor != "alice" || out.Action != "set_field" {
		t.Errorf("basic fields: %+v", out)
	}
	if out.FieldPath != "app.name" {
		t.Errorf("FieldPath: got %q", out.FieldPath)
	}
	if out.OldValue != "old" || out.NewValue != "new" {
		t.Errorf("Values: old=%q new=%q", out.OldValue, out.NewValue)
	}
	if out.ConfigVersion == nil || *out.ConfigVersion != 7 {
		t.Errorf("ConfigVersion: got %v", out.ConfigVersion)
	}
	if !out.CreatedAt.Equal(now) {
		t.Errorf("CreatedAt: got %v", out.CreatedAt)
	}
	if out.ObjectKind != "field" || out.EntryHash != "hash1" || out.PreviousHash != "hash0" {
		t.Errorf("chain fields: %+v", out)
	}
}

// --- timeToProto ---

func TestTimeToProto_Nil(t *testing.T) {
	if timeToProto(nil) != nil {
		t.Error("timeToProto(nil) must return nil")
	}
}

func TestTimeToProto_WithTime(t *testing.T) {
	ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	proto := timeToProto(&ts)
	if proto == nil {
		t.Fatal("expected non-nil timestamp")
	}
	if !proto.AsTime().Equal(ts) {
		t.Errorf("got %v, want %v", proto.AsTime(), ts)
	}
}

// --- schemaFromProto ---

func TestSchemaFromProto_Nil(t *testing.T) {
	if schemaFromProto(nil) != nil {
		t.Error("schemaFromProto(nil) must return nil")
	}
}

func TestSchemaFromProto_Basic(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	pv := int32(1)
	proto := &pb.Schema{
		Id:                 "s1",
		Name:               "main",
		Description:        "desc",
		Version:            2,
		VersionDescription: "v2",
		Checksum:           "chk",
		Published:          true,
		ParentVersion:      &pv,
		CreatedAt:          timestamppb.New(now),
		Fields: []*pb.SchemaField{
			{Path: "app.name", Type: pb.FieldType_FIELD_TYPE_STRING},
		},
		Info: &pb.SchemaInfo{
			Title:  "Main Schema",
			Author: "alice",
			Labels: map[string]string{"env": "prod"},
			Contact: &pb.SchemaContact{
				Name:  "Alice",
				Email: "alice@example.com",
				Url:   "https://example.com",
			},
		},
	}
	out := schemaFromProto(proto)
	if out.ID != "s1" || out.Name != "main" || out.Version != 2 || !out.Published {
		t.Errorf("basic fields: %+v", out)
	}
	if out.ParentVersion == nil || *out.ParentVersion != 1 {
		t.Errorf("ParentVersion: got %v", out.ParentVersion)
	}
	if !out.CreatedAt.Equal(now) {
		t.Errorf("CreatedAt: got %v", out.CreatedAt)
	}
	if len(out.Fields) != 1 || out.Fields[0].Path != "app.name" {
		t.Errorf("Fields: got %v", out.Fields)
	}
	if out.Info == nil || out.Info.Title != "Main Schema" {
		t.Errorf("Info: got %v", out.Info)
	}
	if out.Info.Contact == nil || out.Info.Contact.Email != "alice@example.com" {
		t.Errorf("Info.Contact: got %v", out.Info.Contact)
	}
}

// ptrF64 and ptrI32 are used in TestConstraintsRoundTrip_ExclusiveMinMax.
var _ = ptrI32
