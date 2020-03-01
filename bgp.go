package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	api "github.com/osrg/gobgp/api"
	gobgp "github.com/osrg/gobgp/pkg/server"
)

type bgpCfg struct {
	AS       uint32
	RouterID string
	NextHop  string
	SourceIP string
	SourceIF string

	Peers []string
	IPv6  bool
}

type bgpServer struct {
	s *gobgp.BgpServer
	c *bgpCfg
}

func newBgp(c *bgpCfg) (b *bgpServer, err error) {
	if c.AS == 0 {
		return nil, fmt.Errorf("You need to provide AS")
	}

	if c.SourceIP != "" && c.SourceIF != "" {
		return nil, fmt.Errorf("SourceIP and SourceIF are mutually exclusive")
	}

	if len(c.Peers) == 0 {
		return nil, fmt.Errorf("You need to provide at least one peer")
	}

	b = &bgpServer{
		s: gobgp.NewBgpServer(),
		c: c,
	}
	go b.s.Serve()

	if err = b.s.StartBgp(context.Background(), &api.StartBgpRequest{
		Global: &api.Global{
			As:         c.AS,
			RouterId:   c.RouterID,
			ListenPort: -1,
		},
	}); err != nil {
		return
	}

	if err = b.s.MonitorPeer(context.Background(), &api.MonitorPeerRequest{}, func(p *api.Peer) { log.Println(p) }); err != nil {
		return
	}

	for _, p := range c.Peers {
		if err = b.addPeer(p); err != nil {
			return
		}
	}

	return
}

func (b *bgpServer) addPeer(addr string) (err error) {
	port := 179

	if t := strings.SplitN(addr, ":", 2); len(t) == 2 {
		addr = t[0]

		if port, err = strconv.Atoi(t[1]); err != nil {
			return fmt.Errorf("Unable to parse port '%s' as int: %s", t[1], err)
		}
	}

	p := &api.Peer{
		Conf: &api.PeerConf{
			NeighborAddress: addr,
			PeerAs:          b.c.AS,
		},

		Timers: &api.Timers{
			Config: &api.TimersConfig{
				ConnectRetry: 10,
			},
		},

		Transport: &api.Transport{
			MtuDiscovery:  true,
			RemoteAddress: addr,
			RemotePort:    uint32(port),
		},
	}

	if b.c.SourceIP != "" {
		p.Transport.LocalAddress = b.c.SourceIP
	}

	if b.c.SourceIF != "" {
		p.Transport.BindInterface = b.c.SourceIF
	}

	return b.s.AddPeer(context.Background(), &api.AddPeerRequest{
		Peer: p,
	})
}

func (b *bgpServer) getPath(ip net.IP) *api.Path {
	var pfxLen uint32 = 32
	if ip.To4() == nil {
		if !b.c.IPv6 {
			return nil
		}

		pfxLen = 128
	}

	nlri, _ := ptypes.MarshalAny(&api.IPAddressPrefix{
		Prefix:    ip.String(),
		PrefixLen: pfxLen,
	})

	a1, _ := ptypes.MarshalAny(&api.OriginAttribute{
		Origin: 0,
	})

	var nh string
	if b.c.NextHop != "" {
		nh = b.c.NextHop
	} else if b.c.SourceIP != "" {
		nh = b.c.SourceIP
	} else {
		nh = b.c.RouterID
	}

	a2, _ := ptypes.MarshalAny(&api.NextHopAttribute{
		NextHop: nh,
	})

	return &api.Path{
		Family: &api.Family{
			Afi:  api.Family_AFI_IP,
			Safi: api.Family_SAFI_UNICAST,
		},
		Nlri:   nlri,
		Pattrs: []*any.Any{a1, a2},
	}
}

func (b *bgpServer) addHost(ip net.IP) (err error) {
	p := b.getPath(ip)
	if p == nil {
		return
	}

	_, err = b.s.AddPath(context.Background(), &api.AddPathRequest{
		Path: p,
	})

	return
}

func (b *bgpServer) delHost(ip net.IP) (err error) {
	p := b.getPath(ip)
	if p == nil {
		return
	}

	return b.s.DeletePath(context.Background(), &api.DeletePathRequest{
		Path: p,
	})
}

func (b *bgpServer) close() error {
	ctx, cf := context.WithTimeout(context.Background(), 5*time.Second)
	defer cf()
	return b.s.StopBgp(ctx, &api.StopBgpRequest{})
}
