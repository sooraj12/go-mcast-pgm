package pgm

import (
	"bytes"
	"encoding/binary"
	"logger"
	"slices"
	"time"
)

func initDestination(config *pgmConfig, dest string, fragmentLen uint16, airDatarate float64,
	retryTimeout time.Duration, ackTimeout time.Duration) (destinationStatus *destination) {

	fragmentAckStatus := map[uint16]bool{}
	for i := uint16(0); i < fragmentLen; i++ {
		fragmentAckStatus[i] = false
	}

	destinationStatus = &destination{
		config:              config,
		dest:                dest,
		completed:           false,
		ack_received:        false,
		last_received_tsecr: 0,
		sent_data_count:     0,
		missed_data_count:   0,
		missed_ack_count:    0,
		fragment_ack_status: fragmentAckStatus,
		air_datarate:        airDatarate,
		retry_timeout:       retryTimeout,
		ack_timeout:         ackTimeout,
		missing_fragments:   &[]uint16{},
	}

	logger.Debugf("TX: Destination(to: %s)", dest)
	logger.Debugf("AirDatarate: %f", destinationStatus.air_datarate)
	logger.Debugf("RetryTimeout: %d", destinationStatus.retry_timeout)
	logger.Debugf("AckTimeout: %d", destinationStatus.ack_timeout)

	return
}

func (d *destination) updateMissedDataCnt() {
	d.missed_data_count += len(*d.missing_fragments)
}

func (d *destination) isCompleted() bool {
	for _, val := range d.fragment_ack_status {
		if !val {
			return false
		}
	}

	return true
}

func (d *destination) updateFragmentAckStatus(missing *[]uint16, seqnohi uint16) {
	// mark all the fragments not in missing as acked
	for i := uint16(0); i < seqnohi+1; i++ {
		if slices.Contains(*missing, i) {
			d.fragment_ack_status[i] = false
		} else {
			d.fragment_ack_status[i] = true
		}
	}

	d.missing_fragments = missing
	// update the transfer status
	d.completed = d.isCompleted()
	// reset the missed ack count
	d.missed_ack_count = 0
}

func (d *destination) isDuplicate(tsecr int64) bool {
	if tsecr == 0 {
		logger.Debugf("TX: Ignore invalid Timestamp Echo Reply for Duplicate Detection")
		return false
	}

	if tsecr == d.last_received_tsecr {
		logger.Debugf("TX: AckPdu from %s is a duplicate", d.dest)
		return true
	}
	d.last_received_tsecr = tsecr
	logger.Debugf("Sender: Updated TSecr to %d", tsecr)
	return false
}

func (d *destination) updateAckTimeout(tsecr int64, tvalue int64) {
	if tsecr == 0 {
		logger.Debugf("TX: Ignore invalid Timestamp Echo Reply for AckTimeout")
		return
	}

	// calculate ack timeout based on RTT and TValue
	deliveryTime := time.Now().UnixMilli() - tsecr
	newAckTimeout := deliveryTime - tvalue
	if newAckTimeout <= 0 {
		logger.Debugf("TX Ignore invalid ackTimeout of %d", newAckTimeout)
		return
	}
	timeout := (d.ack_timeout.Milliseconds() + newAckTimeout) / 2
	d.ack_timeout = time.Duration(timeout)
	logger.Debugf("TX: Updated AckTimeout for %s to %d new_ack_timeout: %d tValue: %d delivery_time: %d", d.dest, d.ack_timeout, newAckTimeout, tvalue, deliveryTime)
}

func (d *destination) updateRetryTimeout(tsecr int64) {
	if tsecr == 0 {
		logger.Debugln("TX: Ignore invalid Timestamp Echo Reply for RetryTimeout")
		return
	}

	retryTimeout := time.Now().UnixMilli() - tsecr
	if retryTimeout <= 0 {
		logger.Debugf("TX: Ignore invalid retry_timeout of %d", retryTimeout)
		return
	}
	retryTimeout = min(retryTimeout, pgmConf.max_retry_timeout.Microseconds())
	retryTimeout = max(retryTimeout, pgmConf.min_retry_timeout.Microseconds())
	retryTimeout = (d.retry_timeout.Milliseconds() + retryTimeout) / 2
	d.retry_timeout = time.Duration(retryTimeout)
	logger.Debugf("TX: Updated retry_timeout for %s to %d new_retry_timeout: %d", d.dest, d.retry_timeout, retryTimeout)
}

func (d *destination) updateAirDatarate(datarate float64) {
	airDatarate := min(datarate, pgmConf.max_datarate)
	airDatarate = max(airDatarate, pgmConf.min_datarate)
	d.air_datarate = round((d.air_datarate + airDatarate) / 2)
	logger.Debugf("TX: Updated air_datarate for %s to %f new_air_datarate: %f", d.dest, d.air_datarate, airDatarate)
}

func (d *destination) log() {
	acked, total := 0, 0
	for _, val := range d.fragment_ack_status {
		if val {
			acked += 1
		}
		total += 1
	}

	logger.Debugf(
		"TX %s: %d/%d air_datarate: %f retry_timeout: %d loss: %d %d/%d missed-ack: %d missing: %d", d.dest,
		acked,
		total,
		d.air_datarate,
		d.retry_timeout,
		100*d.missed_data_count/d.sent_data_count,
		d.missed_data_count,
		d.sent_data_count,
		d.missed_ack_count,
		d.missing_fragments,
	)
}

func (de *destinationEntry) toBuffer() []byte {
	destEntry := bytes.Buffer{}
	entry := destEncoder{
		Destid: ipToInt32(de.dest_ipaddr),
		Seqno:  de.seqno,
	}

	binary.Write(&destEntry, binary.LittleEndian, entry)

	return destEntry.Bytes()
}

func (de *destinationEntry) fromBuffer(data []byte) int {
	if len(data) < destination_entry_len {
		logger.Errorln("RX: DestinationEntry.from_buffer() FAILED with: Message too small")
		return 0
	}

	buf := bytes.Buffer{}
	buf.Write(data)
	var des destEncoder
	err := binary.Read(&buf, binary.LittleEndian, &des)
	if err != nil {
		logger.Errorln(err)
		return 0
	}

	de.dest_ipaddr = ipInt32ToString(des.Destid)
	de.seqno = des.Seqno

	return destination_entry_len
}
