package cbyge

import (
	"fmt"
	"strings"
)

const (
	PacketTypeAuth uint8 = 1
	PacketTypePipe       = 7
)

const (
	PacketPipeTypeSetStatus uint8 = 0xd0
	PacketPipeTypeSetLum          = 0xd2
	PacketPipeTypeSetCT           = 0xe2
	PacketPipeTypeGetStatus       = 0xdb
)

type Packet struct {
	Type       uint8
	IsResponse bool
	Data       []byte
}

// NewPacketPipe creates a "pipe buffer" packet with a given subtype.
func NewPacketPipe(subtype uint8, data []byte) *Packet {
	if len(data) > 0xff {
		panic("payload is too long")
	}
	fmt.Println(len(data))
	return &Packet{
		Type: PacketTypePipe,
		Data: append(append([]byte{
			0x2a, 0x71, 0x67, 0x63, // Always present at the start of these packets
			0x7a, 0x61, // Sequence number?
			0x00,
			0x7e, 0x0d, // Sequence number?

			0x01, 0x00, // Unsure what this is, can be set to zero.
			0x00, 0xf8, // Unsure what this is, but it is required.

			subtype, uint8(len(data)),
		}, data...), 0, 0, 0),
	}
}

// NewPacketSetDeviceStatus creates a packet for turning on or off a device.
//
// Set status to 1 to turn on, or 0 to turn off.
func NewPacketSetDeviceStatus(device, status int) *Packet {
	return NewPacketPipe(PacketPipeTypeSetStatus, []byte{
		0, 0, 0, 0, 0,
		byte(device >> 8), byte(device & 0xff), // Device index
		0, PacketPipeTypeSetStatus, // Command, repeated

		// 0x11, 0x02, // Unknown, can be set to zero
		0, 0,

		byte(status),
		0,
	})
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

// Encode the packet in raw binary form.
func (p *Packet) Encode() []byte {
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
