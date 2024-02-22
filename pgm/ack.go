package pgm

import (
	"bytes"
	"encoding/binary"
	"logger"
)

// // AckPDU class definition
// 0                   1                   2                   3
// 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |        Length_of_PDU        |    Priority   |   |  PDU_Type |  4
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |           unused            |            Checksum           |  8
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |                  Source_ID_of_ACK_Sender                    | 12
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |  Count_of_ACK_Info_Entries  |                               | 14
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+                               +
// |                                                             |
// |          List of ACK_Info_Entries (variable length)         |
// |                                                             |
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// // AckInfoEntry class definition
//
// This field contains the Source_ID of the transmitting node, the Message_ID
// and a variable length list of the sequence numbers of the Data_PDUs that
// have not yet been received.
//
// 0                   1                   2                   3
// 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
// One ACK_Info_Entry            +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
//	|    Length_of_ACK_Info_Entry   |
//
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |          Reserved           |  Highest_Seq_Number_in_Window |
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |                         Source_ID                           |
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |                     Message_ID (MSID)                       |
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// | Count_Of_Missing_Seq_Number | Missing_Data_PDU_Seq_Number 1 |
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |...                          | Missing_Data_PDU_Seq_Number n |
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |                                                             |
// +                           TValue                            +
// |                                                             |
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |                                                             |
// |                  Options (variable length)                  |
// |                                                             |
// +-------------------------------------------------------------+
//

func (ack *ackPDU) init(seqnohi uint16, remoteIP string, msid int32, tsval int64, tvalue int64, missed *[]uint16) {
	ack.srcIP = getInterfaceIP().String()
	ack.infoEntries = &[]ackInfoEntry{}

	entry := ackInfoEntry{}
	entry.init(seqnohi, remoteIP, msid, tsval, tvalue, missed)
	*ack.infoEntries = append(*ack.infoEntries, entry)
}

func (ack *ackPDU) length() uint16 {
	length := min_ack_pdu_len
	for _, val := range *ack.infoEntries {
		length += val.length()
	}
	return length
}

func (ack *ackPDU) toBuffer(b *bytes.Buffer) {
	pdu := ackPDUEncoder{
		Length:     ack.length(),
		Priority:   0,
		PduType:    Ack,
		Map:        0,
		CheckSum:   0,
		SrcID:      ipToInt32(ack.srcIP),
		AckInfoLen: uint16(len(*ack.infoEntries)),
	}
	binary.Write(b, binary.LittleEndian, &pdu)

	for _, info := range *ack.infoEntries {
		info.toBuffer(b)
	}
}

func (ack *ackPDU) fromBuffer(data []byte) {
	if uint16(len(data)) < min_ack_pdu_len {
		logger.Errorln("RX: AckPdu.from_buffer() FAILED with: Message to small")
		return
	}

	buf := bytes.Buffer{}
	ackpdu := ackPDUEncoder{}
	buf.Write(data[:min_ack_pdu_len])
	binary.Read(&buf, binary.LittleEndian, &ackpdu)

	if uint16(len(data)) < ackpdu.Length {
		logger.Errorln("RX: AckPdu.from_buffer() FAILED with: Message to small")
		return
	}

	if ackpdu.Map != 0 {
		logger.Errorln("RX: AckPdu.from_buffer() FAILED witg: Unused field is not zero")
		return
	}

	// read ack info entries
	count := uint16(0)
	nBytes := min_ack_pdu_len
	entries := []ackInfoEntry{}
	for count < ackpdu.AckInfoLen {
		infoEntry := ackInfoEntry{}
		infoEntry.fromBuffer(data[nBytes:])
		entries = append(entries, infoEntry)
		count += 1
		nBytes += infoEntry.length()
	}

	ack.srcIP = ipInt32ToString(ackpdu.SrcID)
	ack.infoEntries = &entries
}

