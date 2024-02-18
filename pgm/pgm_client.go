package pgm

import (
	"logger"
)

func initClient(tp *clientTransport, dests *[]*nodeInfo, data *[]byte, msid int, trafficType Traffic) *client {
	dest_list := &[]string{}
	dest_status := &destStatus{}
	for _, val := range *dests {
		*dest_list = append(*dest_list, val.addr)

		(*dest_status)[val.addr] = initDestination(pgmConf, val.addr, 1, val.air_datarate, val.retry_timeout, val.ack_timeout)
	}

	fragments := fragment(*data, pgmConf.mtu)

	return &client{
		message:     data,
		cli:         tp,
		dests:       dests,
		msid:        msid,
		trafficType: trafficType,
		config:      pgmConf,
		dest_list:   dest_list,
		pdu_delay:   min_pdu_delay,
		tx_datarate: 1,
		dest_status: dest_status,
		fragments:   fragments,
		event:       make(chan clientEvent),
		state:       make(chan clientState),
		currState:   Idle,
	}
}

func (cli *client) sync() {
	select {
	case state := <-cli.state:
		cli.currState = state
	case event := <-cli.event:
		// pass the event to state
		switch cli.currState {
		case Idle:
			cli.idle(event)
		case SendingData:
			cli.sendingData(event)
		case SendingExtraAddressPdu:
			cli.sendingExtraAddr(event)
		case WaitingForAcks:
			cli.waitingForAck(event)
		default:
			cli.finished(event)
		}
	}
}

func (cli *client) idle(event clientEvent) {
	logger.Debugf("SND | IDLE: %d", event)
}

func (cli *client) sendingData(event clientEvent) {}

func (cli *client) sendingExtraAddr(event clientEvent) {}

func (cli *client) waitingForAck(event clientEvent) {}

func (cli *client) finished(event clientEvent) {
	logger.Debugf("SND | WAITING_FOR_ACKS: %v", event)
}

func (cli *client) log(state string) {
	logger.Debugf("%s +--------------------------------------------------------------+", state)
	logger.Debugf("%s | Client State                                                 |", state)
	logger.Debugf("%s +--------------------------------------------------------------+", state)
	logger.Debugf("%s | dest_list: %v", state, *cli.dest_list)
	logger.Debugf("%s | msid: %d", state, cli.msid)
	logger.Debugf("%s | traffic_type: %+v", state, cli.trafficType)
	logger.Debugf("%s | PduDelay: %d msec", state, cli.pdu_delay)
	logger.Debugf("%s | Datarate: %d bit/s", state, cli.tx_datarate)
	logger.Debugf("%s | MinDatarate: %d bit/s", state, cli.config.min_datarate)
	logger.Debugf("%s | MaxDatarate: %d bit/s", state, cli.config.max_datarate)
	logger.Debugf("%s | MaxIncreasePercent: %f %%", state, cli.config.max_increase*100)
	for key, val := range *cli.dest_status {
		logger.Debugf("%s | fragmentsAckStatus[%s]: %#v", state, key, val.fragment_ack_status)
	}
	logger.Debugf("%s +--------------------------------------------------------------+", state)
}
