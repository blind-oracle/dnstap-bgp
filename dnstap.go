package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"runtime"
	"strconv"

	dnstap "github.com/dnstap/golang-dnstap"
	"github.com/golang/protobuf/proto"
	"github.com/miekg/dns"
)

type dnstapCfg struct {
	Socket string
	Perm   string
}

type fCb func(net.IP, string)
type fCbErr func(error)

type dnstapServer struct {
	cb          fCb
	cbErr       fCbErr
	fstrmServer *dnstap.FrameStreamSockInput
	ch          chan []byte
}

func stripDot(d string) string {
	return d[:len(d)-1]
}

func (ds *dnstapServer) handleDNSMsg(m *dns.Msg) {
	var (
		domain   string
		cnameTgt string
	)

loop:
	for _, rr := range m.Answer {
		hdr := rr.Header()

		var ip net.IP
		switch rr.(type) {
		case *dns.CNAME:
			cnameTgt = rr.(*dns.CNAME).Target
			domain = hdr.Name
			continue loop

		case *dns.A, *dns.AAAA:
			if cnameTgt == "" {
				domain = hdr.Name
			} else if cnameTgt != hdr.Name {
				continue loop
			}

			switch r := rr.(type) {
			case *dns.A:
				ip = r.A
			case *dns.AAAA:
				ip = r.AAAA
			}
		default:
			continue loop
		}

		log.Println(ip)
		ds.cb(ip, stripDot(domain))
	}
}

func (ds *dnstapServer) ProcessProtobuf() {
	for frame := range ds.ch {
		tap := &dnstap.Dnstap{}
		if err := proto.Unmarshal(frame, tap); err != nil {
			ds.cbErr(fmt.Errorf("Unmarshal failed: %s", err))
			continue
		}

		msg := tap.Message
		if msg.GetType() != dnstap.Message_CLIENT_RESPONSE {
			continue
		}

		dnsMsg := new(dns.Msg)
		if err := dnsMsg.Unpack(msg.ResponseMessage); err != nil {
			ds.cbErr(fmt.Errorf("Unpack failed: %s", err))
			continue
		}

		go ds.handleDNSMsg(dnsMsg)
	}
}

func newDnstapServer(c *dnstapCfg, cb fCb, cbErr fCbErr) (ds *dnstapServer, err error) {
	ds = &dnstapServer{
		ch:    make(chan []byte, 1024),
		cb:    cb,
		cbErr: cbErr,
	}

	if c.Socket == "" {
		log.Fatal("You need to specify DNSTap socket")
	}

	ds.fstrmServer, err = dnstap.NewFrameStreamSockInputFromPath(c.Socket)
	if err != nil {
		return nil, fmt.Errorf("DNSTap listening error: %s", err)
	}

	if c.Perm != "" {
		octal, err := strconv.ParseInt(c.Perm, 8, 32)
		if err != nil {
			return nil, fmt.Errorf("Unable to parse '%s' as octal: %s", c.Perm, err)
		}

		os.Chmod(c.Socket, os.FileMode(octal))
	}

	for i := 0; i < runtime.NumCPU(); i++ {
		go ds.ProcessProtobuf()
	}

	go ds.fstrmServer.ReadInto(ds.ch)
	return
}
