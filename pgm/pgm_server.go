package pgm

import (
	"bytes"
	"logger"
	"math/rand"
	"time"
)

func (srv *server) init(msid int32, remoteIP string, tp *serverTransport) {
	stateChan := make(chan *severEventChan)

	srv.state = make(chan serverState)
	srv.event = stateChan
	srv.currState = server_Idle
	srv.msid = msid
	srv.remoteIP = remoteIP
	srv.received = &map[int]int{}
	srv.mcastACKTimeout = 0
	srv.fragments = &map[uint16]*[]byte{}
	srv.dests = &[]string{}
	srv.rxDatarate = 0
	srv.transport = tp
	srv.total = 0
	srv.cwnd = 0
	srv.seqnohi = 0
	srv.tsval = 0
	srv.tvalue = 0
	srv.ackRetryCount = 0
	srv.maxAddrPDULen = 0
	srv.cwndSeqno = 0
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
			srv.event <- &severEventChan{id: server_LastPduTimeout}
		case <-srv.ackTimerChan:
			srv.event <- &severEventChan{id: server_AckPduTimeout}
		default:
		}
	}
}

func (srv *server) saveFragment(seqno uint16, payload *[]byte) {
	if _, ok := (*srv.fragments)[seqno]; !ok {
		(*srv.fragments)[seqno] = payload
	}
	logger.Debugf("RCV | Saved fragment %d with len %d", seqno, len(*payload))
}

func (srv *server) startPDUDelayTimer(d time.Duration) {
	if srv.pduTimer != nil {
		srv.pduTimer.Stop()
	}
	srv.pduTimer = time.NewTimer(d)
	srv.pduTimerChan = srv.pduTimer.C
}

func (srv *server) cancelPDUDelayTimer() {
	if srv.pduTimer != nil {
		srv.pduTimer.Stop()
	}
}

func (srv *server) startACKTimer(d time.Duration) {
	if srv.ackTimer != nil {
		srv.ackTimer.Stop()
	}
	srv.ackTimer = time.NewTimer(d)
	srv.ackTimerChan = srv.ackTimer.C
}

func (srv *server) cancelACKTimer() {
	if srv.ackTimer != nil {
		srv.ackTimer.Stop()
	}
}

func (srv *server) calcAckPDUTimeout() time.Duration {
	datarate := srv.rxDatarate
	if datarate == 0 {
		datarate = default_air_datarate
	}
	messageLen := int64(max_addr_pdu_len)
	timeout := pgmConf.rtt_extra_delay
	factor := time.Duration(len(*srv.dests)) * ack_pdu_delay_msec
	ratePerBits := time.Duration(messageLen * 8 * 1000 / int64(datarate))
	timeout = timeout + factor + ratePerBits
	logger.Debugf("RX AckPduTimeout: %d num_dests: %d", timeout, len(*srv.dests))
	return timeout
}

func (srv *server) updateMaxAddrPDULen(pduLen uint16) {
	if pduLen > srv.maxAddrPDULen {
		srv.maxAddrPDULen = pduLen
	}
}

func (srv *server) getMissingFragments() *[]uint16 {
	missed := []uint16{}
	var i uint16
	for i = 0; i < srv.seqnohi; i++ {
		if _, ok := (*srv.fragments)[i]; !ok {
			missed = append(missed, i)
		}
	}

	return &missed
}

func (srv *server) calculateTvalue() time.Duration {
	deltatime := time.Since(srv.startTimestamp)
	tvalue := deltatime
	return tvalue
}

func (srv *server) calcRemainingTime(remaining int, datarate float64) time.Duration {
	if datarate == 0 {
		datarate = default_air_datarate
	}

	msec := round(float64(1000*remaining*8) / datarate)
	logger.Infof("RCV | Remaining Time %f msec - Payload %d bytes - AirDatarate: %f bit/s", msec, remaining, datarate)
	return time.Duration(msec * float64(time.Millisecond))
}

func (srv *server) updateRxDatarate(seqno uint16) {
	var datarate float64
	var nBytes int

	if srv.total > 1 {
		remaining := srv.getRemainingFragments(seqno)
		nBytes = (int(srv.cwnd) - remaining) * pgmConf.mtu
	} else {
		for _, val := range *srv.received {
			nBytes += val
		}
	}
	// calculate avg datarate since first received data pdu
	datarate = calcGoodput(srv.startTimestamp, nBytes)
	// weight new datarate with old datarate
	if srv.rxDatarate == 0 {
		srv.rxDatarate = datarate
	}
	datarate = avgDatarate(srv.rxDatarate, datarate)
	// update the stored datarate
	srv.rxDatarate = min(datarate, pgmConf.max_datarate)
	srv.rxDatarate = max(datarate, pgmConf.min_datarate)

	logger.Debugf("RCV | %s Updated RxDatarate to %f bit/s", getInterfaceIP().String(), srv.rxDatarate)
}

