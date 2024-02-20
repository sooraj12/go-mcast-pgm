package pgm

import (
	"bytes"
	"logger"
	"time"
)

func initClient(tp *clientTransport, dests *[]nodeInfo, data *[]byte, msid int32, trafficType Traffic) client {
	dest_list := &[]string{}
	dest_status := &destStatus{}
	for _, val := range *dests {
		*dest_list = append(*dest_list, val.addr)

		(*dest_status)[val.addr] = initDestination(pgmConf, val.addr, 1, val.air_datarate, val.retry_timeout, val.ack_timeout)
	}

	fragments := fragment(*data, pgmConf.mtu)
	fragmentTxCount := &map[int]int{}
	for i := range *fragments {
		(*fragmentTxCount)[i] = 0
	}

	return client{
		message:            data,
		transport:          tp,
		dests:              dests,
		msid:               msid,
		trafficType:        trafficType,
		config:             pgmConf,
		dest_list:          dest_list,
		pdu_delay:          min_pdu_delay,
		tx_datarate:        1,
		dest_status:        dest_status,
		fragments:          fragments,
		event:              make(chan clientEvent),
		state:              make(chan clientState),
		currState:          Idle,
		tx_fragments:       &txFragments{},
		seqno:              0,
		num_sent_data_pdus: 0,
		cwnd_seqno:         0,
		useMinPDUDelay:     true,
		fragmentTxCount:    fragmentTxCount,
		retry_timeout:      0,
		air_datarate:       pgmConf.default_datarate,
		retry_timestamp:    time.Now(),
		pdu_timer_chan:     make(chan time.Time),
		retry_timer_chan:   make(chan time.Time),
	}
}

func (cli *client) sync() {
	for {
		select {
		case state := <-cli.state:
			cli.currState = state
		case event := <-cli.event:
			// pass the event to state
			switch cli.currState {
			case Idle:
				go cli.idle(event)
			case SendingData:
				go cli.sendingData(event)
			case SendingExtraAddressPdu:
				go cli.sendingExtraAddr(event)
			case WaitingForAcks:
				go cli.waitingForAck(event)
			default:
				go cli.finished(event)
			}
		}
	}
}

func (cli *client) timerSync() {
	for {
		select {
		case <-cli.retry_timer_chan:
			cli.event <- RetransmissionTimeout
		case <-cli.pdu_timer_chan:
			cli.event <- PduDelayTimeout
		// fixme : remove this costly default,
		// but then the channel becomes blocking
		default:
		}
	}
}

func (cli *client) getSeqnohi() uint16 {
	seqno := 0
	for key := range *cli.tx_fragments {
		seqno = key
	}
	return uint16(seqno)
}

func (cli *client) incNumOfSentDataPDU() {
	for _, val := range *cli.dest_list {
		if dest, ok := (*cli.dest_status)[val]; ok {
			dest.sent_data_count += 1
		}

	}
}

func (cli *client) minAirDatarate() (minVal float64) {
	minVal = 0
	for _, dest := range *cli.dest_status {
		if minVal == 0 {
			minVal = dest.air_datarate
		} else if dest.air_datarate < minVal {
			minVal = dest.air_datarate
		}
	}

	if minVal == 0 {
		minVal = float64(cli.config.mtu)
	}

	return
}

func (cli *client) unAckFragments(cwnd float64) (unAckList *txFragments) {
	unAckList = &txFragments{}
	curr := 0
	for i, fr := range *cli.fragments {
		for _, dest := range *cli.dest_status {
			if !dest.fragment_ack_status[i] {
				if _, ok := (*unAckList)[i]; !ok {
					(*unAckList)[i] = txFragment{
						sent: false,
						len:  len(fr),
					}
					curr += len(fr)

					if float64(curr) >= cwnd {
						logger.Debugf("TX unacked_fragments: %#v cwnd: %f curr: %d", *unAckList, cwnd, curr)

					}
				}
			}
		}
	}
	logger.Debugf("TX unacked_fragments: %+v", unAckList)
	return
}

func (cli *client) maxRetryTimeout() (maxTimeout time.Duration) {
	maxTimeout = 0
	for _, dest := range *cli.dest_status {
		if maxTimeout == 0 {
			maxTimeout = dest.retry_timeout
		} else if dest.retry_timeout > maxTimeout {
			maxTimeout = dest.retry_timeout
		}
	}

	if maxTimeout == 0 {
		maxTimeout = cli.config.max_retry_timeout
	}

	return
}

