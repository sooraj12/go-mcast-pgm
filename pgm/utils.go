package pgm

import (
	"bytes"
	"encoding/binary"
	"logger"
	"math"
	"net"
	"os"
	"time"

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

func reassemble(fragments *map[uint16]*[]byte, b *bytes.Buffer) {
	for _, f := range *fragments {
		b.Write(*f)
	}
}

func getCwnd(cwndList *[]map[string]float64, airDatarate float64, defaultAirDatarate float64) float64 {
	for _, val := range *cwndList {
		if airDatarate <= val["datarate"] {
			return val["cwnd"]
		}
	}

	return defaultAirDatarate
}

func min[k minmax](x, y k) k {
	if x > y {
		return y
	}
	return x
}

func max[k minmax](x, y k) k {
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

func calcGoodput(ts time.Time, totalBytes int) float64 {
	var datarate float64
	d := time.Since(ts).Milliseconds()
	if d > 0 {
		datarate = float64(totalBytes) * 8 * 1000 / float64(d)
	}

	logger.Infof("RCV | Datarate(): %f Received bytes: %d Interval: %d", datarate, totalBytes, d)

	return round(datarate)
}

func messageLen(fr *[][]byte) (count int) {
	count = 0
	for _, val := range *fr {
		count += len(val)
	}
	return
}

func avgDatarate(old float64, new float64) float64 {
	avg := old * 0.50 * new * 0.50
	logger.Infof("RCV | Avg_datarate() old: %f bit/s new: %f bit/s avg: %f bit/s", old, new, avg)
	return round(avg)
}
