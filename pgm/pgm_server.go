package pgm

import (
	"logger"
	"math/rand"
	"time"
)

func (srv *server) init(msid int32, remoteIP string, destList *[]string) {
	stateChan := make(chan *severEventChan)

	srv.state = make(chan serverState)
	srv.event = stateChan
	srv.currState = server_Idle
	srv.msid = msid
	srv.remoteIP = remoteIP
	srv.received = &map[int]int{}
	srv.mcastACKTimeout = 0
	srv.fragments = &map[int]*[]byte{}
	srv.dests = destList
}

func (srv *server) sync() {
	for {
		select {
		case state := <-srv.state:
			srv.currState = state
		case event := <-srv.event:
			switch srv.currState {
			case server_Idle:
				go srv.idle(event)
			case server_ReceivingData:
				go srv.receivingData(event)
			case server_SentAck:
				go srv.sentAck(event)
			default:
				go srv.finished(event)
			}
		}
	}
}

func (srv *server) timerSync() {
	for {
		select {
		case <-srv.pduTimerChan:
			srv.event<- &severEventChan{id: server_LastPduTimeout}
		case <-srv.ackTimerChan:
			srv.event<- &severEventChan{id: server_AckPduTimeout}
		default:
		}
	}
}

func (srv *server) saveFragment(seqno int, payload *[]byte){
	if _, ok := (*srv.fragments)[seqno]; !ok{
		(*srv.fragments)[seqno] = payload
	}
	logger.Debugf("RCV | Saved fragment %d with len %d", seqno, len(*payload))
}

func (srv *server) startPDUDelayTimer(d time.Duration){
	if srv.pduTimer != nil {
		srv.pduTimer.Stop()
	}
	srv.pduTimer = time.NewTimer(d)
	srv.pduTimerChan = srv.pduTimer.C
}

func (srv *server) cancelPDUDelayTimer(){
	if srv.pduTimer != nil {
		srv.pduTimer.Stop()
	}
}

func (srv *server) startACKTimer(d time.Duration){
	if srv.ackTimer != nil {
		srv.ackTimer.Stop()
	}
	srv.ackTimer = time.NewTimer(d)
	srv.ackTimerChan = srv.ackTimer.C
}

func (srv *server) cancelACKTimer(){
	if srv.ackTimer != nil {
		srv.ackTimer.Stop()
	}
}

func (srv *server) idle(ev *severEventChan) {
	logger.Debugf("RCV | state_IDLE: %#v", ev.id)

	switch ev.id {
	case server_AddressPDU:
		// initialize the reception phase
		var addrPDU *addressPDU
		addrPDU = ev.data.(*addressPDU)
		srv.startTimestamp = time.Now()
		srv.log()

		if len(*addrPDU.payload) > 0{
			// single mode
			(*srv.received)[0] = len(*addrPDU.payload)
			srv.saveFragment(0, addrPDU.payload)

			// wait random time before sending the ack pdu
			randomNum := rand.Float64()
			ackTimeout := round(randomNum * (float64(ack_pdu_delay_msec) * float64(len(*srv.dests))))
			srv.mcastACKTimeout = time.Duration(time.Duration(ackTimeout) * time.Millisecond)
			logger.Debugf("RCV | start LAST_PDU timer with a %d msec timeout", srv.mcastACKTimeout.Milliseconds())
			srv.startPDUDelayTimer(srv.mcastACKTimeout)
			logger.Debugf("RCV | IDLE - Change state to RECEIVING_DATA")
			srv.state <- server_ReceivingData
		}
	}
}

func (srv *server) receivingData(ev *severEventChan) {
	logger.Debugf("RCV | state_RECEIVING_DATA: %#v", ev.id)
}

func (srv *server) sentAck(ev *severEventChan) {}

func (srv *server) finished(ev *severEventChan) {}

func (srv *server) log(){
	logger.Debugf("RCV +--------------------------------------------------------------+")
	logger.Debugf("RCV | RX Phase                                                     |")
	logger.Debugf("RCV +--------------------------------------------------------------+")
	logger.Debugf("RCV | remote_ipaddr: %s", srv.remoteIP)
	logger.Debugf("RCV | dests: %#v", srv.dests)
	// logger.Debugf("RCV | cwnd: {}", srv.cwnd)
	// logger.Debugf("RCV | total: %d", srv.total)
	// logger.Debugf("RCV | seqnohi: {}", srv.seqnohi)
	// logger.Debugf("RCV | tsVal: {}", srv.ts_val)
	// logger.Debugf("RCV | rx_datarate: {}", srv.rx_datarate)
	logger.Debugf("RCV +--------------------------------------------------------------+")
}
