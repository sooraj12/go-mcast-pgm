package pgm

import "net"

// client
type client struct {
	message     *[]byte
	cli         *clientTransport
	dests       *[]*nodeInfo
	msid        int
	trafficType Traffic
	config      *pgmConfig
	dest_list   *[]string
	pdu_delay   int
	tx_datarate int
	dest_status *destStatus
	fragments   [][]byte
	event       chan clientEvent
	state       chan clientState
	currState   clientState
}

type destStatus map[string]*destination

type destination struct {
	config              *pgmConfig
	dest                string
	completed           bool
	ack_received        bool
	last_received_tsecr int
	sent_data_count     int
	missed_data_count   int
	missed_ack_count    int
	fragment_ack_status map[int]bool
	air_datarate        int
	retry_timeout       int
	ack_timeout         int
	missing_fragments   []int
}

// client transport
type clientTransport struct {
	protocol  *clientProtocol
	mConn     *net.UDPConn
	uConn     *net.UDPConn
	nodesInfo *nodesInfo

	tx_ctx_list *map[int]*client
}

// client protocol
type clientProtocol struct {
	transport *clientTransport
	conf      *mcastConfig
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
	conf      *mcastConfig
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

// client states
type clientState int

const (
	Idle clientState = iota
	SendingData
	SendingExtraAddressPdu
	WaitingForAcks
	Finished
)

// client events
type clientEvent int

const (
	Start clientEvent = iota
	AckPdu
	PduDelayTimeout
	RetransmissionTimeout
)
