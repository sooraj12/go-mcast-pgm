package pgm

import (
	"fmt"
	"logger"
	"net"
	"os"
)

func (tp *serverTransport) onDataPDU() {}

func (tp *serverTransport) onAddrPDU() {}

// server transport
func (tp *serverTransport) listerForDatagrams() {
	logger.Debugf("socket listens for data in port: %d", tp.protocol.conf.dport)

	buf := make([]byte, 1024)
	for {
		n, srcAddr, err := tp.sock.ReadFromUDP(buf)
		if err != nil {
			logger.Errorf("Error reading from UDP connection: %v\n", err)
			continue
		}
		logger.Debugf("RX Received packet from %s type: %d len:%d", srcAddr.IP.String(), 2, n)
		tp.processPDU(buf)
	}
}

func (tp *serverTransport) processPDU(data []byte) {
	pdu := PDU{}
	pdu.fromBuffer(data)
	pdu.log("RX")
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
		sock:        conn,
		protocol:    protocol,
		rx_ctx_list: &map[int]server{},
	}

	return transport
}

func CreateServerProtocol() *serverProtocol {
	protocol := &serverProtocol{
		conf: mcastConf,
	}
	transport := createServerTransport(protocol)

	protocol.transport = transport
	logger.Debugln("protocol is ready")
	return protocol
}
