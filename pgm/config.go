package pgm

// multicast config
type mcastConfig struct {
	mcast_ipaddr  string
	mcast_ttl     int
	dport         int
	aport         int
	min_bulk_size int
}

var mcastConf *mcastConfig = &mcastConfig{
	mcast_ipaddr:  "234.0.0.1",
	mcast_ttl:     32,
	dport:         22000,
	aport:         20000,
	min_bulk_size: 512,
}

// pgm config
type pgmConfig struct {
	mtu          int
	min_datarate int
	max_datarate int
	max_increase float32
}

var pgmConf *pgmConfig = &pgmConfig{
	mtu:          1024,
	min_datarate: 250,
	max_datarate: 160000,
	max_increase: 0.25,
}
