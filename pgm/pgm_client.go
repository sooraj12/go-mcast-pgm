package pgm

func initClient(tp *clientTransport, dests *[]*nodeInfo, data *[]byte, msid int, trafficType Traffic) *client {
	return &client{
		cli:         tp,
		dests:       dests,
		message:     data,
		msid:        msid,
		trafficType: trafficType,
	}
}
