package pgm

import (
	"fmt"
	"net"
	"os"
)

func getInterface(ipaddr string) (ifaceInfo net.Interface) {
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

				if ip.IP.String() == ipaddr {
					ifaceInfo = iface
					return
				}
			}
		}
	}

	return
}
