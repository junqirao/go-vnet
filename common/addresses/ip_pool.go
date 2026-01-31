package addresses

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"
)

// IPAllocator IP address allocator
type IPAllocator struct {
	network        string          // Network address in CIDR format
	startIP        net.IP          // Start IP
	endIP          net.IP          // End IP
	allocated      map[string]bool // Allocated IP mapping for quick lookup
	subnetMask     string          // Subnet mask
	subnetMaskBits int             // Subnet mask bits
	totalIPs       uint32          // Total number of IPs
	mutex          sync.RWMutex    // Read-write lock for concurrency safety
}

// serializableIPAllocator is a struct used for serialization
type serializableIPAllocator struct {
	Network        string   `json:"network"`
	StartIP        string   `json:"start_ip"`
	EndIP          string   `json:"end_ip"`
	Allocated      []string `json:"allocated"`
	SubnetMask     string   `json:"subnet_mask"`
	SubnetMaskBits int      `json:"subnet_mask_bits"`
	TotalIPs       uint32   `json:"total_ips"`
}

// NewIPAllocator creates a new IP address allocator
func NewIPAllocator(networkCIDR string) (*IPAllocator, error) {
	_, ipnet, err := net.ParseCIDR(networkCIDR)
	if err != nil {
		return nil, fmt.Errorf("invalid network CIDR: %v", err)
	}

	// Get network mask
	mask := ipnet.Mask
	subnetMask := fmt.Sprintf("%d.%d.%d.%d", mask[0], mask[1], mask[2], mask[3])
	subnetMaskBits, _ := ipnet.Mask.Size()
	// Validate IPv4
	startIP := ipnet.IP.To4()
	if startIP == nil {
		return nil, fmt.Errorf("only IPv4 addresses are supported")
	}

	// Calculate start and end IP (excluding network and broadcast addresses)
	startAvailable := make(net.IP, len(startIP))
	copy(startAvailable, startIP)
	startAvailable[3] = 1 // First available IP

	endAvailable := make(net.IP, len(startIP))
	copy(endAvailable, ipnet.IP.Mask(ipnet.Mask))
	for i := range endAvailable {
		endAvailable[i] |= ^ipnet.Mask[i]
	}
	endAvailable[3] = 254 // Last available IP

	// Calculate total number of IPs
	totalIPs := calculateIPRangeSize(startAvailable, endAvailable)

	return &IPAllocator{
		network:        networkCIDR,
		startIP:        startAvailable,
		endIP:          endAvailable,
		allocated:      make(map[string]bool),
		subnetMask:     subnetMask,
		subnetMaskBits: subnetMaskBits,
		totalIPs:       totalIPs,
		mutex:          sync.RWMutex{},
	}, nil
}

// calculateIPRangeSize calculates the number of IPs in the given range
func calculateIPRangeSize(startIP, endIP net.IP) uint32 {
	start := ipToUint32(startIP)
	end := ipToUint32(endIP)
	if start > end {
		return 0
	}
	return end - start + 1
}

// ipToUint32 converts an IP address to uint32
func ipToUint32(ip net.IP) uint32 {
	ip = ip.To4()
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}

// uint32ToIP converts an uint32 to an IP address
func uint32ToIP(val uint32) net.IP {
	return net.IPv4(byte(val>>24), byte(val>>16), byte(val>>8), byte(val))
}

// AssignRandom assigns a random available IP address
func (alloc *IPAllocator) AssignRandom() (string, error) {
	alloc.mutex.Lock()
	defer alloc.mutex.Unlock()

	if uint32(len(alloc.allocated)) >= alloc.totalIPs {
		return "", fmt.Errorf("no available IP addresses")
	}

	// Generate random IP
	start := ipToUint32(alloc.startIP)
	end := ipToUint32(alloc.endIP)

	// Maximum attempts to prevent infinite loop
	maxAttempts := 1000
	for i := 0; i < maxAttempts; i++ {
		randomIP := start + uint32(rand.New(rand.NewSource(time.Now().UnixMilli())).Intn(int(end-start+1)))
		ipStr := uint32ToIP(randomIP).String()

		if !alloc.allocated[ipStr] {
			alloc.allocated[ipStr] = true
			return fmt.Sprintf("%s/%d", ipStr, alloc.subnetMaskBits), nil
		}
	}

	return "", fmt.Errorf("failed to find available IP address, check IP pool status")
}

