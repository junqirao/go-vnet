package network

import (
	"sync"
)

var (
	globalManager *Manager
	once          sync.Once
)

// GetManager 获取全局单例网络管理器
func GetManager() *Manager {
	once.Do(func() {
		globalManager = NewNetworkManager()
	})
	return globalManager
}

type (
	Manager struct {
		mu       sync.RWMutex
		networks map[string]*Network
	}
)

func NewNetworkManager() *Manager {
	return &Manager{
		networks: make(map[string]*Network),
	}
}

// RegisterNetwork 注册网络
func (nm *Manager) RegisterNetwork(network *Network) {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	if network != nil && network.ID != "" {
		nm.networks[network.ID] = network
	}
}

// RemoveNetwork 通过网络ID移除网络
func (nm *Manager) RemoveNetwork(networkID string) {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	delete(nm.networks, networkID)
}

// GetNetwork 通过网络ID获取网络
func (nm *Manager) GetNetwork(networkID string) (*Network, bool) {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	network, exists := nm.networks[networkID]
	return network, exists
}

// GetAllNetworks 获取所有网络（线程安全副本）
func (nm *Manager) GetAllNetworks() map[string]*Network {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	result := make(map[string]*Network)
	for id, network := range nm.networks {
		result[id] = network
	}
	return result
}

// NetworkCount 获取网络数量
func (nm *Manager) NetworkCount() int {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	return len(nm.networks)
}
