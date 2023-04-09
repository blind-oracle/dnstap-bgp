package main

import (
	"net"
	"os"
	"testing"

	"google.golang.org/protobuf/proto"

	dnstap "github.com/dnstap/golang-dnstap"
	"github.com/miekg/dns"

	"github.com/stretchr/testify/assert"
)

func Test_ParseDNSReply(t *testing.T) {
	msg1 := &dns.Msg{
		Answer: []dns.RR{
			&dns.CNAME{
				Hdr: dns.RR_Header{
					Name: "mqtt-mini.facebook.com.",
				},

				Target: "mqtt-mini.c10r.facebook.com.",
			},

			&dns.A{
				Hdr: dns.RR_Header{
					Name: "mqtt-mini.c10r.facebook.com.",
				},

				A: net.ParseIP("157.240.17.34"),
			},

			&dns.AAAA{
				Hdr: dns.RR_Header{
					Name: "mqtt-mini.c10r.facebook.com.",
				},

				AAAA: net.ParseIP("2a03:2880:f15b:84:face:b00c:0:1ea0"),
			},
		},
	}

	msg2 := &dns.Msg{
		Answer: []dns.RR{
			&dns.A{
				Hdr: dns.RR_Header{
					Name: "mqtt-mini.c10r.facebook.com.",
				},

				A: net.ParseIP("157.240.17.34"),
			},

			&dns.AAAA{
				Hdr: dns.RR_Header{
					Name: "mqtt-mini.c10r.facebook.com.",
				},

				AAAA: net.ParseIP("2a03:2880:f15b:84:face:b00c:0:1ea0"),
			},
		},
	}

	r := parseDNSReply(msg1, false)
	assert.Equal(t, []*dnsEntry{
		{
			fqdn: "mqtt-mini.facebook.com.",
			ip:   net.ParseIP("157.240.17.34"),
		},
	}, r)

	r = parseDNSReply(msg1, true)
	assert.Equal(t, []*dnsEntry{
		{
			fqdn: "mqtt-mini.facebook.com.",
			ip:   net.ParseIP("157.240.17.34"),
		},
		{
			fqdn: "mqtt-mini.facebook.com.",
			ip:   net.ParseIP("2a03:2880:f15b:84:face:b00c:0:1ea0"),
		},
	}, r)

	r = parseDNSReply(msg2, true)
	assert.Equal(t, []*dnsEntry{
		{
			fqdn: "mqtt-mini.c10r.facebook.com.",
			ip:   net.ParseIP("157.240.17.34"),
		},
		{
			fqdn: "mqtt-mini.c10r.facebook.com.",
			ip:   net.ParseIP("2a03:2880:f15b:84:face:b00c:0:1ea0"),
		},
	}, r)
}

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
		Listen: "dnstap.sock",
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
