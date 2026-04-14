package configwatcher

import "sync"

// Change represents a value transition for a typed config field.
type Change[T any] struct {
	// Old is the previous value (or the default if WasNull is true).
	Old T
	// New is the current value (or the default if IsNull is true).
	New T
	// WasNull is true if the previous value was null or missing.
	WasNull bool
	// IsNull is true if the new value is null or missing.
	IsNull bool
}

// Value is a live, typed configuration value that automatically updates
// when the underlying config changes via the subscription stream.
//
// Value is safe for concurrent use. [Value.Get] never blocks and always
// returns the most recent value. Use [Value.Changes] to observe transitions.
type Value[T any] struct {
	mu         sync.RWMutex
	current    T
	isSet      bool
	defaultVal T
	parse      func(string) (T, error)
	changesCh  chan Change[T]
}

func newValue[T any](defaultVal T, parse func(string) (T, error)) *Value[T] {
	return &Value[T]{
		current:    defaultVal,
		isSet:      false,
		defaultVal: defaultVal,
		parse:      parse,
		changesCh:  make(chan Change[T], 16),
	}
}

// Get returns the current value of the field. If the field is null or missing,
// the default value provided during registration is returned.
//
// Get never blocks and is safe for concurrent use.
func (v *Value[T]) Get() T {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.current
}

// GetWithNull returns the current value and whether the field has a value set.
// If ok is false, the field is null or missing and val is the default value.
func (v *Value[T]) GetWithNull() (val T, ok bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.current, v.isSet
}

// Changes returns a channel that receives [Change] events whenever the field
// value is updated via the subscription stream. The channel is buffered (capacity 16).
//
// The channel is closed when the [Watcher] is closed.
func (v *Value[T]) Changes() <-chan Change[T] {
	return v.changesCh
}

// update is called internally when a new raw value arrives from the stream.
func (v *Value[T]) update(rawValue string, isSet bool) {
	v.mu.Lock()
	defer v.mu.Unlock()

	oldVal := v.current
	wasNull := !v.isSet

	if !isSet || rawValue == "" {
		v.current = v.defaultVal
		v.isSet = false
	} else {
		parsed, err := v.parse(rawValue)
		if err != nil {
			// Parse error — keep default, mark as not set.
			v.current = v.defaultVal
			v.isSet = false
		} else {
			v.current = parsed
			v.isSet = true
		}
	}

	// Send change notification (non-blocking).
	select {
	case v.changesCh <- Change[T]{
		Old:     oldVal,
		New:     v.current,
		WasNull: wasNull,
		IsNull:  !v.isSet,
	}:
	default:
		// Channel full — drop oldest, send new.
		select {
		case <-v.changesCh:
		default:
		}
		select {
		case v.changesCh <- Change[T]{
			Old:     oldVal,
			New:     v.current,
			WasNull: wasNull,
			IsNull:  !v.isSet,
		}:
		default:
		}
	}
}

// close closes the changes channel.
func (v *Value[T]) close() {
	close(v.changesCh)
}
