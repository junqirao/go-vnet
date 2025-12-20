package addresses

import (
	"net"
	"strings"
	"testing"
	"time"
)

func TestNewIPAllocator(t *testing.T) {
	tests := []struct {
		name      string
		subnet    string
		wantError bool
	}{
		{"valid subnet", "192.168.1.0/24", false},
		{"invalid subnet", "invalid", true},
		{"IPv6 not supported", "2001:db8::/32", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewIPAllocator(tt.subnet)
			if (err != nil) != tt.wantError {
				t.Errorf("NewIPAllocator() error = %v, wantError %v", err, tt.wantError)
				return
			}
		})
	}
}

func TestAssignRandom(t *testing.T) {
	allocator, err := NewIPAllocator("192.168.1.0/24")
	if err != nil {
		t.Fatalf("Failed to create allocator: %v", err)
	}

	// Test multiple random allocations
	for i := 0; i < 10; i++ {
		ip, err := allocator.AssignRandom()
		if err != nil {
			t.Fatalf("AssignRandom() error = %v", err)
		}

		t.Logf("assign random: %s", ip)
		ip = strings.Split(ip, "/")[0]

		// Verify the IP is in the correct range
		parsedIP := net.ParseIP(ip)
		_, ipnet, _ := net.ParseCIDR("192.168.1.0/24")
		if !ipnet.Contains(parsedIP) {
			t.Errorf("Assigned IP %s is not in the expected subnet", ip)
		}

		// Verify the IP is marked as allocated
		if !allocator.IsAllocated(ip) {
			t.Errorf("IP %s should be marked as allocated", ip)
		}

		time.Sleep(10 * time.Millisecond) // Ensure different random seeds
	}
}

func TestAssignSpecific(t *testing.T) {
	allocator, err := NewIPAllocator("192.168.1.0/24")
	if err != nil {
		t.Fatalf("Failed to create allocator: %v", err)
	}

	tests := []struct {
		name      string
		ip        string
		wantError bool
	}{
		{"valid IP", "192.168.1.10", false},
		{"out of range", "10.0.0.1", true},
		{"network address", "192.168.1.0", true},
		{"broadcast address", "192.168.1.255", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := allocator.AssignSpecific(tt.ip)
			if (err != nil) != tt.wantError {
				t.Errorf("AssignSpecific() error = %v, wantError %v, ip %s", err, tt.wantError, tt.ip)
			}

			if !tt.wantError && !allocator.IsAllocated(tt.ip) {
				t.Errorf("IP %s should be marked as allocated", tt.ip)
			}
		})
	}
}

func TestReleaseIP(t *testing.T) {
	allocator, err := NewIPAllocator("192.168.1.0/24")
	if err != nil {
		t.Fatalf("Failed to create allocator: %v", err)
	}

	// First assign an IP to release
	testIP := "192.168.1.10"
	if err := allocator.AssignSpecific(testIP); err != nil {
		t.Fatalf("Failed to assign test IP: %v", err)
	}

	// Test releasing the IP
	t.Run("release allocated IP", func(t *testing.T) {
		if err := allocator.ReleaseIP(testIP); err != nil {
			t.Errorf("ReleaseIP() error = %v", err)
		}

		if allocator.IsAllocated(testIP) {
			t.Errorf("IP %s should be released", testIP)
		}
	})

	// Test releasing non-existent IP
	t.Run("release non-existent IP", func(t *testing.T) {
		err := allocator.ReleaseIP("192.168.1.100")
		if err == nil {
			t.Error("Expected error when releasing non-existent IP")
		}
	})
}

func TestSerialization(t *testing.T) {
	allocator, err := NewIPAllocator("192.168.1.0/24")
	if err != nil {
		t.Fatalf("Failed to create allocator: %v", err)
	}

	// Allocate some IPs
	ips := []string{"192.168.1.10", "192.168.1.20", "192.168.1.30"}
	for _, ip := range ips {
		if err := allocator.AssignSpecific(ip); err != nil {
			t.Fatalf("Failed to assign IP %s: %v", ip, err)
		}
	}

	// Serialize
	data, err := allocator.Bytes()
	if err != nil {
		t.Fatalf("Bytes() error = %v", err)
	}

	// Deserialize
	loadedAllocator, err := LoadFromData(data)
	if err != nil {
		t.Fatalf("LoadFromData() error = %v", err)
	}

	// Verify the loaded state
	for _, ip := range ips {
		if !loadedAllocator.IsAllocated(ip) {
			t.Errorf("IP %s should be allocated after deserialization", ip)
		}
	}

	// Verify the network is the same
	if allocator.network != loadedAllocator.network {
		t.Errorf("Network mismatch after deserialization: got %s, want %s",
			loadedAllocator.network, allocator.network)
	}
}

func TestGetStatus(t *testing.T) {
	allocator, err := NewIPAllocator("192.168.1.0/24")
	if err != nil {
		t.Fatalf("Failed to create allocator: %v", err)
	}

	// Allocate some IPs
	testIPs := []string{"192.168.1.10", "192.168.1.20"}
	for _, ip := range testIPs {
		if err := allocator.AssignSpecific(ip); err != nil {
			t.Fatalf("Failed to assign IP %s: %v", ip, err)
		}
	}

	status := allocator.GetStatus()

	// Verify status fields
	if status["network"] != "192.168.1.0/24" {
		t.Errorf("Unexpected network in status: %v", status["network"])
	}

	if status["allocated"].(int) != len(testIPs) {
		t.Errorf("Unexpected number of allocated IPs: got %d, want %d",
			status["allocated"], len(testIPs))
	}

	// Verify allocated IPs list
	allocatedIPs := status["allocated_ips"].([]string)
	if len(allocatedIPs) != len(testIPs) {
		t.Fatalf("Unexpected number of allocated IPs in list: got %d, want %d",
			len(allocatedIPs), len(testIPs))
	}

	// Convert to map for easier checking
	allocatedMap := make(map[string]bool)
	for _, ip := range allocatedIPs {
		allocatedMap[ip] = true
	}

	for _, ip := range testIPs {
		if !allocatedMap[ip] {
			t.Errorf("Expected IP %s in allocated list", ip)
		}
	}
}
