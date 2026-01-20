package hub

import (
	"testing"
)

func TestBytes(t *testing.T) {
	src := make([]byte, 10)
	dst := make([]byte, 10)
	for i := 0; i < 10; i++ {
		src[0] = byte(i)
	}
	copy(dst, src)
	src[0] = 254
	t.Logf("dst = %v", dst)
	t.Logf("src = %v", src)
}

func BenchmarkBytes(b *testing.B) {
	src := make([]byte, 10)
	dst := make([]byte, 10)
	for i := 0; i < 10; i++ {
		src[0] = byte(i)
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		copy(dst, src)
	}
}
