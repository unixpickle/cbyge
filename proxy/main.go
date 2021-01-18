package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/unixpickle/cbyge"
	"github.com/unixpickle/essentials"
)

func main() {
	var listenAddr string
	var outputDir string
	flag.StringVar(&listenAddr, "-source", ":23778", "address to listen on")
	flag.StringVar(&outputDir, "-output", "saved-packets", "output directory")
	flag.Parse()

	tcpAddr, err := net.ResolveTCPAddr("tcp", listenAddr)
	essentials.Must(err)
	listener, err := net.ListenTCP("tcp", tcpAddr)
	essentials.Must(err)
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		essentials.Must(err)
		connDir, connID := MakeOutputDir(outputDir)
		go HandleConn(conn, connID, connDir)
	}
}

func MakeOutputDir(root string) (string, int) {
	for i := 0; true; i++ {
		newDir := filepath.Join(root, strconv.Itoa(i))
		if err := os.MkdirAll(newDir, 0755); err == nil {
			return newDir, i
		} else if !os.IsExist(err) {
			essentials.Must(err)
		}
	}
	panic("unreachable")
}

func HandleConn(conn net.Conn, id int, outDir string) {
	log.Printf("connection created with ID: %d", id)
	defer log.Printf("connection terminated: %d", id)

	clientConn := cbyge.NewPacketConnWrap(conn)
	defer clientConn.Close()

	serverConn, err := cbyge.NewPacketConn()
	essentials.Must(err)
	defer serverConn.Close()

	var packetLock sync.Mutex

	var wg sync.WaitGroup
	wg.Add(2)

	forward := func(direction string, source, dest *cbyge.PacketConn) {
		defer wg.Done()
		defer dest.Close()
		for {
			packet, err := source.Read()
			if err != nil {
				return
			}
			packetLock.Lock()
			WritePacket(outDir, direction, packet)
			packetLock.Unlock()
			log.Printf("conn=%d direction=%s packet=%s", id, direction, packet)
			if dest.Write(packet) != nil {
				return
			}
		}
	}
	go forward("in", clientConn, serverConn)
	go forward("out", serverConn, clientConn)

	wg.Wait()
}

func WritePacket(outDir, direction string, packet *cbyge.Packet) {
	listing, err := ioutil.ReadDir(outDir)
	essentials.Must(err)
	nextIdx := 0
	for _, entry := range listing {
		name := entry.Name()
		parts := strings.Split(name, "_")
		if len(parts) != 2 {
			continue
		}
		x, err := strconv.Atoi(parts[0])
		if err == nil {
			nextIdx = essentials.MaxInt(nextIdx, x+1)
		}
	}
	outName := fmt.Sprintf("%06d_%s", nextIdx, direction)
	outFile := filepath.Join(outDir, outName)
	essentials.Must(ioutil.WriteFile(outFile, packet.Encode(), 0644))
}
