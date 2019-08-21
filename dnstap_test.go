package main

import (
	"net"
	"os"
	"testing"

	"github.com/golang/protobuf/proto"

	dnstap "github.com/dnstap/golang-dnstap"
	"github.com/miekg/dns"

	"github.com/stretchr/testify/assert"
)

func Test_DNSTap(t *testing.T) {
	ip, domain := net.ParseIP("1.2.3.4"), "test.foo."
	ip2, domain2 := net.IP{}, ""

	ch := make(chan struct{})
	cb := func(i net.IP, d string) {
		ip2 = i
		domain2 = d
		close(ch)
	}

	var err2 error
	cbe := func(err error) {
		err2 = err
	}

	_, err := newDnstapServer(&dnstapCfg{
		Socket: "dnstap.sock",
		Perm:   "666",
	}, cb, cbe)
	assert.Nil(t, err)

	addr, err := net.ResolveUnixAddr("unix", "dnstap.sock")
	assert.Nil(t, err)

	out, err := dnstap.NewFrameStreamSockOutput(addr)
	assert.Nil(t, err)
	go out.RunOutputLoop()

	dmsg := &dns.Msg{
		Answer: []dns.RR{
			&dns.A{
				A: ip,
				Hdr: dns.RR_Header{
					Name:   domain,
					Rrtype: dns.TypeA,
				},
			},
		},
	}

	b, err := dmsg.Pack()
	assert.Nil(t, err)

	typ1 := dnstap.Dnstap_MESSAGE
	typ2 := dnstap.Message_CLIENT_RESPONSE
	msg := &dnstap.Dnstap{
		Type: &typ1,
		Message: &dnstap.Message{
			Type:            &typ2,
			ResponseMessage: b,
		},
	}

	b2, err := proto.Marshal(msg)
	assert.Nil(t, err)

	out.GetOutputChannel() <- b2
	out.Close()

	<-ch
	assert.Nil(t, err2)
	assert.Equal(t, "test.foo", domain2)
	assert.Equal(t, ip.To4(), ip2)

	os.Remove("dnstap.sock")
}
