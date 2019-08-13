package main

import (
	"fmt"
	"regexp"
	"runtime"

	dnstap "github.com/dnstap/golang-dnstap"
	"github.com/golang/protobuf/proto"
	"github.com/miekg/dns"
)

var (
	rrRegex = regexp.MustCompile(`^(.*)\.\t\d+\tIN\tA\t(.*)$`)
)

type fCb func(string, string)
type fCbErr func(error)

type dnstapServer struct {
	cb          fCb
	cbErr       fCbErr
	fstrmServer *dnstap.FrameStreamSockInput
	ch          chan []byte
}

func (ds *dnstapServer) handleDNSMsg(m *dns.Msg) {
	for _, a := range m.Answer {
		if s := rrRegex.FindStringSubmatch(a.String()); len(s) == 3 {
			zone := s[1]
			ip := s[2]

			if !dTree.has(zone) {
				continue
			}

			if ipCache.exists(ip) {
				continue
			}

			ds.cb(ip, zone)
		}
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

		ds.handleDNSMsg(dnsMsg)
	}
}

func newDnstapServer(socket string, cb fCb, cbErr fCbErr) (ds *dnstapServer, err error) {
	ds = &dnstapServer{
		ch:    make(chan []byte, 1024),
		cb:    cb,
		cbErr: cbErr,
	}

	ds.fstrmServer, err = dnstap.NewFrameStreamSockInputFromPath(socket)
	if err != nil {
		return nil, fmt.Errorf("DNSTap listening error: %s", err)
	}

	for i := 0; i < runtime.NumCPU(); i++ {
		go ds.ProcessProtobuf()
	}

	go ds.fstrmServer.ReadInto(ds.ch)
	//go ds.fstrmServer.Wait()
	return
}
