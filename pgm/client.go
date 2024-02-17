package pgm

import (
	"fmt"
	"log"
	"net"
	"os"
)

type client struct {
	message []byte
	_cli    *clientTransport
}

// func (cl *client) start() {}

type clientTransport struct {
	protocol *clientProtocol
	mConn    *net.UDPConn
	uConn    *net.UDPConn

	tx_ctx_list map[int32]*client
}

func (tp *clientTransport) initSend(data []byte) {
	// decide if single msg or bulk msg and pass to state machine
	tp.sendCast(data)
}

// function which sends multicast messages
func (tp *clientTransport) sendCast(data []byte) {
	tp.mConn.Write(data)
}

// listen for ack
func (tp *clientTransport) listenForAck() {
	fmt.Println("Client Listening for ack")

	buf := make([]byte, 1024)
	for {
		n, srcAddr, err := tp.uConn.ReadFromUDP(buf)
		if err != nil {
			fmt.Printf("Error reading from UDP connection: %v\n", err)
			continue
		}
		fmt.Printf("Received message from %s: %s\n", srcAddr.String(), string(buf[:n]))
	}
}

type clientProtocol struct {
	transport *clientTransport
}

func (cp *clientProtocol) SendMessage(data []byte) {
	cp.transport.initSend(data)
}

func createClientTransport() (t *clientTransport) {
	// create client transport
	// client will send multicast messages over dport and
	// wait for unicast acks over aport
	groupAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", mcast_ipaddr, dport))
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	// get Interface ip
	unicastAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", aport))
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	// create UDP connection
	multicastConn, err := net.DialUDP("udp", &net.UDPAddr{IP: net.ParseIP(src_ipaddr)}, groupAddr)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	unicastConnection, err := net.ListenUDP("udp", unicastAddr)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	t = &clientTransport{
		mConn: multicastConn,
		uConn: unicastConnection,
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

	return
}