func (cli *client) getMaxRetryCount() int {
	count := 0
	for _, c := range *cli.fragmentTxCount {
		if c > count {
			count += 1
		}
	}
	return count - 1
}

func (cli *client) maxAckTimeout() (timeout time.Duration) {
	timeout = 0
	for _, dest := range *cli.dest_status {
		if timeout == 0 {
			timeout = dest.ack_timeout
		} else if dest.ack_timeout > timeout {
			timeout = dest.ack_timeout
		}
	}

	if timeout == 0 {
		timeout = cli.config.max_ack_timeout
	}

	return
}

func (cli *client) initTxn(timeoutOccured bool) {
	var _cwnd float64
	remainingBytes := 0
	retryTimeout := time.Duration(0)
	ackTimeout := time.Duration(0)
	airDatarate := float64(0)

	// loss detection for each destination
	for _, dest := range *cli.dest_status {
		dest.updateMissedDataCnt()
		dest.missing_fragments = []int{}
	}

	// address pdu initialization
	// initialize list of destinations, all destinaitons which haven't acked
	// all fragments are retransmitted to the destinations event tho some of them
	// would have go some of the fragments.
	cli.dest_list = &[]string{}
	for addr, dest := range *cli.dest_status {
		if dest.completed = dest.isCompleted(); dest.completed {
			dest.ack_received = true
		} else {
			dest.ack_received = false
			*cli.dest_list = append(*cli.dest_list, addr)
		}
	}

	// increment the seq no of the transmission window. receiver should know
	// if we have started a new transmission phase
	cli.cwnd_seqno += 1

	// tx_datarate control and window management
	// init tx_datarate with min airdatarate of all the destinations
	cli.tx_datarate = cli.minAirDatarate()
	logger.Debugf("TX-CTX: Measured AirDatarate: %f", cli.tx_datarate)

	// calculate the current window based on the calculated tx_datarate
	_cwnd = getCwnd(&cli.config.cwnds, cli.tx_datarate, default_air_datarate)
	// when sending with the min pdu delay, use the default window size
	if cli.useMinPDUDelay || timeoutOccured {
		_cwnd = cli.config.initial_cwnd
	}

	// calculate the number of fragments in the current window based on the cwnd
	cli.tx_fragments = cli.unAckFragments(_cwnd)
	// update the number of transmission for each fragment
	for i := range *cli.tx_fragments {
		(*cli.fragmentTxCount)[i] += 1
	}

	old_tx_datarate := cli.minAirDatarate()
	remaining := len(*cli.tx_fragments) * cli.config.mtu
	if !timeoutOccured && remaining > 0 {
		duration := round((float64(remaining) * 8 * 1000) / cli.tx_datarate)
		// The txDatarate is increased by the configured number of inflightBytes.
		// This shall ensure that the txDatarate gets higher if more AirDatarate
		// is available. The limitation to the maximum number of inflightBytes
		// shall prevent packet loss due buffer overflow if the AirDatarate didn´t increase */
		logger.Debugf("TX-CTX: Increased txDatarate: %f", cli.tx_datarate)
		logger.Debugf("TX-CTX: Limit TxDatarate to %f", old_tx_datarate*(1+cli.config.max_increase))
		cli.tx_datarate = round(((float64(remaining) + cli.config.inflight_bytes) * 8 * 1000) / duration)
		cli.tx_datarate = min(cli.tx_datarate, old_tx_datarate*(1+cli.config.max_increase))
		logger.Debugf("TX-CTX: Increased txDatarate to %f", cli.tx_datarate)
	}

	// at full speed the tx_datarate is set to the maximum configured tx_datarate
	if cli.useMinPDUDelay {
		cli.tx_datarate = cli.config.max_datarate
		cli.useMinPDUDelay = false
	}

	// correct the data rate to be within the boundaries
	cli.tx_datarate = max(cli.tx_datarate, cli.config.min_datarate)
	cli.tx_datarate = min(cli.tx_datarate, cli.config.max_datarate)

	// pdu delay control
	// update pdu delay timeout based on the current tx_datarate
	cli.pdu_delay = time.Duration(round((1000 * float64(cli.config.mtu) * 8) / cli.tx_datarate))
	cli.pdu_delay = time.Duration(max(float64(cli.pdu_delay), float64(min_pdu_delay)))

	// retransmission timeout
	if cli.trafficType == Message { // for single messages
		// when sending a single message the minimum pdu delay can be used
		cli.pdu_delay = min_pdu_delay

		// retransmission timeout is based on the maximum timeout that was
		// previously measured for on of the destination nodes
		retryTimeout = cli.maxRetryTimeout()
		retryTimeout = time.Duration(min(float64(retryTimeout), float64(cli.config.max_retry_timeout)))
		retryTimeout = time.Duration(max(float64(retryTimeout), float64(cli.config.min_retry_timeout)))

		maxRetryCount := cli.getMaxRetryCount()
		for i := 0; i < maxRetryCount; i++ {
			retryTimeout = retryTimeout * 2
		}
		if maxRetryCount == 0 {
			retryTimeout = time.Duration(float64(retryTimeout) * (1 + float64(cli.config.max_increase)))
		}
		logger.Debugf("TX-CTX: Message: Set RetryTimeout to %d at retrycount of %d", retryTimeout, maxRetryCount)

		cli.retry_timestamp = time.Now().Add(time.Millisecond * retryTimeout)
		cli.retry_timeout = retryTimeout
		cli.air_datarate = 0 // not used
	} else { // for bulk messages
		// The retransmission timeout is based on an estimation of the airDatarate.
		// This estimation takes into account that the airDatarate can have a large
		// decrease. This amount of decrease can be configure. e.g. in a worst case
		// scenario the datarate can be 50% smaller due modulation change of the waveform. */
		airDatarate = round(cli.minAirDatarate() - (1 - cli.config.max_decrease))
		// calculate the retransmission timeout based on the remaining bytes to send
		remainingBytes = len(*cli.tx_fragments) * cli.config.mtu
		// retransmission timeout takes into account that an extra addres pdu has to be sent
		remainingBytes += cli.config.mtu
		// calculate the time interval it takes to send data to the reciever
		retryTimeout = time.Duration(round((float64(remainingBytes) * 8 * 1000) / airDatarate))
		// At multicast communication the retransmission timeout must take into account that
		// multiple time-sliced ACK_PDUs have to be sent.
		retryTimeout = time.Duration(float64(retryTimeout) + float64(len(*cli.dest_list))*float64(ack_pdu_delay_msec))
		// ack timeout is the maximum value of destinations
		ackTimeout = cli.maxAckTimeout()

		// limit retransmission timeout
		retryTimeout = time.Duration(max(float64(retryTimeout), float64(cli.config.min_retry_timeout)))
		retryTimeout = time.Duration(min(float64(retryTimeout), float64(cli.config.max_retry_timeout)))

		cli.retry_timestamp = time.Now().Add(time.Millisecond * time.Duration(retryTimeout))
		cli.retry_timeout = retryTimeout
		logger.Debugf("TX-CTX Bulk: Set RetryTimeout %d to for AirDatarate of %f and ackTimeout of %d", cli.retry_timeout, airDatarate, ackTimeout)
		logger.Debugf("TX Init() tx_datarate: %f air_datarate: %f retry_timeout: %d ack_timeout: %d", cli.tx_datarate, airDatarate, cli.retry_timeout, ackTimeout)
	}

	// update statistics
	logger.Debugf("TX +--------------------------------------------------------------+")
	logger.Debugf("SND | TX Phase                                                     |")
	logger.Debugf("SND |--------------------------------------------------------------+")
	logger.Debugf("SND | dest_list: %+v", *cli.dest_list)
	logger.Debugf("SND | cwnd: %f", _cwnd)
	logger.Debugf("SND | seqnohi: %d", cli.getSeqnohi())
	logger.Debugf("SND | tx_fragments: %#v", *cli.tx_fragments)
	ack_recv_status := map[string]bool{}
	for addr, dest := range *cli.dest_status {
		if !dest.completed {
			ack_recv_status[addr] = dest.ack_received
		}
	}
	logger.Debugf("SND | AckRecvStatus: %#v", ack_recv_status)
	logger.Debugf("SND | OldTxDatarate: %f", old_tx_datarate)
	logger.Debugf("SND | NewTxDatarate: %f", cli.tx_datarate)
	logger.Debugf("SND | AirDatarate: %f", airDatarate)
	logger.Debugf("SND | AckTimeout: %d", ackTimeout)
	logger.Debugf("SND | RetryTimeout: %d", retryTimeout)
	logger.Debugf("SND | InflightBytes: %f", cli.config.inflight_bytes)
	logger.Debugf("SND | RemainingBytes: %d", remainingBytes)
	logger.Debugf("SND | RetryCount: %d Max: %d", cli.getMaxRetryCount(), cli.config.max_retry_count)
	logger.Debugf("TX +--------------------------------------------------------------+")
}

