package pgm

import (
	"fmt"
	"logger"
	"net"
	"os"
)

// server transport
func (tp *serverTransport) listerForDatagrams() {

	buf := make([]byte, 1024)
	for {
		n, srcAddr, err := tp.sock.ReadFromUDP(buf)
		if err != nil {
			logger.Errorf("Error reading from UDP connection: %v\n", err)
			continue
		}

		logger.Infof("Received message from %s: %s\n", srcAddr.String(), string(buf[:n]))

		ackAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", srcAddr.IP.String(), tp.protocol.conf.aport))
		if err != nil {
			logger.Errorln(err)
			os.Exit(1)
		}

		// send ack
		_, err = tp.sock.WriteToUDP([]byte("ack from server"), ackAddr)
		if err != nil {
			logger.Errorln(err)
			os.Exit(1)
		}

	}
}

// server protocol
func (pt *serverProtocol) Listen() {
	go pt.transport.listerForDatagrams()
}

func createServerTransport(protocol *serverProtocol) *serverTransport {
	// create server transport
	// server listens for multicast messages on dport and
	// send unicacst ack on aport
	groupAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", protocol.conf.mcast_ipaddr, protocol.conf.dport))
	if err != nil {
		logger.Errorln(err)
		os.Exit(1)
	}
	ifaceIp := getInterfaceIP()
	iface := getInterface(ifaceIp.String())

	// create udp multicast listener
	conn, err := net.ListenMulticastUDP("udp", &iface, groupAddr)
	if err != nil {
		logger.Errorln(err)
		os.Exit(1)
	}

	transport := &serverTransport{
		sock:     conn,
		protocol: protocol,
	}

	return transport
}

func CreateServerProtocol() *serverProtocol {
	protocol := &serverProtocol{
		conf: mcastConf,
	}
	transport := createServerTransport(protocol)

	protocol.transport = transport

	return protocol
}
