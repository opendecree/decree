package config

import (
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/zeevdr/central-config-service/api/centralconfig/v1"
)

func BenchmarkTypedValueToString_Integer(b *testing.B) {
	tv := &pb.TypedValue{Kind: &pb.TypedValue_IntegerValue{IntegerValue: 42}}
	for b.Loop() {
		typedValueToString(tv)
	}
}

func BenchmarkTypedValueToString_Number(b *testing.B) {
	tv := &pb.TypedValue{Kind: &pb.TypedValue_NumberValue{NumberValue: 3.14}}
	for b.Loop() {
		typedValueToString(tv)
	}
}

func BenchmarkTypedValueToString_String(b *testing.B) {
	tv := &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "hello world"}}
	for b.Loop() {
		typedValueToString(tv)
	}
}

func BenchmarkTypedValueToString_Bool(b *testing.B) {
	tv := &pb.TypedValue{Kind: &pb.TypedValue_BoolValue{BoolValue: true}}
	for b.Loop() {
		typedValueToString(tv)
	}
}

func BenchmarkTypedValueToString_Time(b *testing.B) {
	tv := &pb.TypedValue{Kind: &pb.TypedValue_TimeValue{TimeValue: timestamppb.Now()}}
	for b.Loop() {
		typedValueToString(tv)
	}
}

func BenchmarkTypedValueToString_Duration(b *testing.B) {
	tv := &pb.TypedValue{Kind: &pb.TypedValue_DurationValue{DurationValue: durationpb.New(24 * time.Hour)}}
	for b.Loop() {
		typedValueToString(tv)
	}
}

func BenchmarkStringToTypedValue_String(b *testing.B) {
	s := "hello world"
	for b.Loop() {
		stringToTypedValue(&s, pb.FieldType_FIELD_TYPE_STRING) //nolint:unparam
	}
}

func BenchmarkComputeChecksum(b *testing.B) {
	for b.Loop() {
		computeChecksum("some-config-value-here")
	}
}