func (cli *client) startRetryTimer(timeout time.Duration) {
	if cli.retry_timer != nil {
		cli.retry_timer.Stop()
	}
	cli.retry_timer = time.NewTimer(timeout)
	cli.retry_timer_chan = cli.retry_timer.C
}

func (cli *client) cancelRetryTimer() {
	if cli.retry_timer != nil {
		cli.retry_timer.Stop()
	}
}

func (cli *client) startPDUTimer(timeout time.Duration) {
	if cli.pdu_timer != nil {
		cli.pdu_timer.Stop()
	}
	cli.pdu_timer = time.NewTimer(timeout)
	cli.pdu_timer_chan = cli.pdu_timer.C
}

func (cli *client) cancelPDUTimer() {
	if cli.pdu_timer != nil {
		cli.pdu_timer.Stop()
	}
}

func (cli *client) idle(event clientEvent) {
	logger.Debugf("SND | IDLE: %d", event)
	// cancel pdu timer and retransmission timer
	cli.cancelRetryTimer()
	cli.cancelPDUTimer()

	switch event {
	case Start:
		// init transaction
		cli.initTxn(false)
		// send address pdu
		addrPDU := initAddrPDU(uint16(len(*cli.fragments)),
			uint16(len(*cli.tx_fragments)),
			cli.getSeqnohi(),
			cli.msid,
			time.Now().UnixMilli(),
			getInterfaceIP().String(),
			cli.dest_list,
			cli.seqno,
		)
		cli.seqno += 1
		if cli.trafficType == Message {
			addrPDU.payload = &(*cli.fragments)[0]
			fragment := (*cli.tx_fragments)[0]
			fragment.sent = true
			fragment.len = len(*addrPDU.payload)
			cli.incNumOfSentDataPDU()
			cli.num_sent_data_pdus += 1
		}
		addrPDU.log("SND")
		var pdu bytes.Buffer
		addrPDU.toBuffer(&pdu)
		cli.transport.sendCast(pdu.Bytes())

		// start timers
		if cli.trafficType == Message {
			timeout := time.Until(cli.retry_timestamp)
			cli.cancelRetryTimer()
			cli.startRetryTimer(timeout)
			logger.Debugf("SND | start Retransmission timer with %d msec delay", timeout.Milliseconds())
			cli.state <- WaitingForAcks
			logger.Debugln("SND | changed state to WAITING_FOR_ACKS")
		} else {
			logger.Debugf("SND | IDLE - start PDU Delay timer with a %d msec timeout", min_pdu_delay)
			cli.startPDUTimer(min_pdu_delay)
			logger.Debugf("SND | IDLE - Change state to SENDING_DATA")
		}
	}
}

