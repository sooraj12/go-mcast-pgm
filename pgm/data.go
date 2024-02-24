package pgm

import (
	"bytes"
	"encoding/binary"
	"logger"
)

func (d *dataPDU) init(cwnd uint16, seqnohi uint16, msid int32, seqno uint16, cwndSeqno uint16, data *[]byte) {
	d.cwnd = cwnd
	d.seqnohi = seqnohi
	d.cwndSeqno = cwndSeqno
	d.seqno = seqno
	d.srcIP = getInterfaceIP().String()
	d.msid = msid
	d.data = data
}

func (d *dataPDU) length() uint16 {
	return uint16(data_pdu_header_len + len(*d.data))
}

func (d *dataPDU) toBuffer(buf *bytes.Buffer) {
	pdu := dataPDUEncoder{
		Length:    d.length(),
		Priority:  0,
		PduType:   uint8(Data),
		Cwnd:      d.cwnd,
		Seqnohi:   d.seqnohi,
		Checksum:  0,
		Seqno:     d.seqno,
		CwndSeqno: d.cwndSeqno,
		Reserved:  0,
		srcID:     ipToInt32(d.srcIP),
		Msid:      d.msid,
	}
	err := binary.Write(buf, binary.LittleEndian, &pdu)
	if err != nil {
		return
	}

	buf.Write(*d.data)
}

func (d *dataPDU) fromBuffer(data *[]byte) {
	if len(*data) < data_pdu_header_len {
		logger.Errorln("RX: DataPdu.from_buffer() FAILED with: Message to small")
		return
	}
	// read data pdu header
	header := bytes.Buffer{}
	header.Write((*data)[:data_pdu_header_len])
	pduHeader := dataPDUEncoder{}
	binary.Read(&header, binary.LittleEndian, &pduHeader)

	if pduHeader.Length < uint16(data_pdu_header_len) {
		logger.Errorln("RX: DataPdu.from_buffer() FAILED with: Invalid lenght field")
		return
	}

	if pduHeader.Length > uint16(len(*data)) {
		logger.Errorln("RX: DataPdu.from_buffer() FAILED with: Message too small")
		return
	}

	if pduHeader.Reserved != 0 {
		logger.Errorln("Rx: DataPdu.from_buffer() FAILED with: Reserved field is not zero")
		return
	}

	d.cwnd = pduHeader.Cwnd
	d.seqnohi = pduHeader.Seqnohi
	d.cwndSeqno = pduHeader.CwndSeqno
	d.seqno = pduHeader.Seqno
	d.srcIP = ipInt32ToString(pduHeader.srcID)
	d.msid = pduHeader.Msid

	dt := (*data)[data_pdu_header_len:]
	d.data = &dt
}

func (d *dataPDU) log(state string) {
	logger.Debugf("%s *** Data *************************************************", state)
	logger.Debugf("%s * cwnd:%d seqnohi:%d cwnd_seqno:%d seqno:%d srcid:%s msid:%d", state,
		d.cwnd,
		d.seqnohi,
		d.cwndSeqno,
		d.seqno,
		d.srcIP,
		d.msid,
	)
	logger.Debugf("%s ****************************************************************", state)
}
