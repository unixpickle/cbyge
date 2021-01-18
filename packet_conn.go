package cbyge

import (
	"bytes"
	"io"
	"net"

	"github.com/pkg/errors"
)

const DefaultPacketConnHost = "cm-ge.xlink.cn:23778"

const (
	PacketTypeAuth uint8 = 1
	PacketTypePipe       = 7
)

type Packet struct {
	Type       uint8
	IsResponse bool
	Data       []byte
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

type PacketConn struct {
	conn net.Conn
}

// NewPacketConn creates a PacketConn connected to the default server.
func NewPacketConn() (*PacketConn, error) {
	remoteAddr, err := net.ResolveTCPAddr("tcp", DefaultPacketConnHost)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialTCP("tcp", nil, remoteAddr)
	if err != nil {
		return nil, err
	}
	return &PacketConn{conn: conn}, nil
}

// NewPacketConnWrap creates a PacketConn on top of an existing socket.
func NewPacketConnWrap(conn net.Conn) *PacketConn {
	return &PacketConn{conn: conn}
}

func (p *PacketConn) Read() (*Packet, error) {
	header := make([]byte, 5)
	_, err := io.ReadFull(p.conn, header)
	if err != nil {
		return nil, err
	}
	typeByte := header[0]
	length := (int(header[1]) << 24) | (int(header[2]) << 16) | (int(header[3]) << 8) | int(header[4])
	if length > 0x100000 {
		return nil, errors.New("packet is unreasonably large")
	}
	data := make([]byte, length)
	_, err = io.ReadFull(p.conn, data)
	if err != nil {
		return nil, err
	}
	return &Packet{
		Type:       typeByte >> 4,
		IsResponse: (typeByte & 8) != 0,
		Data:       data,
	}, nil
}

func (p *PacketConn) Write(packet *Packet) error {
	data := packet.encode()
	for len(data) > 0 {
		n, err := p.conn.Write(data)
		if err != nil {
			return err
		}
		data = data[n:]
	}
	return nil
}

func (p *PacketConn) Close() error {
	return p.conn.Close()
}

// Auth does an authentication exchange with the server.
//
// Provide an authorization code, as obtained by Login().
func (p *PacketConn) Auth(code string) error {
	data := bytes.NewBuffer(nil)
	data.Write([]byte{0x03, 0x24, 0x8c, 0x57, 0x7d, 0, 0x10})
	data.Write([]byte(code))
	data.Write([]byte{0, 0, 0xb4})
	packet := &Packet{
		Type: PacketTypeAuth,
		Data: data.Bytes(),
	}
	if err := p.Write(packet); err != nil {
		return errors.Wrap(err, "authenticate")
	}
	response, err := p.Read()
	if err != nil {
		return errors.Wrap(err, "authenticate")
	}
	if response.Type != PacketTypeAuth || !packet.IsResponse {
		return errors.New("authenticate: unexpected response packet type")
	}
	if !bytes.Equal(response.Data, []byte{0, 0}) {
		return errors.New("authenticate: credentials not recognized")
	}
	return nil
}
