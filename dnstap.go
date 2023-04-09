package main

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"strconv"

	dnstap "github.com/dnstap/golang-dnstap"
	"github.com/miekg/dns"
	"google.golang.org/protobuf/proto"
)

type dnstapCfg struct {
	Listen string
	Perm   string
	IPv6   bool
}

type dnsEntry struct {
	ip   net.IP
	fqdn string
}

type fCb func(net.IP, string)
type fCbErr func(error)

type dnstapServer struct {
	cfg         *dnstapCfg
	cb          fCb
	cbErr       fCbErr
	fstrmServer *dnstap.FrameStreamSockInput
	l           net.Listener
	ch          chan []byte
}

/*
ch-odc.samsungapps.com.                                             300     IN      CNAME   ch-odc.gw.samsungapps.com.
ch-odc.gw.samsungapps.com.                                          10      IN      CNAME   fe-pew1-ext-s3store-elb-1085125128.eu-west-1.elb.amazonaws.com.
fe-pew1-ext-s3store-elb-1085125128.eu-west-1.elb.amazonaws.com.     60      IN      A       34.251.108.185
*/

func parseDNSReply(m *dns.Msg, ipv6 bool) []*dnsEntry {
	var domain string
	result := []*dnsEntry{}

	for _, rr := range m.Answer {
		hdr := rr.Header()

		switch rv := rr.(type) {
		case *dns.CNAME:
			if domain == "" {
				domain = hdr.Name
			}

		case *dns.A:
			if domain == "" {
				domain = hdr.Name
			}

			result = append(result, &dnsEntry{rv.A, domain})

		case *dns.AAAA:
			if !ipv6 {
				break
			}

			if domain == "" {
				domain = hdr.Name
			}

			result = append(result, &dnsEntry{rv.AAAA, domain})
		}
	}

	return result
}

func (ds *dnstapServer) handleDNSMsg(m *dns.Msg) {
	for _, d := range parseDNSReply(m, ds.cfg.IPv6) {
		ds.cb(d.ip, d.fqdn[:len(d.fqdn)-1])
	}
}

func (ds *dnstapServer) ProcessProtobuf() {
	for frame := range ds.ch {
		tap := &dnstap.Dnstap{}
		if err := proto.Unmarshal(frame, tap); err != nil {
			ds.cbErr(fmt.Errorf("unmarshal failed: %w", err))
			continue
		}

		msg := tap.Message
		if msg.GetType() != dnstap.Message_CLIENT_RESPONSE {
			continue
		}

		dnsMsg := new(dns.Msg)
		if err := dnsMsg.Unpack(msg.ResponseMessage); err != nil {
			ds.cbErr(fmt.Errorf("unpack failed: %w", err))
			continue
		}

		go ds.handleDNSMsg(dnsMsg)
	}
}

func newDnstapServer(c *dnstapCfg, cb fCb, cbErr fCbErr) (ds *dnstapServer, err error) {
	ds = &dnstapServer{
		cfg:   c,
		ch:    make(chan []byte, 1024),
		cb:    cb,
		cbErr: cbErr,
	}

	if c.Listen == "" {
		return nil, fmt.Errorf("you need to specify DNSTap listening poing")
	}

	if addr, err := net.ResolveTCPAddr("tcp", c.Listen); err == nil {
		if ds.l, err = net.ListenTCP("tcp", addr); err != nil {
			return nil, fmt.Errorf("unable to listen on '%s': %w", c.Listen, err)
		}

		ds.fstrmServer = dnstap.NewFrameStreamSockInput(ds.l)
	} else {
		ds.fstrmServer, err = dnstap.NewFrameStreamSockInputFromPath(c.Listen)
		if err != nil {
			return nil, fmt.Errorf("unable to listen on '%s': %w", c.Listen, err)
		}

		if c.Perm != "" {
			octal, err := strconv.ParseInt(c.Perm, 8, 32)
			if err != nil {
				return nil, fmt.Errorf("unable to parse '%s' as octal: %w", c.Perm, err)
			}

			os.Chmod(c.Listen, os.FileMode(octal))
		}
	}

	for i := 0; i < runtime.NumCPU(); i++ {
		go ds.ProcessProtobuf()
	}

	go ds.fstrmServer.ReadInto(ds.ch)
	return
}