func (cli *client) sendingData(event clientEvent) {
	logger.Debugf("SND | SENDING_DATA: %d", event)
}

func (cli *client) sendingExtraAddr(event clientEvent) {
	logger.Debugf("SND | SENDING_EXTRA_ADDRESS_PDU: %d", event)
}

func (cli *client) waitingForAck(event clientEvent) {
	logger.Debugf("SND | WAITING_FOR_ACKS: %v", event)
}

func (cli *client) finished(event clientEvent) {
	logger.Debugf("SND | FINISHED: %v", event)
}

func (cli *client) log(state string) {
	logger.Debugf("%s +--------------------------------------------------------------+", state)
	logger.Debugf("%s | Client State                                                 |", state)
	logger.Debugf("%s +--------------------------------------------------------------+", state)
	logger.Debugf("%s | dest_list: %v", state, *cli.dest_list)
	logger.Debugf("%s | msid: %d", state, cli.msid)
	logger.Debugf("%s | traffic_type: %+v", state, cli.trafficType)
	logger.Debugf("%s | PduDelay: %d msec", state, cli.pdu_delay)
	logger.Debugf("%s | Datarate: %f bit/s", state, cli.tx_datarate)
	logger.Debugf("%s | MinDatarate: %f bit/s", state, cli.config.min_datarate)
	logger.Debugf("%s | MaxDatarate: %f bit/s", state, cli.config.max_datarate)
	logger.Debugf("%s | MaxIncreasePercent: %f %%", state, cli.config.max_increase*100)
	for key, val := range *cli.dest_status {
		logger.Debugf("%s | fragmentsAckStatus[%s]: %#v", state, key, val.fragment_ack_status)
	}
	logger.Debugf("%s +--------------------------------------------------------------+", state)
}
