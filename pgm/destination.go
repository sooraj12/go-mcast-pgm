package pgm

import (
	"logger"
	"time"
)

func initDestination(config *pgmConfig, dest string, fragmentLen int, airDatarate float64,
	retryTimeout time.Duration, ackTimeout time.Duration) (destinationStatus destination) {

	fragmentAckStatus := map[int]bool{}
	for i := 0; i < fragmentLen; i++ {
		fragmentAckStatus[i] = false
	}

	destinationStatus = destination{
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
		missing_fragments:   []int{},
	}

	logger.Debugf("TX: Destination(to: %s)", dest)
	logger.Debugf("AirDatarate: %f", destinationStatus.air_datarate)
	logger.Debugf("RetryTimeout: %d", destinationStatus.retry_timeout)
	logger.Debugf("AckTimeout: %d", destinationStatus.ack_timeout)

	return
}

func (d *destination) updateMissedDataCnt() {
	d.missed_data_count += len(d.missing_fragments)
}

func (d *destination) isCompleted() bool {
	for _, val := range d.fragment_ack_status {
		if !val {
			return false
		}
	}

	return true
}
