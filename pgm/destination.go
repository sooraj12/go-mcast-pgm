package pgm

import "logger"

func initDestination(config *pgmConfig, dest string, fragmentLen int, airDatarate int,
	retryTimeout int, ackTimeout int) (destinationStatus *destination) {

	fragmentAckStatus := map[int]bool{}
	for i := 0; i < fragmentLen; i++ {
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
		missing_fragments:   make([]int, 2000),
	}

	logger.Debugf("TX: Destination(to: %s)", dest)
	logger.Debugf("AirDatarate: %d", destinationStatus.air_datarate)
	logger.Debugf("RetryTimeout: %d", destinationStatus.retry_timeout)
	logger.Debugf("AckTimeout: %d", destinationStatus.ack_timeout)

	return
}

func (d *destination) log() {}
