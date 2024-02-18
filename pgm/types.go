package pgm

import "net"

// client
type client struct {
	message     *[]byte
	cli         *clientTransport
	dests       *[]*nodeInfo
	msid        int
	trafficType Traffic
	// config
}

// client transport
type clientTransport struct {
	protocol      *clientProtocol
	mConn         *net.UDPConn
	uConn         *net.UDPConn
	min_bulk_size int
	nodesInfo     *nodesInfo

	tx_ctx_list *map[int]*client
}

// client protocol
type clientProtocol struct {
	transport *clientTransport
}

// server
type server struct{}

// server protocol
type serverTransport struct {
	protocol *serverProtocol
	sock     *net.UDPConn
}

// server transport
type serverProtocol struct {
	transport *serverTransport
}

// traffic type
type Traffic int

const (
	Message Traffic = iota
	Bulk
)

// destination info
type nodeInfo struct {
	air_datarate  int
	ack_timeout   int
	retry_timeout int
	addr          string
}

type nodesInfo map[string]*nodeInfo
