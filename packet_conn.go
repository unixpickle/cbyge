package cbyge

import (
	"bytes"
	"io"
	"net"
	"time"

	"github.com/pkg/errors"
)

const DefaultPacketConnHost = "cm.gelighting.com:23778"
const PacketConnTimeout = time.Second * 10

type PacketConn struct {
	conn net.Conn
}

// NewPacketConn creates a PacketConn connected to the default server.
func NewPacketConn() (*PacketConn, error) {
	remoteAddr, err := net.ResolveTCPAddr("tcp", DefaultPacketConnHost)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialTimeout("tcp", remoteAddr.String(), PacketConnTimeout)
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
	data := packet.Encode()
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
// If timeout is non-zero, it is a socket read/write
// timeout; otherwise, no timeout is used.
func (p *PacketConn) Auth(code string, timeout time.Duration) error {
	if timeout != 0 {
		p.conn.SetDeadline(time.Now().Add(timeout))
		defer p.conn.SetDeadline(time.Time{})
	}

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
	if response.Type != PacketTypeAuth || !response.IsResponse {
		return errors.New("authenticate: unexpected response packet type")
	}
	if !bytes.Equal(response.Data, []byte{0, 0}) {
		return errors.New("authenticate: credentials not recognized")
	}
	return nil
}
