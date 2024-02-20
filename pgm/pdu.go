package pgm

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"logger"
)

func (p *PDU) fromBuffer(data []byte) {
	if len(data) < minimun_packet_len {
		logger.Errorln("Pdu.from_buffer() FAILED with: Message too small")
		return
	}

	buf := bytes.Buffer{}
	buf.Write(data[:minimun_packet_len])
	err := binary.Read(&buf, binary.LittleEndian, p)
	if err != nil {
		fmt.Println(err)
		return
	}
}

func (p *PDU) log(rxtx string) {
	logger.Debugf("%s: --- PDU --------------------------------------------------------", rxtx)
	logger.Debugf("%s: - Len[%d] Prio[%d] Type[%d]", rxtx, p.Len, p.Priority, p.PduType)
	logger.Debugf("%s: ----------------------------------------------------------------", rxtx)
}
