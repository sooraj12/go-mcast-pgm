package pgm

import "bytes"

func (ack *ackPDU) init() {}

func (ack *ackPDU) length() {}

func (ack *ackPDU) toBuffer(b *bytes.Buffer) {}

func (ack *ackPDU) fromBuffer() {}

func (ae *ackInfoEntry) length() {}

func (ae *ackInfoEntry) toBuffer() {}

func (ae *ackInfoEntry) fromBuffer() {}
