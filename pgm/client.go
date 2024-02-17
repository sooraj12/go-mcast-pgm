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
	protocol      *clientProtocol
	src_udpaddr   *net.UDPAddr
	mcast_udpaddr *net.UDPAddr
	sock          *net.UDPConn
	mcast_ttl     int16
	dport         string
	aport         string

	tx_ctx_list map[int32]*client
}

func (tp *clientTransport) initSend(data []byte) {
	// decide if single msg or bulk msg and pass to state machine
	tp.sendCast(data)
}

// function which sends multicast messages
func (tp *clientTransport) sendCast(data []byte) {
	tp.sock.Write(data)
}

// listen for ack
func (tp *clientTransport) listenForAck() {
	fmt.Println("Client Listening for ack")

	buf := make([]byte, 1024)
	for {
		n, srcAddr, err := tp.sock.ReadFromUDP(buf)
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
	groupAddr, err := net.ResolveUDPAddr("udp4", net.JoinHostPort(mcast_ipaddr, dport))
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	// get Interface ip
	_, interfaceIp := getInterface()
	fmt.Println(interfaceIp)
	ifaceAddr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf(":%s", aport))
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	// create UDP connection
	conn, err := net.DialUDP("udp", ifaceAddr, groupAddr)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	t = &clientTransport{
		mcast_ttl:     mcast_ttl,
		dport:         dport,
		aport:         aport,
		mcast_udpaddr: groupAddr,
		sock:          conn,
		src_udpaddr:   ifaceAddr,
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
