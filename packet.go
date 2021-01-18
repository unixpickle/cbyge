package cbyge

import (
	"fmt"
	"strings"
)

const (
	PacketTypeAuth uint8 = 1
	PacketTypePipe       = 7
)

type Packet struct {
	Type       uint8
	IsResponse bool
	Data       []byte
}

func (p *Packet) String() string {
	var hexStr strings.Builder
	for i, x := range p.Data {
		if i > 0 {
			hexStr.WriteByte(' ')
		}
		hexChar := fmt.Sprintf("%02x", x)
		hexStr.WriteString(hexChar)
	}
	return fmt.Sprintf("Packet(type=%d, response=%v, data=[%s])",
		p.Type, p.IsResponse, hexStr.String())
}

func (p *Packet) encode() []byte {
	typeByte := byte(p.Type<<4) | 3
	if p.IsResponse {
		typeByte |= 8
	}
	length := len(p.Data)
	header := []byte{
		typeByte,
		byte((length >> 24) & 0xff),
		byte((length >> 16) & 0xff),
		byte((length >> 8) & 0xff),
		byte(length & 0xff),
	}
	return append(header, p.Data...)
}
