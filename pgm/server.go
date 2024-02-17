package pgm

import (
	"fmt"
	"log"
	"net"
	"os"
)

type server struct{}

type serverTransport struct {
	protocol     *serverProtocol
	sock         *net.UDPConn
	mcast_ttl    int16
	dport        string
	aport        string
	mcast_ipaddr *net.UDPAddr
	iface        *net.Interface
}

func (tp *serverTransport) listerForDatagrams() {
	fmt.Println("Listening for messages")

	buf := make([]byte, 1024)
	for {
		n, srcAddr, err := tp.sock.ReadFromUDP(buf)
		if err != nil {
			fmt.Printf("Error reading from UDP connection: %v\n", err)
			continue
		}

		fmt.Printf("Received message from %s: %s\n", srcAddr.String(), string(buf[:n]))

		// send ack
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
	groupAddr, err := net.ResolveUDPAddr("udp4", net.JoinHostPort(mcast_ipaddr, dport))
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	// get Interface ip
	iface, _ := getInterface()

	// create udp multicast listener
	conn, err := net.ListenMulticastUDP("udp4", iface, groupAddr)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	transport := &serverTransport{
		mcast_ttl:    mcast_ttl,
		dport:        dport,
		aport:        aport,
		sock:         conn,
		mcast_ipaddr: groupAddr,
		iface:        iface,
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
