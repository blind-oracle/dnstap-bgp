package main

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"runtime"
	"strconv"

	dnstap "github.com/dnstap/golang-dnstap"
	"github.com/golang/protobuf/proto"
	"github.com/miekg/dns"
)

var (
	// yandex.ru.  217     IN      A       5.255.255.77
	rrRegex = regexp.MustCompile(`^(.*)\.\t\d+\tIN\tA\t(.*)$`)
)

type dnstapCfg struct {
	Socket string
	Perm   string
}

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
			ds.cb(s[2], s[1])
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
