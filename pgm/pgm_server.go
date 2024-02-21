package pgm

import "logger"

func (srv *server) init(msid int32) {
	stateChan := make(chan *severEventChan)

	srv.state = make(chan serverState)
	srv.event = stateChan
	srv.currState = server_Idle
	srv.msid = msid
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

func (srv *server) timerSync() {}

func (srv *server) idle(ev *severEventChan) {
	logger.Debugf("RCV | state_IDLE: %#v", ev.id)
}

func (srv *server) receivingData(ev *severEventChan) {}

func (srv *server) sentAck(ev *severEventChan) {}

func (srv *server) finished(ev *severEventChan) {}
