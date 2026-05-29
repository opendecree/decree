package configwatcher

import (
	"testing"
)

func BenchmarkValueUpdate(b *testing.B) {
	v := newValue("default", parseString)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v.update("hello", true)
		// drain to prevent channel-full path from dominating
		select {
		case <-v.changesCh:
		default:
		}
	}
}

func BenchmarkValueGet(b *testing.B) {
	v := newValue("default", parseString)
	v.update("hello", true)
	// drain initial change
	<-v.changesCh

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = v.Get()
	}
}
