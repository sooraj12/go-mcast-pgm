package pgm

import (
	"net"
	"time"
)

// client
type client struct {
	message            *[]byte
	transport          *clientTransport
	msid               int32
	trafficType        Traffic
	config             *pgmConfig
	dest_list          *[]string
	pdu_delay          time.Duration
	tx_datarate        float64
	dest_status        *destStatus
	fragments          *[][]byte
	event              chan *clientEventChan
	state              chan clientState
	currState          clientState
	tx_fragments       *txFragments
	seqno              int32
	num_sent_data_pdus int
	cwnd_seqno         uint16
	useMinPDUDelay     bool
	fragmentTxCount    *map[uint16]int
	retry_timeout      time.Duration
	air_datarate       float64
	retry_timestamp    time.Time
	pdu_timer_chan     <-chan time.Time
	retry_timer_chan   <-chan time.Time
	pdu_timer          *time.Timer
	retry_timer        *time.Timer
	startTimestamp     time.Time
}

type clientEventChan struct {
	id   clientEvent
	data interface{}
}

type clientAckEvent struct {
	remoteIP  string
	infoEntry ackInfoEntry
}

type txFragment struct {
	sent bool
	len  int
}

type txFragments map[uint16]*txFragment

type destStatus map[string]*destination

type destination struct {
	config              *pgmConfig
	dest                string
	completed           bool
	ack_received        bool
	last_received_tsecr int64
	sent_data_count     int
	missed_data_count   int
	missed_ack_count    int
	fragment_ack_status map[uint16]bool
	air_datarate        float64
	retry_timeout       time.Duration
	ack_timeout         time.Duration
	missing_fragments   *[]uint16
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
	Destid int32
	Seqno  int32
}

type addrPDUHeaderEncoder struct {
	Length   uint16
	Priority uint8
	PduType  uint8
	Total    uint16
	Checksum uint16
	Cwnd     uint16
	Seqnohi  uint16
	Offset   uint16
	Reserved uint16
	Srcid    int32
	Msid     int32
	Expires  int32
	Dest_len uint16
	Rsvlen   uint16
}

type addressPDUOptionsEncoder struct {
	Tsopt TsOption
	L     uint8
	V     uint16
	Tsval int64
}

// client transport
type clientTransport struct {
	protocol  *clientProtocol
	mConn     *net.UDPConn
	uConn     *net.UDPConn
	nodesInfo *nodesInfo

	tx_ctx_list *map[int32]client
}

// destination info
type nodeInfo struct {
	air_datarate  float64
	ack_timeout   time.Duration
	retry_timeout time.Duration
	addr          string
}

type nodesInfo map[string]*nodeInfo

// client protocol
type clientProtocol struct {
	transport *clientTransport
	conf      *mcastConfig
}

// server
type server struct {
	state           chan serverState
	event           chan *severEventChan
	currState       serverState
	msid            int32
	remoteIP        string
	dests           *[]string
	mcastACKTimeout time.Duration
	startTimestamp  time.Time
	received        *map[int]int
	fragments       *map[uint16]*[]byte
	pduTimerChan    <-chan time.Time
	ackTimerChan    <-chan time.Time
	pduTimer        *time.Timer
	ackTimer        *time.Timer
	rxDatarate      float64
	transport       *serverTransport

	total         uint16
	cwnd          uint16
	seqnohi       uint16
	tsval         int64
	tvalue        int64
	ackRetryCount int
	maxAddrPDULen uint16
}

type severEventChan struct {
	id   serverEvent
	data interface{}
}

// server protocol
type serverTransport struct {
	protocol *serverProtocol
	sock     *net.UDPConn

	rx_ctx_list *map[uniqKey]server
}

// server transport
type serverProtocol struct {
	transport *serverTransport
	conf      *mcastConfig
}

type ackPDU struct {
	srcIP       string // ip of ack emitter
	infoEntries *[]ackInfoEntry
}

type ackInfoEntry struct {
	missingSeqnos *[]uint16
	seqnohi       uint16
	remoteIP      string // ip of addr pdu sender(remote ip addr)
	msid          int32
	tsval         int64
	tsecr         int64
}

type ackPDUEncoder struct {
	Length     uint16
	Priority   uint8
	PduType    pduType
	Map        uint16
	CheckSum   uint16
	SrcID      int32 // ip of ack emitter
	AckInfoLen uint16
}

type ackInfoEntryEncoder struct {
	Length     uint16
	Reserved   uint16
	Seqnohi    uint16
	RemoteID   int32 // ip of addr pdu sender(remote ip addr)
	Msid       int32
	MissingLen uint16
}

type ackInfoEntryOptionsEncoder struct {
	Tsval int64
	Tsopt TsOption
	L     uint8
	V     uint16
	Tsecr int64
}

type dataPDU struct {
	cwnd      uint16
	seqnohi   uint16
	cwndSeqno uint16
	seqno     uint16
	srcIP     string
	msid      int32
	data      *[]byte
}

type dataPDUEncoder struct {
	Length    uint16
	Priority  uint8
	PduType   uint8
	Cwnd      uint16
	Seqnohi   uint16
	Seqno     uint16
	Checksum  uint16
	CwndSeqno uint16
	Reserved  uint16
	srcID     int32
	Msid      int32
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

type serverEvent int

const (
	server_AddressPDU serverEvent = iota
	server_ExtraAddressPDU
	server_DataPDU
	server_LastPduTimeout
	server_AckPduTimeout
)

type serverState int

const (
	server_Idle serverState = iota
	server_ReceivingData
	server_SentAck
	server_Finished
)

type uniqKey struct {
	remoteIP string
	msid     int32
}

type TsOption uint8

const (
	Tsval TsOption = iota
	TsEcr
)
