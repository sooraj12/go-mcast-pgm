package pgm

import (
	"net"
	"time"
)

// client
type client struct {
	message            *[]byte
	transport          *clientTransport
	dests              *[]nodeInfo
	msid               int32
	trafficType        Traffic
	config             *pgmConfig
	dest_list          *[]string
	pdu_delay          time.Duration
	tx_datarate        float64
	dest_status        *destStatus
	fragments          *[][]byte
	event              chan clientEvent
	state              chan clientState
	currState          clientState
	tx_fragments       *txFragments
	seqno              int32
	num_sent_data_pdus int
	cwnd_seqno         int
	useMinPDUDelay     bool
	fragmentTxCount    *map[int]int
	retry_timeout      time.Duration
	air_datarate       float64
	retry_timestamp    time.Time
	pdu_timer_chan     <-chan time.Time
	retry_timer_chan   <-chan time.Time
	pdu_timer          *time.Timer
	retry_timer        *time.Timer
}

type txFragment struct {
	sent bool
	len  int
}

type txFragments map[int]txFragment

type destStatus map[string]destination

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
	air_datarate        float64
	retry_timeout       time.Duration
	ack_timeout         time.Duration
	missing_fragments   []int
}

type destinationEntry struct {
	dest_ipaddr string
	seqno       int32
}

type addressPDU struct {
	pduType     pduType
	total       uint16
	cwnd        uint16
	seqnohi     uint16
	msid        int32
	expires     int32
	rsvlen      uint16
	dst_entries *[]destinationEntry
	tsval       int64
	payload     *[]byte
	srcIP       string
}

type destEncoder struct {
	destid int32
	seqno  int32
}

type addrPDUHeaderEncoder struct {
	length   uint16
	priority uint8
	pduType  uint8
	total    uint16
	checksum uint16
	cwnd     uint16
	seqnohi  uint16
	offset   uint16
	reserved uint16
	srcid    int32
	msid     int32
	expires  int32
	dest_len uint16
	rsvlen   uint16
}

type addressPDUOptionsEncoder struct {
	tsopt uint8
	l     uint8
	v     uint16
	tsval int64
}

// client transport
type clientTransport struct {
	protocol  *clientProtocol
	mConn     *net.UDPConn
	uConn     *net.UDPConn
	nodesInfo *nodesInfo

	tx_ctx_list *map[int]client
}

// destination info
type nodeInfo struct {
	air_datarate  float64
	ack_timeout   time.Duration
	retry_timeout time.Duration
	addr          string
}

type nodesInfo map[string]nodeInfo

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

	rx_ctx_list *map[int]server
}

// server transport
type serverProtocol struct {
	transport *serverTransport
	conf      *mcastConfig
}

type PDU struct {
	Len      uint16
	Priority uint8
	PduType  uint8
}

// traffic type
type Traffic int

const (
	Message Traffic = iota
	Bulk
)

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

// pdu types
type pduType uint8

const (
	Address pduType = iota
	Ack
	Data
	ExtraAddress
)
