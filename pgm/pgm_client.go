package pgm

import (
	"bytes"
	"logger"
	"slices"
	"time"
)

func initClient(tp *clientTransport, dests *[]*nodeInfo, data *[]byte, msid int32, trafficType Traffic) client {
	fragments := fragment(*data, pgmConf.mtu)
	fragmentTxCount := &map[uint16]int{}
	for i := range *fragments {
		(*fragmentTxCount)[uint16(i)] = 0
	}

	dest_list := &[]string{}
	dest_status := &destStatus{}
	for _, val := range *dests {
		*dest_list = append(*dest_list, val.addr)

		(*dest_status)[val.addr] = initDestination(pgmConf, val.addr, uint16(len(*fragments)), val.air_datarate, val.retry_timeout, val.ack_timeout)
	}

	return client{
		message:            data,
		transport:          tp,
		msid:               msid,
		trafficType:        trafficType,
		config:             pgmConf,
		dest_list:          dest_list,
		pdu_delay:          min_pdu_delay,
		tx_datarate:        1,
		dest_status:        dest_status,
		fragments:          fragments,
		event:              make(chan *clientEventChan),
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
		startTimestamp:     time.Now(),
	}
}

func (cli *client) sync() {
	for {
		select {
		case state := <-cli.state:
			cli.currState = state
		case <-cli.retry_timer_chan:
			go func() {
				cli.event <- &clientEventChan{id: RetransmissionTimeout}
			}()
		case <-cli.pdu_timer_chan:
			go func() {
				cli.event <- &clientEventChan{id: PduDelayTimeout}
			}()
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
			case Finished:
				go cli.finished(event)
			}
		}
	}
}

