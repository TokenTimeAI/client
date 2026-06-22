package platform

import (
	"fmt"
	"net"
	"os"
	"strings"
)

type Identity struct {
	MACAddress  string
	NetworkName string
}

func Identify() (Identity, error) {
	networkName, err := os.Hostname()
	if err != nil || strings.TrimSpace(networkName) == "" {
		networkName = "unknown-machine"
	}

	macAddress, err := primaryMACAddress()
	if err != nil {
		return Identity{NetworkName: networkName}, err
	}

	return Identity{
		MACAddress:  macAddress,
		NetworkName: networkName,
	}, nil
}

func ResolveIdentity(fallbackNetworkName string) Identity {
	identity, err := Identify()
	if err != nil {
		return Identity{NetworkName: fallbackNetworkName}
	}
	if identity.NetworkName == "" {
		identity.NetworkName = fallbackNetworkName
	}
	return identity
}

func primaryMACAddress() (string, error) {
	interfaces, err := interfaceLister()
	if err != nil {
		return "", err
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if len(iface.HardwareAddr) == 0 {
			continue
		}
		return normalizeMAC(iface.HardwareAddr.String()), nil
	}

	return "", fmt.Errorf("no active network interface with hardware address")
}

func normalizeMAC(value string) string {
	parts := strings.Split(strings.ToLower(value), ":")
	if len(parts) != 6 {
		return strings.ToLower(value)
	}
	return strings.Join(parts, ":")
}

var interfaceLister = net.Interfaces
