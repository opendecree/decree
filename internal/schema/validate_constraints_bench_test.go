package schema

import (
	"fmt"
	"testing"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
)

func makeFields(n int, overlapPair bool) []*pb.SchemaField {
	fields := make([]*pb.SchemaField, 0, n)
	for i := range n {
		fields = append(fields, &pb.SchemaField{
			Path: fmt.Sprintf("group_%04d.field_%04d", i/10, i),
			Type: pb.FieldType_FIELD_TYPE_STRING,
		})
	}
	if overlapPair && n >= 2 {
		// Force the last sorted pair to overlap by injecting "x" and "x.y".
		fields[0].Path = "x"
		fields[1].Path = "x.y"
	}
	return fields
}

func BenchmarkValidateNoPrefixOverlap_NoOverlap10(b *testing.B) {
	fields := makeFields(10, false)
	b.ResetTimer()
	for b.Loop() {
		_ = validateNoPrefixOverlap(fields)
	}
}

func BenchmarkValidateNoPrefixOverlap_NoOverlap100(b *testing.B) {
	fields := makeFields(100, false)
	b.ResetTimer()
	for b.Loop() {
		_ = validateNoPrefixOverlap(fields)
	}
}

func BenchmarkValidateNoPrefixOverlap_NoOverlap1000(b *testing.B) {
	fields := makeFields(1000, false)
	b.ResetTimer()
	for b.Loop() {
		_ = validateNoPrefixOverlap(fields)
	}
}

func BenchmarkValidateNoPrefixOverlap_OverlapEarly(b *testing.B) {
	// Conflict surfaces in the first adjacent pair after sort.
	fields := makeFields(1000, true)
	b.ResetTimer()
	for b.Loop() {
		_ = validateNoPrefixOverlap(fields)
	}
}
