package system

import (
	"boops/client"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func applyWindows(ifaces []client.InterfaceInfo, ops Ops) error {
	if _, err := ops.LookPath("netsh"); err != nil {
		return err
	}
	for _, info := range ifaces {
		if err := runChecked(ops, "netsh", "interface", "ipv4", "show", "addresses", "name="+info.Name); err != nil {
			return fmt.Errorf("NIC %s preflight: %w", info.Name, err)
		}
	}
	// Capture the native replay script before any OS changes.
	original, err := ops.Run("netsh", "interface", "ipv4", "dump")
	if err != nil {
		return fmt.Errorf("backup Windows IPv4 settings: %w", err)
	}
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return err
	}
	backup := filepath.Join(os.TempDir(), fmt.Sprintf("boops-network-%x.txt", suffix))
	var commands [][]string
	for _, info := range ifaces {
		gateway := info.Gateway
		if gateway == "" {
			gateway = "none"
		}
		first := info.IPs[0]
		commands = append(commands, []string{"interface", "ipv4", "set", "address", "name=" + info.Name, "source=static", "address=" + first.IP, "mask=" + first.Subnet, "gateway=" + gateway, "store=persistent"})
		for _, ip := range info.IPs[1:] {
			commands = append(commands, []string{"interface", "ipv4", "add", "address", "name=" + info.Name, "address=" + ip.IP, "mask=" + ip.Subnet, "store=persistent"})
		}
		dns := dnsAddresses(info)
		address := "none"
		if len(dns) > 0 {
			address = dns[0]
		}
		register := "none"
		for _, ip := range info.IPs {
			if ip.DNSRegister == 1 {
				register = "primary"
			}
		}
		commands = append(commands, []string{"interface", "ipv4", "set", "dnsservers", "name=" + info.Name, "source=static", "address=" + address, "register=" + register, "validate=no"})
		for i, ip := range dns {
			if i == 0 {
				continue
			}
			commands = append(commands, []string{"interface", "ipv4", "add", "dnsservers", "name=" + info.Name, "address=" + ip, fmt.Sprintf("index=%d", i+1), "validate=no"})
		}
	}
	if err := ops.WriteFile(backup, original, 0600); err != nil {
		return fmt.Errorf("save Windows network backup: %w", err)
	}
	defer ops.Remove(backup)
	for _, args := range commands {
		if err := runChecked(ops, "netsh", args...); err != nil {
			if restore := runChecked(ops, "netsh", "-f", backup); restore != nil {
				return errors.Join(err, fmt.Errorf("restore Windows IPv4 settings: %w", restore))
			}
			return err
		}
	}
	return nil
}
