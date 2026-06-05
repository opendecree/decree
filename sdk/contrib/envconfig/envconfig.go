// Package envconfig populates struct fields from OpenDecree config values.
//
// Fields are mapped via the `decree` struct tag:
//
//	type AppConfig struct {
//	    Name    string        `decree:"app.name"`
//	    Debug   bool          `decree:"app.debug"`
//	    Timeout time.Duration `decree:"jobs.timeout"`
//	}
package envconfig

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/opendecree/decree/sdk/configclient"
)

// Process reads all fields tagged with `decree:"<field-path>"` from the struct
// pointed to by v and fetches their values from the decree config.
func Process(ctx context.Context, client *configclient.Client, tenantID string, v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() || rv.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("envconfig: v must be a non-nil pointer to a struct, got %T", v)
	}
	rv = rv.Elem()
	rt := rv.Type()

	for i := range rv.NumField() {
		field := rt.Field(i)
		tag := field.Tag.Get("decree")
		if tag == "" || tag == "-" {
			continue
		}
		fv := rv.Field(i)
		if !fv.CanSet() {
			continue
		}
		if err := setField(ctx, client, tenantID, tag, fv); err != nil {
			return fmt.Errorf("envconfig: field %s (decree:%q): %w", field.Name, tag, err)
		}
	}
	return nil
}

var durationType = reflect.TypeOf(time.Duration(0))

func setField(ctx context.Context, client *configclient.Client, tenantID, path string, fv reflect.Value) error {
	switch {
	case fv.Type() == durationType:
		v, err := client.GetDuration(ctx, tenantID, path)
		if err != nil {
			return err
		}
		fv.SetInt(int64(v))
	case fv.Kind() == reflect.String:
		v, err := client.GetString(ctx, tenantID, path)
		if err != nil {
			return err
		}
		fv.SetString(v)
	case fv.Kind() == reflect.Bool:
		v, err := client.GetBool(ctx, tenantID, path)
		if err != nil {
			return err
		}
		fv.SetBool(v)
	case fv.Kind() >= reflect.Int && fv.Kind() <= reflect.Int64:
		v, err := client.GetInt(ctx, tenantID, path)
		if err != nil {
			return err
		}
		fv.SetInt(v)
	case fv.Kind() == reflect.Float64 || fv.Kind() == reflect.Float32:
		v, err := client.GetFloat(ctx, tenantID, path)
		if err != nil {
			return err
		}
		fv.SetFloat(v)
	default:
		return fmt.Errorf("unsupported field type %s", fv.Type())
	}
	return nil
}
