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

func (addr *addressPDU) init(total uint16, cwnd uint16, seqnohi uint16, msid int32, tsval int64, srcIP string, destEntries *[]destinationEntry) {
	addr.pduType = Address
	addr.total = total
	addr.cwnd = cwnd
	addr.seqnohi = seqnohi
	addr.msid = msid
	addr.expires = 0
	addr.rsvlen = 0
	addr.tsval = tsval
	addr.srcIP = srcIP
	addr.dst_entries = destEntries
}

func (addr *addressPDU) length() uint16 {
	return uint16(minimum_addr_pdu_len +
		(destination_entry_len * len(*addr.dst_entries)) +
		opt_ts_val_len + len(*addr.payload))
}

func (addr *addressPDU) toBuffer(b *bytes.Buffer) {

	destEntries := bytes.Buffer{}
	for _, d := range *addr.dst_entries {
		destEntries.Write(d.toBuffer())
	}

	pduHeader := addrPDUHeaderEncoder{
		Length:   addr.length(),
		Priority: 0,
		PduType:  uint8(addr.pduType),
		Total:    addr.total,
		Checksum: 0,
		Cwnd:     addr.cwnd,
		Seqnohi:  addr.seqnohi,
		Offset:   addr.length() - uint16(len(*addr.payload)),
		Reserved: 0,
		Srcid:    ipToInt32(addr.srcIP),
		Msid:     addr.msid,
		Expires:  addr.expires,
		Dest_len: uint16(len(*addr.dst_entries)),
		Rsvlen:   addr.rsvlen,
	}

	pduOptions := addressPDUOptionsEncoder{
		Tsopt: Tsval,
		L:     12,
		V:     0,
		Tsval: addr.tsval,
	}

	err := binary.Write(b, binary.LittleEndian, pduHeader)
	if err != nil {
		return
	}

	b.Write(destEntries.Bytes())

	err = binary.Write(b, binary.LittleEndian, pduOptions)
	if err != nil {
		return
	}

	_, err = b.Write(*addr.payload)
	if err != nil {
		return
	}
}

func (addr *addressPDU) fromBuffer(data []byte) {
	if len(data) < minimum_addr_pdu_len {
		logger.Errorln("AddressPdu.from_buffer() FAILED with: Message too small")
		return
	}

	headerBuf := bytes.Buffer{}
	headerBuf.Write(data[:minimum_addr_pdu_len])

	var pduHeader addrPDUHeaderEncoder
	// read fixed header
	err := binary.Read(&headerBuf, binary.LittleEndian, &pduHeader)
	if err != nil {
		logger.Errorln(err)
		return
	}

	if len(data) < int(pduHeader.Offset) {
		logger.Errorln("RX: AddressPdu.from_buffer() FAILED with: Message to small")
		return
	}

	if (len(data) - minimum_addr_pdu_len) < (int(pduHeader.Dest_len) * destination_entry_len) {
		logger.Errorln("RX: AddressPdu.from_buffer() FAILED with: Message to small")
		return
	}

	// read destination entries
	numEntries := 0
	nBytes := minimum_addr_pdu_len
	destEntries := []destinationEntry{}
	for numEntries < int(pduHeader.Dest_len) {
		if (nBytes + destination_entry_len + int(pduHeader.Reserved)) > int(pduHeader.Offset) {
			logger.Debugln("RX: AddressPdu.from_buffer() FAILED with: Invalid DestinationEntry")
			return
		}

		entry := destinationEntry{}
		n := entry.fromBuffer(data[nBytes : nBytes+destination_entry_len])
		if n < 0 {
			logger.Errorln("RX: AddressPdu.from_buffer() FAILED with: Invalid DestinationEntry")
			return
		}

		nBytes += n
		numEntries += 1
		destEntries = append(destEntries, entry)
	}

	optBuf := bytes.Buffer{}
	optBuf.Write(data[nBytes : nBytes+opt_ts_val_len])
	var pduOptions addressPDUOptionsEncoder
	// read additional options
	err = binary.Read(&optBuf, binary.LittleEndian, &pduOptions)
	if err != nil {
		logger.Errorln(err)
		return
	}

	// read data
	nBytes += opt_ts_val_len
	payload := data[nBytes:pduHeader.Length]

	addr.init(
		pduHeader.Total,
		pduHeader.Cwnd,
		pduHeader.Seqnohi,
		pduHeader.Msid,
		pduOptions.Tsval,
		ipInt32ToString(pduHeader.Srcid),
		&destEntries,
	)

	addr.payload = &payload
}

func (addr *addressPDU) isRecepient(ip string) bool {
	for _, dest := range *addr.dst_entries {
		if dest.dest_ipaddr == ip {
			return true
		}
	}

	return false
}

func (add *addressPDU) getDestList() []string {
	dests := []string{}
	for _, val := range *add.dst_entries {
		dests = append(dests, val.dest_ipaddr)
	}

	return dests
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
