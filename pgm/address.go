package pgm

import (
	"bytes"
	"encoding/binary"
	"logger"
)

// 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |        Length_of_PDU        |    Priority   |MAP|  PDU_Type | 4
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |     Total_Number_of_PDUs    |            Checksum           | 8
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |   Number_of_PDUs_in_Window  |  Highest_Seq_Number_in_Window | 12
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |        Data Offset          |            Reserved           | 16
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |                         Source_ID                           | 20
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |                      Message_ID (MSID)                      | 24
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |                        Expiry_Time                          | 28
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |Count_of_Destination_Entries |   Length_of_Reserved_Field    | 32
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |                                                             | 8 each
// |        List of Destination_Entries (variable length)        |
// |                                                             |
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |                                                             | 12
// /                 Options (variable length)                   /
// |                                                             |
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |                                                             |
// /                  Data (variable length)                     /
// |                                                             |
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//

// 0                   1                   2                   3
// 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |                      Destination ID                         |
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |                  Message_Sequence_Number                    |
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |              Reserved_Field (variable length)               |
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//

func initAddrPDU(total uint16, cwnd uint16, seqnohi uint16, msid int32, tsval int64, srcIP string, dest_list *[]string, seqno int32) *addressPDU {
	addrPDU := &addressPDU{
		pduType:     Address,
		total:       total,
		cwnd:        cwnd,
		seqnohi:     seqnohi,
		msid:        msid,
		expires:     0,
		rsvlen:      0,
		tsval:       tsval,
		srcIP:       srcIP,
		dst_entries: &[]destinationEntry{},
	}

	for _, val := range *dest_list {
		entry := destinationEntry{
			dest_ipaddr: val,
			seqno:       seqno,
		}
		*addrPDU.dst_entries = append(*addrPDU.dst_entries, entry)
	}

	return addrPDU
}

func (addr *addressPDU) length() uint16 {
	return uint16(minimum_addr_pdu_len +
		(destination_entry_len * len(*addr.dst_entries)) +
		opt_ts_val_len + len(*addr.payload))
}

func (addr *addressPDU) toBuffer(b *bytes.Buffer) {

	destEntries := []destEncoder{}
	for _, d := range *addr.dst_entries {
		destEntries = append(destEntries, destEncoder{
			destid: ipToInt32(d.dest_ipaddr),
			seqno:  d.seqno,
		})
	}
	// convert dest entries to fixed length array for encoding
	var entries [1]destEncoder
	copy(entries[:], destEntries[:])

	pdu := addrPDUEncoder{
		length:      addr.length(),
		priority:    0,
		pduType:     uint8(addr.pduType),
		total:       addr.total,
		checksum:    0,
		cwnd:        addr.cwnd,
		seqnohi:     addr.seqnohi,
		offset:      addr.length() - uint16(len(*addr.payload)),
		reserved:    0,
		srcid:       ipToInt32(addr.srcIP),
		msid:        addr.msid,
		expires:     addr.expires,
		dest_len:    uint16(len(*addr.dst_entries)),
		rsvlen:      addr.rsvlen,
		destEntries: entries,
		tsopt:       0,
		l:           12,
		v:           0,
		tsval:       addr.tsval,
	}

	err := binary.Write(b, binary.LittleEndian, pdu)
	if err != nil {
		return
	}

	_, err = b.Write(*addr.payload)
	if err != nil {
		return
	}
}

func (addr *addressPDU) log(state string) {
	logger.Debugf("%s *** Address PDU *************************************************", state)
	logger.Debugf("%s * total:%d cwnd:%d seqnoh:%d srcid:%s msid:%d expires:%d rsvlen:%d", state,
		addr.total,
		addr.cwnd,
		addr.seqnohi,
		addr.srcIP,
		addr.msid,
		addr.expires,
		addr.rsvlen,
	)

	for i, val := range *addr.dst_entries {
		logger.Debugf("%s * dst[%d] dstid:%s seqno:%d", state, i, val.dest_ipaddr, val.seqno)

	}
	logger.Debugf("%s * tsval: %d", state, addr.tsval)
	logger.Debugf("%s ****************************************************************", state)
}