func (ack *ackPDU) log(rxtx string) {
	logger.Debugf("%s *** AckPdu *************************************************", rxtx)
	logger.Debugf("%s * src:%s", rxtx, ack.srcIP)
	for _, val := range *ack.infoEntries {
		logger.Debugf("%s * seqnohi:%d remote_ipaddr:%s msid:%d missed:%#v tval:%d tsecr:%d", rxtx,
			val.seqnohi,
			val.remoteIP,
			val.msid,
			val.missingSeqnos,
			val.tsval,
			val.tsecr,
		)
	}

	logger.Debugf("%s ****************************************************************", rxtx)
}

// ack info entry
func (ae *ackInfoEntry) init(seqnohi uint16, remoteIP string, msid int32, tsval int64, tvalue int64, missed *[]uint16) {
	ae.seqnohi = seqnohi
	ae.remoteIP = remoteIP
	ae.msid = msid
	ae.tsecr = tsval
	ae.tsval = tvalue
	ae.missingSeqnos = missed
}

func (ae *ackInfoEntry) length() uint16 {
	return min_ack_info_entry_len + (2 * uint16(len(*ae.missingSeqnos))) + ack_pdu_options_len
}

func (ae *ackInfoEntry) toBuffer(b *bytes.Buffer) {
	ackInfo := ackInfoEntryEncoder{
		Length:     ae.length(),
		Reserved:   0,
		Seqnohi:    ae.seqnohi,
		RemoteID:   ipToInt32(ae.remoteIP),
		Msid:       ae.msid,
		MissingLen: uint16(len(*ae.missingSeqnos)),
	}
	binary.Write(b, binary.LittleEndian, &ackInfo)

	for _, val := range *ae.missingSeqnos {
		binary.Write(b, binary.LittleEndian, val)
	}

	ackOptions := ackInfoEntryOptionsEncoder{
		Tsval: ae.tsval,
		Tsopt: TsEcr,
		L:     12,
		V:     0,
		Tsecr: ae.tsecr,
	}
	binary.Write(b, binary.LittleEndian, &ackOptions)
}

func (ae *ackInfoEntry) fromBuffer(data []byte) {
	if len(data) < int(min_ack_info_entry_len) {
		logger.Errorln("RX: AckInfoEntry.from_buffer() FAILED with: Message to small")
		return
	}

	// read ack info entry fixed header
	header := data[:ack_info_header_len]
	ackInfoHeader := ackInfoEntryEncoder{}
	buf := bytes.Buffer{}
	buf.Write(header)
	err := binary.Read(&buf, binary.LittleEndian, &ackInfoHeader)
	if err != nil {
		logger.Errorln(err)
		return
	}
	buf.Reset()
	if ackInfoHeader.Length > uint16(len(data)) {
		logger.Errorln("RX AckInfoEntry.from_buffer() FAILED with: Buffer too small")
		return
	}
	if ackInfoHeader.Reserved != 0 {
		logger.Errorln("RX AckInfoEntry.from_buffer() FAILED with: Reserved field is not zero")
		return
	}
	if ackInfoHeader.Length != min_ack_info_entry_len+uint16(opt_ts_val_len)+(ackInfoHeader.MissingLen*2) {
		logger.Errorln("RX AckInfoEntry.from_buffer() FAILED with: Corrupt PDU")
		return
	}

	// read missing fragments sequence nos
	count := uint16(0)
	nBytes := ack_info_header_len
	missing := []uint16{}
	var seq uint16
	for count < ackInfoHeader.MissingLen {
		num := data[nBytes : nBytes+2]
		buf.Write(num)
		binary.Read(&buf, binary.LittleEndian, seq)
		missing = append(missing, seq)
		buf.Reset()
		nBytes += 2
	}

	// read options
	ackInfoOptions := ackInfoEntryOptionsEncoder{}
	buf.Write(data[nBytes:])
	binary.Read(&buf, binary.LittleEndian, &ackInfoOptions)

	ae.missingSeqnos = &missing
	ae.seqnohi = ackInfoHeader.Seqnohi
	ae.remoteIP = ipInt32ToString(ackInfoHeader.RemoteID)
	ae.msid = ackInfoHeader.Msid
	ae.tsval = ackInfoOptions.Tsval
	ae.tsecr = ackInfoOptions.Tsecr
}
