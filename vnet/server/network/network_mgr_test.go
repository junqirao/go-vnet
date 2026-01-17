package network

import (
	"fmt"
	"testing"
)

func TestNetworkManager(t *testing.T) {
	// 创建网络管理器
	nm := NewNetworkManager()

	// 测试初始状态
	if nm.NetworkCount() != 0 {
		t.Errorf("Expected 0 networks, got %d", nm.NetworkCount())
	}

	// 创建测试网络
	network1 := &Network{ID: "test-network-1", CIDR: "192.168.1.0/24"}
	network2 := &Network{ID: "test-network-2", CIDR: "10.0.0.0/24"}

	// 测试注册网络
	nm.RegisterNetwork(network1)
	nm.RegisterNetwork(network2)

	if nm.NetworkCount() != 2 {
		t.Errorf("Expected 2 networks after registration, got %d", nm.NetworkCount())
	}

	// 测试获取网络
	network, exists := nm.GetNetwork("test-network-1")
	if !exists {
		t.Error("Network test-network-1 should exist")
	}
	if network.ID != "test-network-1" {
		t.Errorf("Expected network ID test-network-1, got %s", network.ID)
	}

	// 测试获取不存在的网络
	_, exists = nm.GetNetwork("non-existent")
	if exists {
		t.Error("Non-existent network should not exist")
	}

	// 测试移除网络
	nm.RemoveNetwork("test-network-1")
	if nm.NetworkCount() != 1 {
		t.Errorf("Expected 1 network after removal, got %d", nm.NetworkCount())
	}

	_, exists = nm.GetNetwork("test-network-1")
	if exists {
		t.Error("Removed network should not exist")
	}

	// 测试获取所有网络
	allNetworks := nm.GetAllNetworks()
	if len(allNetworks) != 1 {
		t.Errorf("Expected 1 network in GetAllNetworks, got %d", len(allNetworks))
	}

	if _, exists := allNetworks["test-network-2"]; !exists {
		t.Error("test-network-2 should be in GetAllNetworks result")
	}
}

func TestNetworkManagerConcurrency(t *testing.T) {
	nm := NewNetworkManager()

	// 并发注册网络
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			network := &Network{ID: fmt.Sprintf("network-%d", id), CIDR: fmt.Sprintf("192.168.%d.0/24", id)}
			nm.RegisterNetwork(network)
			done <- true
		}(i)
	}

	// 等待所有goroutine完成
	for i := 0; i < 10; i++ {
		<-done
	}

	if nm.NetworkCount() != 10 {
		t.Errorf("Expected 10 networks after concurrent registration, got %d", nm.NetworkCount())
	}
}
