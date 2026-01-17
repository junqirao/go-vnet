package main

import (
	"flag"

	"go-vnet/vnet/client"
)

func main() {
	server := flag.String("server", "", "server ip")
	flag.Parse()

	c := client.NewClient(&client.Config{
		Server: *server,
		Type:   client.TypeQuic,
	})
	c.Run()
}
