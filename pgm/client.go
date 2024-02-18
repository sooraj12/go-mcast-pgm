package pgm

import (
	"fmt"
	"logger"
	"math/rand"
	"net"
	"os"
)

// client transport
func (tp *clientTransport) generateMSID() int {
	for i := 0; i < 1000; i++ {
		msid := rand.Intn(10000000)
		if _, ok := (*tp.tx_ctx_list)[msid]; !ok {
			return msid
		}
	}
	return -1
}

func (tp *clientTransport) initSend(data []byte, destIPS []string, trafficType Traffic) {
	// generate unique message-id
	msid := tp.generateMSID()
	if msid == -1 {
		logger.Errorln("send_message() FAILED with: No msid found")
		return
	}

	// convert IPv4 addresses from string format into 32bit values
	dests := []*nodeInfo{}
	for _, ip := range destIPS {
		entry := &nodeInfo{}
		entry.addr = ip
		if node, ok := (*tp.nodesInfo)[ip]; !ok {
			entry.air_datarate = default_air_datarate
			entry.ack_timeout = default_ack_timeout
			entry.retry_timeout = default_retry_timeout
		} else {
			n := *node
			entry.air_datarate = n.air_datarate
			entry.ack_timeout = n.ack_timeout
			entry.retry_timeout = n.retry_timeout
		}

		dests = append(dests, entry)
	}

	// initialize a new state
	state := initClient(tp, &dests, &data, msid, trafficType)

	(*tp.tx_ctx_list)[msid] = state
}

func (tp *clientTransport) sendMessage(data []byte, destIPS []string) {
	// decide if single msg or bulk msg
	if len(data) < tp.min_bulk_size {
		tp.initSend(data, destIPS, Message)
	} else {
		tp.initSend(data, destIPS, Bulk)
	}
}

// function which sends multicast messages
func (tp *clientTransport) sendCast(data []byte) {
	tp.mConn.Write(data)
}

// listen for ack
func (tp *clientTransport) listenForAck() {
	logger.Infof("socket listening for ack on: %s", tp.mConn.LocalAddr().String())
	buf := make([]byte, 1024)
	for {
		n, srcAddr, err := tp.uConn.ReadFromUDP(buf)
		if err != nil {
			continue
		}
		logger.Infof("Received message from %s: %s\n", srcAddr.String(), string(buf[:n]))
	}
}

// client protocol
func (cp *clientProtocol) SendMessage(data []byte, destIPS []string) {
	logger.Debugf("SND | sending message of len %d before encoding", len(data))
	cp.transport.sendMessage(data, destIPS)
}

func createClientTransport() (t *clientTransport) {
	// create client transport
	// client will send multicast messages over dport and
	// wait for unicast acks over aport
	groupAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", mcast_ipaddr, dport))
	if err != nil {
		logger.Errorln(err)
		os.Exit(1)
	}

	// get Interface ip
	unicastAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", aport))
	if err != nil {
		logger.Errorln(err)
		os.Exit(1)
	}

	ifaceIp := getInterfaceIP()
	// create UDP connection
	multicastConn, err := net.DialUDP("udp", &net.UDPAddr{IP: net.ParseIP(ifaceIp.String())}, groupAddr)
	if err != nil {
		logger.Errorln(err)
		os.Exit(1)
	}
	logger.Debugln("UDP socket is ready")

	unicastConnection, err := net.ListenUDP("udp", unicastAddr)
	if err != nil {
		logger.Errorln(err)
		os.Exit(1)
	}

	t = &clientTransport{
		mConn:         multicastConn,
		uConn:         unicastConnection,
		min_bulk_size: min_bulk_size,
		nodesInfo:     &nodesInfo{},
		tx_ctx_list:   &map[int]*client{},
	}

	// listen for ack from servers in the background
	go t.listenForAck()

	return
}

func CreateClientProtocol() (protocol *clientProtocol) {
	transport := createClientTransport()
	protocol = &clientProtocol{
		transport: transport,
	}
	transport.protocol = protocol
	logger.Debugln("multicast protocol is ready")

	return
}