func (cli *client) getSeqnohi() uint16 {
	seqno := uint16(0)
	for key := range *cli.tx_fragments {
		seqno = key
	}
	return seqno
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
			if !dest.fragment_ack_status[uint16(i)] {
				if _, ok := (*unAckList)[uint16(i)]; !ok {
					(*unAckList)[uint16(i)] = &txFragment{
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
		dest.missing_fragments = &[]uint16{}
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

func (cli *client) calcReceivedBytes(fragID uint16) int {
	count := 0
	for key := range *cli.tx_fragments {
		if key > fragID {
			count += pgmConf.mtu
		}
	}

	return count
}

func (cli *client) getFirstReceivedFragID(missing *[]uint16) uint16 {
	for key := range *cli.tx_fragments {
		if slices.Contains(*missing, key) {
			return key
		}
	}

	return 0
}

func (cli *client) calcAirdatarate(remoteIP string, tsecr int64, missing *[]uint16, tvalue int64) (airDatarate float64) {
	if len(*cli.tx_fragments) == 0 {
		return 0 // nothing was sent
	}

	if tsecr != 0 {
		// address pdu was received
		sentBytes := getSentBytes(cli.tx_fragments)
		airDatarate = round(float64(sentBytes*8*1000) / float64(tvalue))
		logger.Debugf("TX: %s measured air-datarate: %f bit/s send-bytes: %d tValue: %d", remoteIP, airDatarate, sentBytes, tvalue)
	} else {
		// address wasn't received
		fragID := cli.getFirstReceivedFragID(missing)
		count := cli.calcReceivedBytes(fragID)
		airDatarate = round(float64(count) * 8 * 1000 / float64(tvalue))
		logger.Debugf("TX: %s measured air-datarate: %f bit/s 1st rx-fragment: %d end-bytes: %d tValue: %d", remoteIP, airDatarate, fragID, count, tvalue)
	}

	return
}

func (cli *client) receivedAllAcks() bool {
	for _, dest := range *cli.dest_status {
		if !dest.ack_received {
			return false
		}
	}

	return true
}

func (cli *client) getDeliveryReport() map[string]interface{} {
	deliveryTime := time.Since(cli.startTimestamp).Milliseconds()
	loss := 100 * (cli.num_sent_data_pdus - len(*cli.fragments)) / len(*cli.fragments)

	status := map[string]interface{}{}
	status["tx_datarate"] = cli.tx_datarate
	status["goodput"] = calcGoodput(cli.startTimestamp, messageLen(cli.fragments))
	status["delivery_time"] = deliveryTime
	status["num_data_pdus"] = len(*cli.fragments)
	status["num_sent_data_pdus"] = cli.num_sent_data_pdus
	status["loss"] = loss

	return status
}

func (cli *client) showDeliveryReport() {
	ackStatus := map[string]map[string]interface{}{}
	for addr, dest := range *cli.dest_status {
		ackStatus[addr] = map[string]interface{}{}
		ackStatus[addr]["ack_status"] = dest.completed
		ackStatus[addr]["ack_timeout"] = dest.ack_timeout
		ackStatus[addr]["retry_timeout"] = dest.retry_timeout
		ackStatus[addr]["retry_count"] = cli.getMaxRetryCount()
		ackStatus[addr]["num_sent"] = dest.sent_data_count
		ackStatus[addr]["air_datarate"] = dest.air_datarate
		ackStatus[addr]["missed"] = dest.missed_data_count
	}

	cli.transport.transmissionFinished(cli.msid, cli.getDeliveryReport(), ackStatus)
}

func (cli *client) getNextTxFragment() uint16 {
	for i, f := range *cli.tx_fragments {
		if !f.sent {
			return i
		}
	}

	return 0
}

func (cli *client) isFinalFragment(seqno uint16) bool {
	lastID := -1
	for key := range *cli.tx_fragments {
		lastID = int(key)
	}

	return int(seqno) == lastID
}

func (cli *client) idle(event *clientEventChan) {
	logger.Debugf("SND | IDLE: %#v", *event)
	// cancel pdu timer and retransmission timer
	cli.cancelRetryTimer()
	cli.cancelPDUTimer()

	switch event.id {
	case Start:
		// init transaction
		cli.initTxn(false)
		// send address pdu
		destEntries := []destinationEntry{}
		for _, val := range *cli.dest_list {
			entry := destinationEntry{
				dest_ipaddr: val,
				seqno:       cli.seqno,
			}

			destEntries = append(destEntries, entry)
		}
		addrPDU := addressPDU{}
		addrPDU.init(Address, uint16(len(*cli.fragments)),
			uint16(len(*cli.tx_fragments)),
			cli.getSeqnohi(),
			cli.msid,
			time.Now().UnixMilli(),
			getInterfaceIP().String(),
			&destEntries,
		)
		cli.seqno += 1
		if cli.trafficType == Message {
			addrPDU.payload = &(*cli.fragments)[0]
			(*cli.tx_fragments)[0].sent = true
			(*cli.tx_fragments)[0].len = len(*addrPDU.payload)
			cli.incNumOfSentDataPDU()
			cli.num_sent_data_pdus += 1
		}
		addrPDU.log("SND")
		var pdu bytes.Buffer
		addrPDU.toBuffer(&pdu)
		pduBytes := pdu.Bytes()
		cli.transport.sendCast(&pduBytes)

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
			cli.state <- SendingData
		}
	}
}

func (cli *client) sendingData(event *clientEventChan) {
	logger.Debugf("SND | SENDING_DATA: %#v", *event)

	if event.id == PduDelayTimeout {
		cli.cancelPDUTimer()

		// send data pdu
		datapdu := dataPDU{}
		seqno := cli.getNextTxFragment()
		data := (*cli.fragments)[seqno]
		datapdu.init(
			uint16(len(*cli.tx_fragments)),
			cli.getSeqnohi(),
			cli.msid,
			seqno,
			cli.cwnd_seqno,
			&data,
		)
		datapdu.log("SND")

		pdu := bytes.Buffer{}
		datapdu.toBuffer(&pdu)
		logger.Debugf("SND | SND DataPdu[%d] len: %d cwnd_seqno: %d", datapdu.seqno, len(*datapdu.data), datapdu.cwndSeqno)
		pduBytes := pdu.Bytes()
		cli.transport.sendCast(&pduBytes)
		(*cli.tx_fragments)[datapdu.seqno].sent = true
		(*cli.tx_fragments)[datapdu.seqno].len = len(*datapdu.data)
		cli.incNumOfSentDataPDU()
		cli.num_sent_data_pdus += 1

		logger.Debugf("SND | SENDING_DATA - start PDU Delay timer with a %d msec timeout", cli.pdu_delay)
		cli.startPDUTimer(cli.pdu_delay)

		if cli.isFinalFragment(datapdu.seqno) {
			logger.Debugf("SND | SENDING_DATA - Change state to SENDING_EXTRA_ADDRESS_PDU")
			cli.state <- SendingExtraAddressPdu
		}
	}
}

func (cli *client) sendingExtraAddr(event *clientEventChan) {
	logger.Debugf("SND | SENDING_EXTRA_ADDRESS_PDU: %#v", event)

	switch event.id {
	case PduDelayTimeout:
		cli.cancelPDUTimer()

		// send address pdu
		destEntries := []destinationEntry{}
		for _, val := range *cli.dest_list {
			entry := destinationEntry{
				dest_ipaddr: val,
				seqno:       cli.seqno,
			}

			destEntries = append(destEntries, entry)
		}
		addrPDU := addressPDU{}
		addrPDU.init(
			ExtraAddress,
			uint16(len(*cli.fragments)),
			uint16(len(*cli.tx_fragments)),
			cli.getSeqnohi(),
			cli.msid,
			time.Now().UnixMilli(),
			getInterfaceIP().String(),
			&destEntries,
		)
		cli.seqno += 1
		if cli.trafficType == Message {
			addrPDU.payload = &(*cli.fragments)[0]
			(*cli.tx_fragments)[0].sent = true
			(*cli.tx_fragments)[0].len = len(*addrPDU.payload)
			cli.incNumOfSentDataPDU()
			cli.num_sent_data_pdus += 1
		}
		addrPDU.log("SND")
		var pdu bytes.Buffer
		addrPDU.toBuffer(&pdu)
		pduBytes := pdu.Bytes()
		cli.transport.sendCast(&pduBytes)

		// start retransmission timer
		timeout := time.Until(cli.retry_timestamp)
		logger.Debugf("SND | start Retransmission timer with %d msec delay", timeout.Milliseconds())
		cli.cancelRetryTimer()
		cli.startRetryTimer(timeout)
		// Change State to WAITING_FOR_ACKS
		logger.Debugln("SND | change state to WAITING_FOR_ACKS")
		cli.state <- WaitingForAcks
	case RetransmissionTimeout:
	}
}

func (cli *client) waitingForAck(event *clientEventChan) {
	logger.Debugf("SND | WAITING_FOR_ACKS: %#v", *event)

	switch event.id {
	case AckPdu:
		eventData := event.data.(clientAckEvent)
		remoteIP := eventData.remoteIP
		infoEntry := eventData.infoEntry

		// check if ack sender is in our destination list
		if !slices.Contains(*cli.dest_list, remoteIP) {
			logger.Debugf("TX: Ignore Ack from %s", remoteIP)
			return
		}
		if destStatus, ok := (*cli.dest_status)[remoteIP]; !ok {
			logger.Debugf("TX: Ignore Ack from %s", remoteIP)
			return
		} else {
			// update ack status of each fragment
			destStatus.updateFragmentAckStatus(infoEntry.missingSeqnos, infoEntry.seqnohi)
			// if this ack pdu with the same timestamp hasn't been received before
			// we update the retry timeout and ack timeout
			if !destStatus.isDuplicate(infoEntry.tsecr) {
				// update retry timeout for the destination
				destStatus.updateRetryTimeout(infoEntry.tsecr)
				// update ack timeout for the destination
				destStatus.updateAckTimeout(infoEntry.tsecr, infoEntry.tsval)
			}
			// we have received an ack for the remote address
			destStatus.ack_received = true

			// update the airdatarate based on the recieved tvalue
			if infoEntry.tsval > 0 {
				airDatarate := cli.calcAirdatarate(remoteIP, infoEntry.tsecr, infoEntry.missingSeqnos, infoEntry.tsval)
				logger.Debugf("TX Measured air-datarate: %f and tvalue: %d sec", airDatarate, infoEntry.tsval)
				// prevent increase of airdatarate in a wrong case
				if airDatarate > cli.tx_datarate {
					logger.Debugf("SND | air-datarate %f is larger than tx-datarate - Update air-datarate to %f", airDatarate, cli.tx_datarate)
					destStatus.updateAirDatarate(cli.tx_datarate)
				} else {
					destStatus.updateAirDatarate(airDatarate)
				}
			}
			destStatus.log()

			// if we have received all the ack pdus we can continue with the next
			// transmission phase sending data pdu retransmissions. if ack pdus are
			// still missing we continue waiting
			if cli.receivedAllAcks() {
				cli.cancelRetryTimer()
				cli.initTxn(false)

				if cli.getMaxRetryCount() > cli.config.max_retry_count {
					logger.Debugln("SND | Max Retransmission Count Reached")
					logger.Debugln("SND | change state to Idle")
					cli.state <- Idle

					// inform transport that message delivery is finished
					cli.showDeliveryReport()
				}

				// send addr pdu
				addrPDU := addressPDU{}
				destEntries := []destinationEntry{}
				for _, val := range *cli.dest_list {
					entry := destinationEntry{
						dest_ipaddr: val,
						seqno:       cli.seqno,
					}

					destEntries = append(destEntries, entry)
				}
				addrPDU.init(Address, uint16(len(*cli.fragments)),
					uint16(len(*cli.tx_fragments)),
					cli.getSeqnohi(),
					cli.msid,
					time.Now().UnixMilli(),
					getInterfaceIP().String(),
					&destEntries,
				)
				cli.seqno += 1

				if cli.trafficType == Message && len(*cli.dest_list) > 0 {
					addrPDU.payload = &(*cli.fragments)[0]
					(*cli.tx_fragments)[0].sent = true
					(*cli.tx_fragments)[0].len = len(*addrPDU.payload)
					cli.incNumOfSentDataPDU()
					cli.num_sent_data_pdus += 1
				}
				addrPDU.log("SND")
				var pdu bytes.Buffer
				addrPDU.toBuffer(&pdu)
				pduBytes := pdu.Bytes()
				cli.transport.sendCast(&pduBytes)

				if len(*cli.dest_list) > 0 {
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
						cli.state <- SendingData
					}
				} else {
					// change state to finished
					cli.cancelRetryTimer()
					logger.Debugln("SND | change state to FINISHED")
					cli.state <- Finished

					// inform transport that message delivery is finished
					cli.showDeliveryReport()
				}
			}
		}

	case RetransmissionTimeout:
	}
}

func (cli *client) finished(event *clientEventChan) {
	logger.Debugf("SND | FINISHED: %#v", *event)
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
