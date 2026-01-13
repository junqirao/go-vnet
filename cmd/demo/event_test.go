package main

import (
	"testing"
)

func TestDeleteElements(t *testing.T) {
	e := deviceReadEvent{
		n: 10,
	}
	buf := make([][]byte, e.n)
	sizes := make([]int, e.n)
	e.buf = &buf
	e.sizes = &sizes
	for i := 0; i < e.n; i++ {
		(*e.buf)[i] = []byte{byte(i)}
		(*e.sizes)[i] = i
	}
	t.Logf("buf(len=%d,cap=%d) = %+v\n", len(*e.buf), cap(*e.buf), e.buf)
	t.Logf("sizes(len=%d,cap=%d) = %+v\n", len(*e.sizes), cap(*e.sizes), e.sizes)
	t.Logf("n = %d", e.n)
	t.Log("===================")
	e.deleteElements(0, 2, 4, 6, 8)
	t.Logf("buf(len=%d,cap=%d) = %+v\n", len(*e.buf), cap(*e.buf), e.buf)
	t.Logf("sizes(len=%d,cap=%d) = %+v\n", len(*e.sizes), cap(*e.sizes), e.sizes)
	t.Logf("n = %d", e.n)
}

func BenchmarkDeleteElements(b *testing.B) {
	b.ReportAllocs()
	data := make([]*deviceReadEvent, b.N)
	for i := 0; i < b.N; i++ {
		e := deviceReadEvent{}
		buf := make([][]byte, 10)
		sizes := make([]int, 10)
		e.buf = &buf
		e.sizes = &sizes
		for j := 0; j < 10; j++ {
			(*e.buf)[j] = []byte{byte(j)}
			(*e.sizes)[j] = j
		}
		data[i] = &e
	}
	// b.Logf("data prepare done. %d", b.N)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data[i].deleteElements(5)
	}
}
