package pgm

import "time"

// destination nodes (for client)
const (
	default_air_datarate  = 5000
	default_ack_timeout   = 10
	default_retry_timeout = 1000
)

// pgm
const (
	min_pdu_delay          time.Duration = 10 //msec
	ack_pdu_delay_msec     time.Duration = 500
	minimum_addr_pdu_len   int           = 32
	max_addr_pdu_len       int           = 100
	destination_entry_len  int           = 8
	opt_ts_val_len         int           = 12
	minimun_packet_len     int           = 4
	min_ack_pdu_len        uint16        = 14
	min_ack_info_entry_len uint16        = 24
	ack_pdu_options_len    uint16        = 12
	ack_info_header_len    int           = 16
	data_pdu_header_len    int           = 24
)
