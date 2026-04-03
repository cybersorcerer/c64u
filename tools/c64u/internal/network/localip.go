package network

import (
	"fmt"
	"net"
)

// LocalIP returns the preferred outbound IP address of this machine.
// It does so by opening a UDP "connection" to the given target address
// (no packets are actually sent) and reading the local address chosen
// by the OS.  targetHost should be the C64 Ultimate's IP so the OS
// picks the right network interface.
func LocalIP(targetHost string) (string, error) {
	conn, err := net.Dial("udp", net.JoinHostPort(targetHost, "11002"))
	if err != nil {
		// Fallback: iterate interfaces and return first non-loopback IPv4
		return firstNonLoopback()
	}
	defer conn.Close()

	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || addr.IP == nil {
		return firstNonLoopback()
	}
	return addr.IP.String(), nil
}

func firstNonLoopback() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", fmt.Errorf("cannot list network interfaces: %w", err)
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			if ip4 := ip.To4(); ip4 != nil {
				return ip4.String(), nil
			}
		}
	}
	return "", fmt.Errorf("no usable local IP address found")
}
