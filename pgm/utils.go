package pgm

import (
	"logger"
	"net"
	"os"

	"github.com/jackpal/gateway"
)

func getInterface(ipaddr string) (ifaceInfo net.Interface) {
	interfaces, err := net.Interfaces()
	if err != nil {
		logger.Errorf("Error getting interface addresses: %v\n", err)
		os.Exit(1)
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback == 0 && iface.Flags&net.FlagUp != 0 {
			addrs, err := iface.Addrs()
			if err != nil {
				logger.Errorln("Error:", err)
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

func getInterfaceIP() net.IP {
	ifaceIP, err := gateway.DiscoverInterface()
	if err != nil {
		logger.Errorln(err)
		os.Exit(1)
	}

	return ifaceIP
}

func fragment(data []byte, mtu int) [][]byte {
	var fragments [][]byte
	curr := 0
	for curr < len(data) {
		if len(data[curr:]) > mtu {
			fragments = append(fragments, data[curr:curr+mtu])
			curr += mtu
		} else {
			fragments = append(fragments, data[curr:])
			curr += len(data[curr:])
		}
	}
	return fragments
}
