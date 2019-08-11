package main

import (
	"log"

	dnstap "github.com/dnstap/golang-dnstap"
	"github.com/golang/protobuf/proto"
	"github.com/miekg/dns"
)

func handleMsg(m *dns.Msg) {
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

			log.Println(zone, ip)
			bgp.addHost(ip)
			ipCache.add(ip, zone)
		}
	}
}

func handleProtobuf(in chan []byte) {
	for frame := range in {
		tap := &dnstap.Dnstap{}
		if err := proto.Unmarshal(frame, tap); err != nil {
			log.Printf("Unmarshal failed: %s", err)
			continue
		}

		msg := tap.Message
		if msg.GetType() != dnstap.Message_CLIENT_RESPONSE {
			continue
		}

		dnsMsg := new(dns.Msg)
		if err := dnsMsg.Unpack(msg.ResponseMessage); err != nil {
			log.Printf("Unpack failed: %s", err)
			continue
		}

		handleMsg(dnsMsg)
	}
}