func (srv *server) getRemainingFragments(seqno uint16) int {
	remaining := 0
	if srv.total <= 0 {
		logger.Debugf("RCV | Remaining fragment: 0 - Total number of PDUs is unkown")
		return 0
	}
	// calculate the remaining number of pdus based on the total number of
	// pdus in the current window and the already received number of pdus
	remainingWindowNum := int(srv.cwnd) - len(*srv.received)
	// calculate the remaining number of puds based on the sequence number
	// of the current received pdu and the highest sequence number of the current window
	remainingWindowSeqno := int(srv.seqnohi) - int(seqno)
	// the smallest number will become the remaining number of pdus
	remaining = minInt(remainingWindowNum, remainingWindowSeqno)
	remaining = maxInt(remaining, 0)
	logger.Debugf("RCV | Remaining: %d windowNum: %d window_seqno: %d", remaining, remainingWindowNum, remainingWindowSeqno)
	return remaining
}

func (srv *server) sendACK(seqnohi uint16, tsval int64, tvalue int64, missed *[]uint16) {
	ackPDU := ackPDU{}
	ackPDU.init(seqnohi, srv.remoteIP, srv.msid, tsval, tvalue, missed)

	var pdu bytes.Buffer
	ackPDU.toBuffer(&pdu)

	logger.Debugf("RCV | %s send AckPdu to %s of len %d with ts_ecr: %d tvalue: %d seqnohi: %d missing: %#v", getInterfaceIP().String(),
		srv.remoteIP,
		len(pdu.Bytes()),
		tsval,
		tvalue,
		seqnohi,
		*missed,
	)

	srv.transport.sendUDPTo(pdu.Bytes(), srv.remoteIP)
}

func (srv *server) idle(ev *severEventChan) {
	logger.Debugf("RCV | state_IDLE: %#v", ev.id)

	switch ev.id {
	case server_AddressPDU:
		// initialize the reception phase
		var addrPDU *addressPDU
		addrPDU = ev.data.(*addressPDU)
		dests := addrPDU.getDestList()

		srv.dests = &dests
		srv.total = addrPDU.total
		srv.startTimestamp = time.Now()
		srv.cwnd = addrPDU.cwnd
		srv.seqnohi = addrPDU.seqnohi
		srv.tsval = addrPDU.tsval
		srv.updateMaxAddrPDULen(addrPDU.length())
		srv.log()

		if len(*addrPDU.payload) > 0 {
			// single mode
			(*srv.received)[0] = len(*addrPDU.payload)
			srv.saveFragment(0, addrPDU.payload)

			// wait random time before sending the ack pdu
			randomNum := rand.Float64()
			ackTimeout := round(randomNum * (float64(ack_pdu_delay_msec) * float64(len(*srv.dests))))
			srv.mcastACKTimeout = time.Duration(ackTimeout * float64(time.Millisecond))
			logger.Debugf("RCV | start LAST_PDU timer with a %d msec timeout", srv.mcastACKTimeout.Milliseconds())
			srv.startPDUDelayTimer(srv.mcastACKTimeout)
			logger.Debugf("RCV | IDLE - Change state to RECEIVING_DATA")
			srv.state <- server_ReceivingData
		} else {
			// TrafficMode.Bulk - start last pdu timer
			remainingBytes := (int(addrPDU.cwnd) + 1) * pgmConf.mtu
			var timeout time.Duration
			if srv.rxDatarate == 0 {
				timeout = srv.calcRemainingTime(remainingBytes, default_air_datarate)
			} else {
				timeout = srv.calcRemainingTime(remainingBytes, srv.rxDatarate)
			}
			randomNum := rand.Float64()
			ackTimeout := round(randomNum * (float64(len(*srv.dests)) * float64(ack_pdu_delay_msec)))
			srv.mcastACKTimeout = time.Duration(ackTimeout * float64(time.Millisecond))
			timeout = timeout + srv.mcastACKTimeout
			// at multicast communication the ack pdu will be randomly delayed to avoid collision
			logger.Debugf("RCV | start LAST_PDU timer with a %d msec timeout", timeout.Milliseconds())
			srv.startPDUDelayTimer(timeout)
			// Change state to RECEIVING_DATA
			logger.Debugln("RCV | IDLE - change state to RECEIVING_DATA")
			srv.state <- server_ReceivingData
		}
	}
}

