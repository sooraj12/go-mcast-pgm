package pgm

// transport config
const (
	mcast_ipaddr  = "234.0.0.1"
	mcast_ttl     = 32
	dport         = 22000
	aport         = 20000
	min_bulk_size = 512
)

// destination nodes (for client)
const (
	default_air_datarate  = 5000
	default_ack_timeout   = 10
	default_retry_timeout = 1000
)
