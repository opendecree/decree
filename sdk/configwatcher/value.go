package configwatcher

import (
	"log/slog"
	"reflect"
	"sync"
	"sync/atomic"
	"time"
)

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
	closed     bool // true after close(); guarded by mu
	defaultVal T
	parse      func(string) (T, error)
	equal      func(a, b T) bool // equality check used for dedup
	changesCh  chan Change[T]

	dropped   atomic.Int64
	logger    *slog.Logger
	fieldPath string
}

func newValue[T any](defaultVal T, parse func(string) (T, error)) *Value[T] {
	return &Value[T]{
		current:    defaultVal,
		isSet:      false,
		defaultVal: defaultVal,
		parse:      parse,
		equal:      makeEqual[T](),
		changesCh:  make(chan Change[T], 16),
	}
}

// makeEqual returns an equality function for T.
// time.Time is compared via .Equal() to handle monotonic clock and location
// differences correctly. All other types fall back to reflect.DeepEqual.
func makeEqual[T any]() func(a, b T) bool {
	var zero T
	if _, ok := any(zero).(time.Time); ok {
		return func(a, b T) bool {
			return any(a).(time.Time).Equal(any(b).(time.Time))
		}
	}
	return func(a, b T) bool {
		return reflect.DeepEqual(a, b)
	}
}

// DroppedCount returns the number of change notifications that were dropped
// because the Changes channel was full. Each drop also emits a WARN log (if a
// logger was configured via [WithLogger]).
func (v *Value[T]) DroppedCount() int64 {
	return v.dropped.Load()
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

	if !isSet {
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

	v.notifyLocked(oldVal, wasNull)
}

// updateDirect sets the value directly without string parsing.
func (v *Value[T]) updateDirect(val T) {
	v.mu.Lock()
	defer v.mu.Unlock()

	oldVal := v.current
	wasNull := !v.isSet

	v.current = val
	v.isSet = true

	v.notifyLocked(oldVal, wasNull)
}

// notifyLocked emits a Change if the effective value or null-state changed.
// Must be called with v.mu held.
func (v *Value[T]) notifyLocked(oldVal T, wasNull bool) {
	// Do not send on a closed channel.
	if v.closed {
		return
	}

	// Skip notification when the effective value and null-state are unchanged.
	// This prevents flooding consumers on reconnect when the server returns the
	// same values that were already in effect.
	if wasNull == !v.isSet && v.equal(oldVal, v.current) {
		return
	}

	change := Change[T]{
		Old:     oldVal,
		New:     v.current,
		WasNull: wasNull,
		IsNull:  !v.isSet,
	}

	// Send change notification (non-blocking).
	select {
	case v.changesCh <- change:
	default:
		// Channel full — record the drop, log a warning, then drop oldest and
		// send the latest change so Get() callers can still observe the newest value.
		v.dropped.Add(1)
		if v.logger != nil {
			v.logger.Warn("configwatcher: change dropped (channel full)",
				"field", v.fieldPath,
				"dropped_total", v.dropped.Load(),
			)
		}
		select {
		case <-v.changesCh:
		default:
		}
		select {
		case v.changesCh <- change:
		default:
		}
	}
}

// close marks the value as closed and closes the changes channel.
// It is safe to call concurrently with update.
func (v *Value[T]) close() {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.closed {
		return
	}
	v.closed = true
	close(v.changesCh)
}
