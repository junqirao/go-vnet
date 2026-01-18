package main

import (
	"flag"
	"fmt"
	"io"
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

	// print config
	printConfig(config)

	// create client
	c := client.NewClient(config)

	// run
	c.Run()
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

// printConfig print config
func printConfig(config *client.Config) {
	fmt.Printf("network_id=%q\n", config.NetworkId)
	fmt.Printf("address=%q\n", config.Address)
	fmt.Printf("port=%d\n", config.Port)
	fmt.Printf("insecure_skip_verify=%t\n", config.InsecureSkipVerify)
	fmt.Printf("auth.type=%q\n", config.Auth.Type)

	if config.Auth.Password != "" {
		maskedPassword := maskSensitiveInfo(config.Auth.Password)
		fmt.Printf("auth.password=%q\n", maskedPassword)
	}

	if config.Auth.PrivateKey != "" {
		maskedPrivateKey := maskSensitiveInfo(config.Auth.PrivateKey)
		fmt.Printf("auth.private_key=%q\n", maskedPrivateKey)
	}

	if config.Auth.PublicKey != "" {
		maskedPublicKey := maskSensitiveInfo(config.Auth.PublicKey)
		fmt.Printf("auth.public_key=%q\n", maskedPublicKey)
	}
}

// maskSensitiveInfo desensitize sensitive info
func maskSensitiveInfo(info string) string {
	if len(info) <= 6 {
		return "***"
	}

	start := info[:3]
	end := info[len(info)-3:]
	mask := ""
	for i := 0; i < len(info)-6; i++ {
		mask += "*"
	}

	return start + mask + end
}