// AssignSpecific assigns the specified IP address
func (alloc *IPAllocator) AssignSpecific(ip string) error {
	alloc.mutex.Lock()
	defer alloc.mutex.Unlock()

	// Validate IP format
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return fmt.Errorf("invalid IP address format: %s", ip)
	}

	// Check if IP is within range
	_, ipnet, _ := net.ParseCIDR(alloc.network)
	if !ipnet.Contains(parsedIP) {
		return fmt.Errorf("IP address %s is not within network %s", ip, alloc.network)
	}

	// Check if IP is already allocated
	if alloc.allocated[ip] {
		return fmt.Errorf("IP address %s is already allocated", ip)
	}

	// Check if IP is within the valid range (startIP <= ip <= endIP)
	parsedStartIP := ipToUint32(alloc.startIP)
	parsedEndIP := ipToUint32(alloc.endIP)
	parsedTargetIP := ipToUint32(parsedIP)
	if parsedTargetIP < parsedStartIP || parsedTargetIP > parsedEndIP {
		return fmt.Errorf("IP address %s is out of range. Must be between %s and %s", ip, alloc.startIP, alloc.endIP)
	}

	alloc.allocated[ip] = true
	return nil
}

// ReleaseIP releases an allocated IP address
func (alloc *IPAllocator) ReleaseIP(ip string) error {
	alloc.mutex.Lock()
	defer alloc.mutex.Unlock()

	ip = strings.Split(ip, "/")[0]

	if !alloc.allocated[ip] {
		return fmt.Errorf("IP address %s is not allocated", ip)
	}

	delete(alloc.allocated, ip)
	return nil
}

// IsAllocated checks if an IP address is allocated
func (alloc *IPAllocator) IsAllocated(ip string) bool {
	alloc.mutex.RLock()
	defer alloc.mutex.RUnlock()
	ip = strings.Split(ip, "/")[0]
	return alloc.allocated[ip]
}

// GetStatus returns the current status of the allocator
func (alloc *IPAllocator) GetStatus() map[string]interface{} {
	alloc.mutex.RLock()
	defer alloc.mutex.RUnlock()

	allocatedIPs := make([]string, 0, len(alloc.allocated))
	for ip := range alloc.allocated {
		allocatedIPs = append(allocatedIPs, ip)
	}

	return map[string]interface{}{
		"network":       alloc.network,
		"subnet_mask":   alloc.subnetMask,
		"total_ips":     alloc.totalIPs,
		"allocated":     len(alloc.allocated),
		"available":     alloc.totalIPs - uint32(len(alloc.allocated)),
		"allocated_ips": allocatedIPs,
		"start_ip":      alloc.startIP.String(),
		"end_ip":        alloc.endIP.String(),
	}
}

// GetNetworkInfo returns network information
func (alloc *IPAllocator) GetNetworkInfo() map[string]string {
	alloc.mutex.RLock()
	defer alloc.mutex.RUnlock()

	return map[string]string{
		"network":     alloc.network,
		"subnet_mask": alloc.subnetMask,
		"start_ip":    alloc.startIP.String(),
		"end_ip":      alloc.endIP.String(),
	}
}

// Bytes to byte
func (alloc *IPAllocator) Bytes() (data []byte, err error) {
	alloc.mutex.RLock()
	defer alloc.mutex.RUnlock()

	allocatedIPs := make([]string, 0, len(alloc.allocated))
	for ip := range alloc.allocated {
		allocatedIPs = append(allocatedIPs, ip)
	}

	serializable := serializableIPAllocator{
		Network:    alloc.network,
		StartIP:    alloc.startIP.String(),
		EndIP:      alloc.endIP.String(),
		Allocated:  allocatedIPs,
		SubnetMask: alloc.subnetMask,
		TotalIPs:   alloc.totalIPs,
	}

	data, err = json.Marshal(serializable)
	return
}

// LoadFromData loads state from byte data
func LoadFromData(data []byte) (*IPAllocator, error) {
	var serializable serializableIPAllocator
	err := json.Unmarshal(data, &serializable)
	if err != nil {
		return nil, err
	}

	// new allocator
	allocator, err := NewIPAllocator(serializable.Network)
	if err != nil {
		return nil, err
	}

	// restore
	allocator.mutex.Lock()
	defer allocator.mutex.Unlock()

	for _, ip := range serializable.Allocated {
		allocator.allocated[ip] = true
	}

	return allocator, nil
}