func (srv *server) receivingData(ev *severEventChan) {
	logger.Debugf("RCV | state_RECEIVING_DATA: %#v", ev.id)

	switch ev.id {
	case server_DataPDU:
		srv.cancelPDUDelayTimer()
		datapdu := ev.data.(dataPDU)
		// save the received fragment
		(*srv.received)[int(datapdu.seqno)] = len(*datapdu.data)
		srv.saveFragment(datapdu.seqno, datapdu.data)
		// update the transmission window parameters
		srv.cwnd = datapdu.cwnd
		srv.seqnohi = datapdu.seqnohi
		srv.cwndSeqno = datapdu.cwndSeqno
		// calculate rx datarate
		srv.updateRxDatarate(datapdu.seqno)

		if srv.cwnd > 0 {
			// calculate how many data pdus are still awaited
			remaining := srv.getRemainingFragments(datapdu.seqno)
			logger.Debugf("RCV | %s | Received DataPDU[%d] Remaining: %d RxDatarate: %f bit/s", getInterfaceIP().String(), datapdu.seqno, remaining, srv.rxDatarate)
			// start last pdu timer
			remaining += 1 // additional extra address pdu
			timeout := srv.calcRemainingTime(remaining*pgmConf.mtu, srv.rxDatarate)
			// at multicast the ack pdu will be randomly delayed to avoid collision
			randomNum := rand.Float64()
			ackTimeout := round(randomNum * (float64(len(*srv.dests)) * float64(ack_pdu_delay_msec)))
			srv.mcastACKTimeout = time.Duration(ackTimeout * float64(time.Millisecond))
			timeout = timeout + srv.mcastACKTimeout
			logger.Debugf("RCV | start LAST_PDU timer with a %d msec timeout", timeout)
			srv.startPDUDelayTimer(timeout)
		}
	case server_LastPduTimeout:
		srv.cancelPDUDelayTimer()

		tvalue := srv.calculateTvalue()
		srv.tvalue = tvalue.Milliseconds() - srv.mcastACKTimeout.Milliseconds()
		missed := srv.getMissingFragments()
		srv.sendACK(srv.seqnohi, srv.tsval, srv.tvalue, missed)
		timeout := time.Duration(srv.calcAckPDUTimeout() * time.Millisecond)
		logger.Debugf("RCV | Start ACK_PDU timer with a %d msec timeout", timeout.Milliseconds())
		srv.startACKTimer(timeout)
		// reset rx phase
		srv.cwnd = 0
		// change state to sent ack
		logger.Debugf("RCV | RECEIVING_DATA - Change state to SENT_ACK")
		srv.state <- server_SentAck
	}
}

func (srv *server) sentAck(ev *severEventChan) {
	logger.Debugf("RCV | state_SENT_ACK: %d", ev.id)

	switch ev.id {
	case server_AddressPDU:
		srv.cancelACKTimer()
		addrPDU := ev.data.(*addressPDU)

		if !addrPDU.isRecepient(getInterfaceIP().String()) {
			// change state to finished
			logger.Debugln("SND | change satte to FINISHED")
			srv.state <- server_Finished
			message := bytes.Buffer{}
			reassemble(srv.fragments, &message)
			srv.transport.messageReceived(srv.msid, message.Bytes(), srv.remoteIP)
		}

	case server_AckPduTimeout:
	}
}

func (srv *server) finished(ev *severEventChan) {
	logger.Debugf("RCV | state_FINISHED: %d", ev.id)
	srv.cancelPDUDelayTimer()
	srv.cancelACKTimer()
}

func (srv *server) log() {
	logger.Debugf("RCV +--------------------------------------------------------------+")
	logger.Debugf("RCV | RX Phase                                                     |")
	logger.Debugf("RCV +--------------------------------------------------------------+")
	logger.Debugf("RCV | remote_ipaddr: %s", srv.remoteIP)
	logger.Debugf("RCV | dests: %#v", srv.dests)
	logger.Debugf("RCV | cwnd: %d", srv.cwnd)
	logger.Debugf("RCV | total: %d", srv.total)
	logger.Debugf("RCV | seqnohi: %d", srv.seqnohi)
	logger.Debugf("RCV | tsVal: %d", srv.tsval)
	logger.Debugf("RCV | rx_datarate: %f", srv.rxDatarate)
	logger.Debugf("RCV +--------------------------------------------------------------+")
}
