package pgm

import (
	"fmt"
	"log"
	"net"
	"os"
)

type server struct{}

type serverTransport struct {
	protocol *serverProtocol
	mConn    *net.UDPConn
	aConn    *net.UDPConn
}

func (tp *serverTransport) listerForDatagrams() {
	fmt.Println("Listening for messages")

	buf := make([]byte, 1024)
	for {
		n, srcAddr, err := tp.mConn.ReadFromUDP(buf)
		if err != nil {
			fmt.Printf("Error reading from UDP connection: %v\n", err)
			continue
		}

		fmt.Printf("Received message from %s: %s\n", srcAddr.String(), string(buf[:n]))

		fmt.Println(srcAddr.IP.String())
		ackAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", srcAddr.IP.String(), aport))
		if err != nil {
			log.Fatal(err)
			os.Exit(1)
		}

		// send ack
		size, err := tp.aConn.WriteToUDP([]byte("ack from server"), ackAddr)
		if err != nil {
			log.Fatal(err)
			os.Exit(1)
		}

		fmt.Printf("Sent ack %d\n", size)
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
		log.Fatal(err)
		os.Exit(1)
	}

	unicastAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", src_ipaddr, aport))
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	iface := getInterface(src_ipaddr)

	// create udp multicast listener
	conn, err := net.ListenMulticastUDP("udp", &iface, groupAddr)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	ackConn, err := net.DialUDP("udp", unicastAddr, unicastAddr)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	transport := &serverTransport{
		mConn: conn,
		aConn: ackConn,
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
