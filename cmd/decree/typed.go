package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/opendecree/decree/sdk/adminclient"
	"github.com/opendecree/decree/sdk/configclient"
)

// parseTypedValue parses raw into a TypedValue matching the field type declared
// in the schema. fieldType values match [domain.FieldType] strings: string,
// integer, number, bool, time (RFC3339), duration (Go duration), url, json.
func parseTypedValue(fieldType, raw string) (*configclient.TypedValue, error) {
	switch fieldType {
	case "", "string":
		return configclient.StringVal(raw), nil
	case "integer":
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid integer value %q: %w", raw, err)
		}
		return configclient.IntVal(v), nil
	case "number":
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid number value %q: %w", raw, err)
		}
		return configclient.FloatVal(v), nil
	case "bool":
		v, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid bool value %q (want true/false): %w", raw, err)
		}
		return configclient.BoolVal(v), nil
	case "time":
		v, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			return nil, fmt.Errorf("invalid time value %q (want RFC3339, e.g. 2006-01-02T15:04:05Z): %w", raw, err)
		}
		return configclient.TimeVal(v), nil
	case "duration":
		v, err := time.ParseDuration(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid duration value %q (want Go duration, e.g. 15s, 2h): %w", raw, err)
		}
		return configclient.DurationVal(v), nil
	case "url":
		return configclient.URLVal(raw), nil
	case "json":
		var tmp any
		if err := json.Unmarshal([]byte(raw), &tmp); err != nil {
			return nil, fmt.Errorf("invalid json value: %w", err)
		}
		return configclient.JSONVal(raw), nil
	default:
		return nil, fmt.Errorf("unsupported field type %q", fieldType)
	}
}

// tenantFieldTypes fetches the schema assigned to tenantID and returns a
// map of field path → field type string.
func tenantFieldTypes(ctx context.Context, admin *adminclient.Client, tenantID string) (map[string]string, error) {
	t, err := admin.GetTenant(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("lookup tenant: %w", err)
	}
	s, err := admin.GetSchemaVersion(ctx, t.SchemaID, t.SchemaVersion)
	if err != nil {
		return nil, fmt.Errorf("lookup schema %s v%d: %w", t.SchemaID, t.SchemaVersion, err)
	}
	types := make(map[string]string, len(s.Fields))
	for _, f := range s.Fields {
		types[f.Path] = f.Type
	}
	return types, nil
}

// lookupFieldType returns the declared type for fieldPath, erroring if the
// field is unknown to the schema.
func lookupFieldType(types map[string]string, fieldPath string) (string, error) {
	ft, ok := types[fieldPath]
	if !ok {
		return "", fmt.Errorf("unknown field %q (not in schema)", fieldPath)
	}
	return ft, nil
}
