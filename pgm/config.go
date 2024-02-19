package pgm

import "time"

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
	mtu               int
	inflight_bytes    float64
	default_datarate  float64
	min_datarate      float64
	max_datarate      float64
	max_increase      float64
	max_decrease      float64
	initial_cwnd      float64
	cwnds             []map[string]float64
	max_retry_timeout time.Duration
	min_retry_timeout time.Duration
	max_retry_count   int
	max_ack_timeout   time.Duration
}

var pgmConf *pgmConfig = &pgmConfig{
	mtu:               1024,
	inflight_bytes:    2000,
	default_datarate:  10000,
	min_datarate:      250,
	max_datarate:      160000,
	max_increase:      0.25,
	max_decrease:      0.75,
	initial_cwnd:      5000,
	max_retry_timeout: 240000,
	min_retry_timeout: 250,
	max_retry_count:   3,
	max_ack_timeout:   120000,
	cwnds: []map[string]float64{
		{"datarate": 3000, "cwnd": 3000},
		{"datarate": 6000, "cwnd": 5000}, // 3000 < x < 6000
		{"datarate": 10000, "cwnd": 10000},
		{"datarate": 30000, "cwnd": 15000},
		{"datarate": 60000, "cwnd": 25000},
		{"datarate": 140000, "cwnd": 50000},
	},
}
