package main

import (
	"context"
	"errors"
	"testing"

	"github.com/opendecree/decree/sdk/adminclient"
	"github.com/opendecree/decree/sdk/configclient"
)

// fakeSchemaTransport stubs adminclient.SchemaTransport with just enough to
// serve GetTenant + GetSchema. Any other method panics (nil receiver on the
// embedded interface), which surfaces as a loud test failure if reached.
type fakeSchemaTransport struct {
	adminclient.SchemaTransport

	tenant *adminclient.Tenant
	schema *adminclient.Schema
}

func (f *fakeSchemaTransport) GetTenant(_ context.Context, id string) (*adminclient.Tenant, error) {
	if f.tenant == nil || id != f.tenant.ID {
		return nil, errors.New("tenant not found")
	}
	return f.tenant, nil
}

func (f *fakeSchemaTransport) GetSchema(_ context.Context, id string, _ *int32) (*adminclient.Schema, error) {
	if f.schema == nil || id != f.schema.ID {
		return nil, errors.New("schema not found")
	}
	return f.schema, nil
}

// fakeConfigTransport stubs configclient.Transport, capturing SetField /
// SetFields requests for assertions. Other methods panic if called.
type fakeConfigTransport struct {
	configclient.Transport

	setField  *configclient.SetFieldRequest
	setFields *configclient.SetFieldsRequest
}

func (f *fakeConfigTransport) SetField(_ context.Context, req *configclient.SetFieldRequest) (*configclient.SetFieldResponse, error) {
	f.setField = req
	return &configclient.SetFieldResponse{}, nil
}

func (f *fakeConfigTransport) SetFields(_ context.Context, req *configclient.SetFieldsRequest) (*configclient.SetFieldsResponse, error) {
	f.setFields = req
	return &configclient.SetFieldsResponse{}, nil
}

func buildClients(t *testing.T, fields ...adminclient.Field) (*adminclient.Client, *configclient.Client, *fakeConfigTransport) {
	t.Helper()
	admin := adminclient.New(
		adminclient.WithSchemaTransport(&fakeSchemaTransport{
			tenant: &adminclient.Tenant{ID: "t1", SchemaID: "s1", SchemaVersion: 1},
			schema: &adminclient.Schema{ID: "s1", Version: 1, Fields: fields},
		}),
	)
	tr := &fakeConfigTransport{}
	cfg := configclient.New(tr)
	return admin, cfg, tr
}

func TestRunConfigSet_PicksTypedValueKindFromSchema(t *testing.T) {
	tests := []struct {
		name     string
		field    adminclient.Field
		raw      string
		wantKind configclient.ValueKind
		check    func(*configclient.TypedValue) bool
	}{
		{"bool", adminclient.Field{Path: "x", Type: "bool"}, "true", configclient.KindBool, func(v *configclient.TypedValue) bool { return v.BoolValue() }},
		{"integer", adminclient.Field{Path: "x", Type: "integer"}, "42", configclient.KindInteger, func(v *configclient.TypedValue) bool { return v.IntValue() == 42 }},
		{"number", adminclient.Field{Path: "x", Type: "number"}, "3.14", configclient.KindNumber, func(v *configclient.TypedValue) bool { return v.FloatValue() == 3.14 }},
		{"duration", adminclient.Field{Path: "x", Type: "duration"}, "15s", configclient.KindDuration, func(v *configclient.TypedValue) bool { return v.DurationValue().Seconds() == 15 }},
		{"string", adminclient.Field{Path: "x", Type: "string"}, "hello", configclient.KindString, func(v *configclient.TypedValue) bool { return v.StringValue() == "hello" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			admin, cfg, tr := buildClients(t, tt.field)
			if err := runConfigSet(context.Background(), admin, cfg, "t1", "x", tt.raw); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tr.setField == nil {
				t.Fatal("SetField not called")
			}
			if tr.setField.Value == nil {
				t.Fatal("SetField called with nil Value")
			}
			if got := tr.setField.Value.Kind(); got != tt.wantKind {
				t.Fatalf("kind: got %v, want %v (CLI sent the wrong TypedValue kind for the schema's field type)", got, tt.wantKind)
			}
			if !tt.check(tr.setField.Value) {
				t.Error("captured value failed type-specific check")
			}
		})
	}
}

func TestRunConfigSet_UnknownFieldErrors(t *testing.T) {
	admin, cfg, tr := buildClients(t, adminclient.Field{Path: "known", Type: "string"})
	err := runConfigSet(context.Background(), admin, cfg, "t1", "unknown", "x")
	if err == nil {
		t.Fatal("expected error for unknown field")
	}
	if tr.setField != nil {
		t.Error("SetField should not be called for unknown field")
	}
}

func TestRunConfigSet_ParseErrorShortCircuits(t *testing.T) {
	admin, cfg, tr := buildClients(t, adminclient.Field{Path: "x", Type: "bool"})
	err := runConfigSet(context.Background(), admin, cfg, "t1", "x", "not-a-bool")
	if err == nil {
		t.Fatal("expected parse error")
	}
	if tr.setField != nil {
		t.Error("SetField should not be called when parsing fails")
	}
}

func TestRunConfigSetMany_SendsTypedValuesMatchingSchema(t *testing.T) {
	admin, cfg, tr := buildClients(t,
		adminclient.Field{Path: "a", Type: "integer"},
		adminclient.Field{Path: "b", Type: "bool"},
		adminclient.Field{Path: "c", Type: "duration"},
	)
	n, err := runConfigSetMany(context.Background(), admin, cfg, "t1", map[string]string{
		"a": "100",
		"b": "true",
		"c": "2m",
	}, "batch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 3 {
		t.Errorf("wrote %d, want 3", n)
	}
	if tr.setFields == nil {
		t.Fatal("SetFields not called")
	}
	if tr.setFields.Description != "batch" {
		t.Errorf("description: got %q, want %q", tr.setFields.Description, "batch")
	}
	byPath := map[string]*configclient.TypedValue{}
	for _, u := range tr.setFields.Updates {
		byPath[u.FieldPath] = u.Value
	}
	if byPath["a"].Kind() != configclient.KindInteger || byPath["a"].IntValue() != 100 {
		t.Errorf("a: got kind=%v int=%d", byPath["a"].Kind(), byPath["a"].IntValue())
	}
	if byPath["b"].Kind() != configclient.KindBool || !byPath["b"].BoolValue() {
		t.Errorf("b: got kind=%v bool=%v", byPath["b"].Kind(), byPath["b"].BoolValue())
	}
	if byPath["c"].Kind() != configclient.KindDuration || byPath["c"].DurationValue().Minutes() != 2 {
		t.Errorf("c: got kind=%v dur=%v", byPath["c"].Kind(), byPath["c"].DurationValue())
	}
}

func TestRunConfigSetMany_OneBadValueFailsAtomically(t *testing.T) {
	admin, cfg, tr := buildClients(t,
		adminclient.Field{Path: "a", Type: "integer"},
		adminclient.Field{Path: "b", Type: "bool"},
	)
	_, err := runConfigSetMany(context.Background(), admin, cfg, "t1", map[string]string{
		"a": "100",
		"b": "not-a-bool",
	}, "")
	if err == nil {
		t.Fatal("expected error")
	}
	if tr.setFields != nil {
		t.Error("SetFields should not be called if any value fails to parse")
	}
}
