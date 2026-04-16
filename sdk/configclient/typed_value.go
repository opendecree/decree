package configclient

import (
	"fmt"
	"strconv"
	"time"
)

// ValueKind identifies the type of a [TypedValue].
type ValueKind int8

const (
	KindString   ValueKind = iota + 1 // string
	KindInteger                       // int64
	KindNumber                        // float64
	KindBool                          // bool
	KindTime                          // time.Time
	KindDuration                      // time.Duration
	KindURL                           // URL (stored as string)
	KindJSON                          // JSON (stored as string)
)

// TypedValue holds a configuration value with its type information.
// Use the constructor functions ([StringVal], [IntVal], etc.) to create values.
type TypedValue struct {
	kind ValueKind
	str  string
	i    int64
	num  float64
	b    bool
	t    time.Time
	d    time.Duration
}

// Kind returns the value's type.
func (tv *TypedValue) Kind() ValueKind { return tv.kind }

// StringValue returns the string value. Panics if Kind is not KindString.
func (tv *TypedValue) StringValue() string {
	if tv.kind != KindString {
		panic("TypedValue.StringValue called on non-string value")
	}
	return tv.str
}

// IntValue returns the int64 value. Panics if Kind is not KindInteger.
func (tv *TypedValue) IntValue() int64 {
	if tv.kind != KindInteger {
		panic("TypedValue.IntValue called on non-integer value")
	}
	return tv.i
}

// FloatValue returns the float64 value. Panics if Kind is not KindNumber.
func (tv *TypedValue) FloatValue() float64 {
	if tv.kind != KindNumber {
		panic("TypedValue.FloatValue called on non-number value")
	}
	return tv.num
}

// BoolValue returns the bool value. Panics if Kind is not KindBool.
func (tv *TypedValue) BoolValue() bool {
	if tv.kind != KindBool {
		panic("TypedValue.BoolValue called on non-bool value")
	}
	return tv.b
}

// TimeValue returns the time.Time value. Panics if Kind is not KindTime.
func (tv *TypedValue) TimeValue() time.Time {
	if tv.kind != KindTime {
		panic("TypedValue.TimeValue called on non-time value")
	}
	return tv.t
}

// DurationValue returns the time.Duration value. Panics if Kind is not KindDuration.
func (tv *TypedValue) DurationValue() time.Duration {
	if tv.kind != KindDuration {
		panic("TypedValue.DurationValue called on non-duration value")
	}
	return tv.d
}

// URLValue returns the URL string value. Panics if Kind is not KindURL.
func (tv *TypedValue) URLValue() string {
	if tv.kind != KindURL {
		panic("TypedValue.URLValue called on non-url value")
	}
	return tv.str
}

// JSONValue returns the JSON string value. Panics if Kind is not KindJSON.
func (tv *TypedValue) JSONValue() string {
	if tv.kind != KindJSON {
		panic("TypedValue.JSONValue called on non-json value")
	}
	return tv.str
}

// String returns the value as a string representation.
func (tv *TypedValue) String() string {
	if tv == nil {
		return ""
	}
	switch tv.kind {
	case KindString:
		return tv.str
	case KindInteger:
		return fmt.Sprintf("%d", tv.i)
	case KindNumber:
		return strconv.FormatFloat(tv.num, 'f', -1, 64)
	case KindBool:
		return strconv.FormatBool(tv.b)
	case KindTime:
		return tv.t.Format(time.RFC3339Nano)
	case KindDuration:
		return tv.d.String()
	case KindURL:
		return tv.str
	case KindJSON:
		return tv.str
	default:
		return ""
	}
}

// StringVal creates a TypedValue holding a string.
func StringVal(s string) *TypedValue {
	return &TypedValue{kind: KindString, str: s}
}

// IntVal creates a TypedValue holding an int64.
func IntVal(n int64) *TypedValue {
	return &TypedValue{kind: KindInteger, i: n}
}

// FloatVal creates a TypedValue holding a float64.
func FloatVal(f float64) *TypedValue {
	return &TypedValue{kind: KindNumber, num: f}
}

// BoolVal creates a TypedValue holding a bool.
func BoolVal(b bool) *TypedValue {
	return &TypedValue{kind: KindBool, b: b}
}

// TimeVal creates a TypedValue holding a time.Time.
func TimeVal(t time.Time) *TypedValue {
	return &TypedValue{kind: KindTime, t: t}
}

// DurationVal creates a TypedValue holding a time.Duration.
func DurationVal(d time.Duration) *TypedValue {
	return &TypedValue{kind: KindDuration, d: d}
}

// URLVal creates a TypedValue holding a URL string.
func URLVal(s string) *TypedValue {
	return &TypedValue{kind: KindURL, str: s}
}

// JSONVal creates a TypedValue holding a JSON string.
func JSONVal(s string) *TypedValue {
	return &TypedValue{kind: KindJSON, str: s}
}
