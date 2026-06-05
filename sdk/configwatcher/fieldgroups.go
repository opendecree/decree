package configwatcher

import (
	"fmt"
	"reflect"
	"time"
)

// Group registers a set of watched fields mapped from a struct's `decree` tags.
// Use [Watcher.NewGroup] to create one.
type Group struct {
	getters []func(rv reflect.Value) // one per tagged field
	typ     reflect.Type             // struct type for validation
}

// NewGroup registers all `decree`-tagged fields in the struct pointed to by s
// with the watcher and returns a Group. The struct pointer must remain valid
// for the lifetime of the watcher.
//
// Example:
//
//	type AppConfig struct {
//	    Name  string  `decree:"app.name"`
//	    Debug bool    `decree:"app.debug"`
//	}
//	g, err := w.NewGroup(ctx, &AppConfig{})
func (w *Watcher) NewGroup(s any) (*Group, error) {
	rv := reflect.ValueOf(s)
	if rv.Kind() != reflect.Ptr || rv.IsNil() || rv.Elem().Kind() != reflect.Struct {
		return nil, fmt.Errorf("configwatcher: NewGroup: s must be a non-nil pointer to a struct, got %T", s)
	}
	rv = rv.Elem()
	rt := rv.Type()

	g := &Group{typ: rt}

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

		getter, err := registerGroupField(w, tag, fv)
		if err != nil {
			return nil, fmt.Errorf("configwatcher: NewGroup: field %s (decree:%q): %w", field.Name, tag, err)
		}
		idx := i
		g.getters = append(g.getters, func(target reflect.Value) {
			getter(target.Field(idx))
		})
	}
	return g, nil
}

// Fill populates the struct pointed to by s with the current values of all
// watched fields. s must be the same type that was passed to NewGroup.
func (g *Group) Fill(s any) error {
	rv := reflect.ValueOf(s)
	if rv.Kind() != reflect.Ptr || rv.IsNil() || rv.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("configwatcher: Group.Fill: s must be a non-nil pointer to a struct")
	}
	rv = rv.Elem()
	if rv.Type() != g.typ {
		return fmt.Errorf("configwatcher: Group.Fill: type mismatch: got %s, want %s", rv.Type(), g.typ)
	}
	for _, get := range g.getters {
		get(rv)
	}
	return nil
}

var durType = reflect.TypeOf(time.Duration(0))

func registerGroupField(w *Watcher, path string, fv reflect.Value) (func(dst reflect.Value), error) {
	switch {
	case fv.Type() == durType:
		val, err := w.Duration(path, 0)
		if err != nil {
			return nil, err
		}
		return func(dst reflect.Value) { dst.SetInt(int64(val.Get())) }, nil

	case fv.Kind() == reflect.String:
		val, err := w.String(path, "")
		if err != nil {
			return nil, err
		}
		return func(dst reflect.Value) { dst.SetString(val.Get()) }, nil

	case fv.Kind() == reflect.Bool:
		val, err := w.Bool(path, false)
		if err != nil {
			return nil, err
		}
		return func(dst reflect.Value) { dst.SetBool(val.Get()) }, nil

	case fv.Kind() >= reflect.Int && fv.Kind() <= reflect.Int64:
		val, err := w.Int(path, 0)
		if err != nil {
			return nil, err
		}
		return func(dst reflect.Value) { dst.SetInt(val.Get()) }, nil

	case fv.Kind() == reflect.Float64 || fv.Kind() == reflect.Float32:
		val, err := w.Float(path, 0)
		if err != nil {
			return nil, err
		}
		return func(dst reflect.Value) { dst.SetFloat(val.Get()) }, nil

	default:
		return nil, fmt.Errorf("unsupported field type %s", fv.Type())
	}
}
