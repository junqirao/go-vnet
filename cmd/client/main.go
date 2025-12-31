package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"

	"go-vnet/client"
)

func main() {
	// 定义命令行参数
	configFile := flag.String("c", "config.yaml", "配置文件路径")
	flag.Parse()

	// 读取配置文件
	config, err := loadConfigFromFile(*configFile)
	if err != nil {
		fmt.Printf("读取配置文件失败: %v\n", err)
		os.Exit(1)
	}

	// 打印配置信息
	printConfig(config)

	// 创建客户端
	c := client.NewClient(config)

	// 运行客户端
	err = c.Run(context.Background())
	if err != nil {
		panic(err)
		return
	}
}

// loadConfigFromFile 从文件中加载配置
func loadConfigFromFile(filename string) (*client.Config, error) {
	// 打开配置文件
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("打开配置文件失败: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	// 读取文件内容
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件内容失败: %w", err)
	}

	// 解析 YAML 配置
	var config = client.NewConfig()
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析YAML配置失败: %w", err)
	}

	return config, nil
}

// printConfig 打印配置信息，隐藏敏感信息
func printConfig(config *client.Config) {
	fmt.Println("Client Config:")
	fmt.Printf("Network ID: %s\n", config.NetworkId)
	fmt.Printf("Address: %s\n", config.Address)
	fmt.Printf("Port: %d\n", config.Port)
	fmt.Printf("Insecure Skip Verify: %t\n", config.InsecureSkipVerify)

	fmt.Println("\nAuthentication:")
	fmt.Printf("Auth Type: %s\n", config.Auth.Type)

	// 隐藏密码信息
	if config.Auth.Password != "" {
		maskedPassword := maskSensitiveInfo(config.Auth.Password)
		fmt.Printf("Password: %s\n", maskedPassword)
	}

	if config.Auth.PrivateKey != "" {
		maskedPrivateKey := maskSensitiveInfo(config.Auth.PrivateKey)
		fmt.Printf("Private Key: %s\n", maskedPrivateKey)
	}

	if config.Auth.PublicKey != "" {
		// 公钥通常不敏感，但为了统一性也进行部分隐藏
		maskedPublicKey := maskSensitiveInfo(config.Auth.PublicKey)
		fmt.Printf("Public Key: %s\n", maskedPublicKey)
	}

	fmt.Println()
}

// maskSensitiveInfo 隐藏敏感信息，只显示前3位和后3位
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
