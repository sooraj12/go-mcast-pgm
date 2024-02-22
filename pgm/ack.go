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

func (ack *ackPDU) fromBuffer(data []byte) {}

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
		Length:       ae.length(),
		Reserved:     0,
		Seqnohi:      ae.seqnohi,
		RemoteID:     ipToInt32(ae.remoteIP),
		Msid:         ae.msid,
		MissingCount: uint16(len(*ae.missingSeqnos)),
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

}
