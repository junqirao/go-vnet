package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"

	"gopkg.in/yaml.v3"

	"go-vnet/vnet/client"
)

func main() {
	configFile := flag.String("c", "config.yaml", "config file path")
	flag.Parse()

	config, err := loadConfigFromFile(*configFile)
	if err != nil {
		fmt.Printf("failed to read config file: %v\n", err)
		os.Exit(1)
	}

	go func() {
		log.Println("start pprof at :6060")
		log.Println(http.ListenAndServe("0.0.0.0:6060", nil))
	}()

	// create client
	c := client.NewClient(config)
	ctx := context.Background()

	// run
	c.Run(ctx)
}

// loadConfigFromFile load config from file
func loadConfigFromFile(filename string) (*client.Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var config = client.NewConfig()
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	return config, nil
}
