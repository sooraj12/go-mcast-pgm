package pgm

import (
	"encoding/binary"
	"logger"
	"math"
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

func fragment(data []byte, mtu int) *[][]byte {
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
	return &fragments
}

func getCwnd(cwndList *[]map[string]float64, airDatarate float64, defaultAirDatarate float64) float64 {
	for _, val := range *cwndList {
		if airDatarate <= val["datarate"] {
			return val["cwnd"]
		}
	}

	return defaultAirDatarate
}

func min(x, y float64) float64 {
	if x > y {
		return y
	}
	return x
}

func max(x, y float64) float64 {
	if x < y {
		return y
	}
	return x
}

func minInt64(x, y int64) int64 {
	if x > y {
		return y
	}
	return x
}

func maxInt64(x, y int64) int64 {
	if x < y {
		return y
	}
	return x
}

func round(val float64) float64 {
	return math.Round(val*100) / 100
}

func ipToInt32(ip string) int32 {
	ipAddr := net.ParseIP(ip)
	ip4 := ipAddr.To4()
	n := binary.LittleEndian.Uint32(ip4)
	return int32(n)
}

func ipInt32ToString(n int32) string {
	// Create a slice of 4 bytes
	ip4 := make([]byte, 4)

	// Convert the int32 value to a slice of bytes
	// using big-endian byte order
	binary.LittleEndian.PutUint32(ip4, uint32(n))

	// Convert the slice of bytes to an IP type
	ip := net.IP(ip4)

	// Convert the IP type to a string
	return ip.String()
}

func getSentBytes(fragments *txFragments) int {
	count := 0
	for _, val := range *fragments {
		count = count + val.len
	}

	return count
}
