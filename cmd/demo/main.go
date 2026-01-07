package main

import (
	"flag"
)

func main() {
	typ := flag.String("type", "", "server or client")
	ip := flag.String("ip", "", "client ip, e.g. 192.168.8.1")
	server := flag.String("server", "", "server ip")
	dst := flag.String("dst", "", "destination ip")
	flag.Parse()

	switch *typ {
	case "server":
		s := NewServer()
		s.Run()
	default:
		c := NewClient(*ip, *server, *dst)
		c.Run()
	}
}
