package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcfg"
	"gopkg.in/yaml.v3"

	web "go-vnet/manager/client"
	"go-vnet/vnet/client"
)

func main() {
	configFile := flag.String("c", "config.yaml", "config file path")
	flag.Parse()

	ctx := context.Background()

	config, err := loadConfigFromFile(*configFile)
	if err != nil {
		g.Log().Errorf(ctx, "failed to read config file: %v", err)
		os.Exit(1)
	}

	// create client
	c := client.NewClient(config)

	// run manager server
	if config.Manager.Enabled {
		// set adaptor for manager client
		adaptor, _ := gcfg.NewAdapterFile(*configFile)
		g.Cfg().SetAdapter(adaptor)

		web.RegisterClientInstance(c)
		go web.RunServer()
		g.Log().Infof(ctx, "manager server started at %s:%d", config.Manager.Listen, config.Manager.Port)
	}

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
