package utils

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os/exec"
	"strings"
)

func CalculateMD5(data []byte) string {
	hash := md5.Sum(data)
	return hex.EncodeToString(hash[:])
}

func EnsureStaticARP(enable bool, ip, mac, iface, role string) error {
	if !enable {
		fmt.Printf("Static ARP disabled for %s\n", role)
		return nil
	}
	if ip == "" || mac == "" || iface == "" {
		return fmt.Errorf("missing ip (%s), mac (%s) or iface (%s) for static ARP", ip, mac, iface)
	}

	cmd := exec.Command("ip", "neigh", "replace", ip, "lladdr", mac, "nud", "permanent", "dev", iface)
	output, err := cmd.CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(output))
		if trimmed != "" {
			return fmt.Errorf("ip neigh replace failed: %v (output: %s)", err, trimmed)
		}
		return fmt.Errorf("ip neigh replace failed: %v", err)
	}

	fmt.Printf("Static ARP configured for %s: %s -> %s via %s\n", role, ip, mac, iface)
	return nil
}
