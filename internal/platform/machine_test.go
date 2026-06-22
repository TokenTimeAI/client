package platform

import (
	"net"
	"testing"
)

func TestIdentifyUsesHostnameAndPrimaryMAC(t *testing.T) {
	originalInterfaces := interfaceLister
	t.Cleanup(func() {
		interfaceLister = originalInterfaces
	})

	interfaceLister = func() ([]net.Interface, error) {
		return []net.Interface{
			{
				Name:         "lo0",
				Flags:        net.FlagUp | net.FlagLoopback,
				HardwareAddr: net.HardwareAddr{0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			},
			{
				Name:         "en0",
				Flags:        net.FlagUp,
				HardwareAddr: net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff},
			},
		}, nil
	}

	identity, err := Identify()
	if err != nil {
		t.Fatalf("identify: %v", err)
	}
	if identity.MACAddress != "aa:bb:cc:dd:ee:ff" {
		t.Fatalf("mac = %q, want aa:bb:cc:dd:ee:ff", identity.MACAddress)
	}
	if identity.NetworkName == "" {
		t.Fatal("expected network name")
	}
}
