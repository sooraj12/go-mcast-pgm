package pgm

import (
	"fmt"
	"logger"
	"net"
	"os"
)

func (tp *serverTransport) onDataPDU(data []byte) {
	datapdu := dataPDU{}
	datapdu.fromBuffer(&data)

	remoteIP := datapdu.srcIP
	msid := datapdu.msid
	key := uniqKey{remoteIP, msid}

	if val, ok := (*tp.rx_ctx_list)[key]; ok {
		datapdu.log("RCV")

		// pass event to state machine
		go val.sync()
		go val.timerSync()

		val.event <- &severEventChan{id: server_DataPDU, data: datapdu}
	}
}

func (tp *serverTransport) onAddrPDU(data []byte) {
	addressPdu := addressPDU{}
	addressPdu.fromBuffer(data)

	var eventID serverEvent
	if addressPdu.pduType == Address {
		eventID = server_AddressPDU
	} else {
		eventID = server_ExtraAddressPDU
	}

	remoteIP := addressPdu.srcIP
	msid := addressPdu.msid
	key := uniqKey{remoteIP, msid}

	// # On receipt of an Address_PDU the receiving node shall first check whether
	// # the Address_PDU with the same tuple "Source_ID, MSID" has already been received
	serverEvent := &severEventChan{id: eventID, data: &addressPdu}
	if val, ok := (*tp.rx_ctx_list)[key]; !ok {
		// If its own ID is not in the list of Destination_Entries,
		// the receiving node shall discard the Address_PDU
		if !addressPdu.isRecepient(getInterfaceIP().String()) && false {
			return
		}
		addressPdu.log("RCV")

		state := server{}
		state.init(msid, remoteIP, tp)

		(*tp.rx_ctx_list)[key] = state

		// start state machine
		go state.sync()
		go state.timerSync()
		// send event to state machine
		state.event <- serverEvent
	} else {
		// There is already a RxContext -> forward message to state machine
		addressPdu.log("RCV")

		// start state machine
		go val.sync()
		go val.timerSync()
		// send event to state machine
		val.event <- serverEvent
	}
}

func (tp *serverTransport) sendUDPTo(pdu []byte, remoteIP string) {
	IP, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", remoteIP, mcastConf.aport))
	if err != nil {
		logger.Errorln(err)
	}

	tp.sock.WriteToUDP(pdu, IP)
}

// server transport
func (tp *serverTransport) listerForDatagrams() {
	logger.Debugf("socket listens for data in port: %d", tp.protocol.conf.dport)

	buf := make([]byte, 1400)
	for {
		n, srcAddr, err := tp.sock.ReadFromUDP(buf)
		b := make([]byte, n)
		copy(b, buf)
		if err != nil {
			logger.Errorf("Error reading from UDP connection: %v\n", err)
			continue
		}
		logger.Debugf("RX Received packet from %s type: %d len:%d", srcAddr.IP.String(), 2, n)
		go tp.processPDU(b)
	}
}

func (tp *serverTransport) processPDU(data []byte) {
	pdu := PDU{}
	pdu.fromBuffer(data)
	pdu.log("RX")

	switch pdu.PduType {
	case uint8(Address):
		tp.onAddrPDU(data)
	case uint8(Data):
		tp.onDataPDU(data)
	case uint8(ExtraAddress):
		tp.onAddrPDU(data)
	default:
		logger.Infof("Received unkown PDU type %d", pdu.PduType)
	}
}

func (tp *serverTransport) messageReceived(msid int32, message []byte, remoteIP string) {
	logger.Debugf("RCV | Received message %d of len %d from %s", msid, len(message), remoteIP)
	tp.protocol.messageReceived(message)
}

// server protocol
func (pt *serverProtocol) Listen() {
	go pt.transport.listerForDatagrams()
}

func (pt *serverProtocol) messageReceived(message []byte) {
	logger.Infof("New message of length %d received, send to application", len(message))
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
		rx_ctx_list: &map[uniqKey]server{},
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
