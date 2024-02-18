package pgm

import (
	"fmt"
	"logger"
	"net"
	"os"
)

type server struct{}

type serverTransport struct {
	protocol *serverProtocol
	sock    *net.UDPConn
}

func (tp *serverTransport) listerForDatagrams() {

	buf := make([]byte, 1024)
	for {
		n, srcAddr, err := tp.sock.ReadFromUDP(buf)
		if err != nil {
			logger.Errorf("Error reading from UDP connection: %v\n", err)
			continue
		}

		logger.Infof("Received message from %s: %s\n", srcAddr.String(), string(buf[:n]))

		ackAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", srcAddr.IP.String(), aport))
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

type serverProtocol struct {
	transport *serverTransport
}

func (pt *serverProtocol) Listen() {
	go pt.transport.listerForDatagrams()
}

func createServerTransport() *serverTransport {
	// create server transport
	// server listens for multicast messages on dport and
	// send unicacst ack on aport
	groupAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", mcast_ipaddr, dport))
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
		sock: conn,
	}

	return transport
}

func CreateServerProtocol() *serverProtocol {
	transport := createServerTransport()
	protocol := &serverProtocol{
		transport: transport,
	}
	transport.protocol = protocol

	return protocol
}
