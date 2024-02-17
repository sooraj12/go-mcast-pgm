package pgm

import (
	"fmt"
	"net"
	"os"
)

func getInterface() (ifaceInfo *net.Interface, ipaddr string) {
	interfaces, err := net.Interfaces()
	if err != nil {
		fmt.Printf("Error getting interface addresses: %v\n", err)
		os.Exit(1)
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback == 0 && iface.Flags&net.FlagUp != 0 {
			addrs, err := iface.Addrs()
			if err != nil {
				fmt.Println("Error:", err)
				continue
			}

			for _, addr := range addrs {
				ip, ok := addr.(*net.IPNet)
				if !ok {
					continue
				}

				if ip.IP.To4() != nil && !ip.IP.IsLinkLocalUnicast() {
					ifaceInfo = &iface
					ipaddr = ip.IP.String()
					return
				}
			}
		}
	}

	return
}
