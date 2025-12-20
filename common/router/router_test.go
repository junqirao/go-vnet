package router

import (
	"fmt"
	"testing"
)

func TestRouteTable_Lookup(t *testing.T) {
	table := NewRouteTable()
	for i := 0; i < 255; i++ {
		_ = table.AddRoute(fmt.Sprintf("192.168.%v.0/24", i), "123")
	}
	for i := 0; i < 255; i++ {
		_ = table.AddRoute(fmt.Sprintf("192.168.1.%v/32", i), i)
	}
	res, ok := table.Lookup("192.168.1.55")
	if !ok {
		t.Fatal("lookup failed")
		return
	}
	if res != 55 {
		t.Fatal("lookup failed")
		return
	}
}

func BenchmarkRouteTable_Lookup(b *testing.B) {
	table := NewRouteTable()
	for i := 0; i < 255; i++ {
		_ = table.AddRoute(fmt.Sprintf("192.168.%v.0/24", i), "123")
	}
	for i := 0; i < 255; i++ {
		_ = table.AddRoute(fmt.Sprintf("192.168.1.%v/32", i), i)
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		table.Lookup("192.168.1.55")
	}
}
