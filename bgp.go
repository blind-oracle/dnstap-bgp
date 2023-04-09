package main

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/any"
	api "github.com/osrg/gobgp/v3/api"
	gobgp "github.com/osrg/gobgp/v3/pkg/server"
	"google.golang.org/protobuf/types/known/anypb"
)

type bgpCfg struct {
	AS          uint32
	RouterID    string
	NextHop     string
	NextHopIPv6 string
	SourceIP    string

	Peers []string
	IPv6  bool
}

type bgpServer struct {
	s *gobgp.BgpServer
	c *bgpCfg
}

func newBgp(c *bgpCfg) (b *bgpServer, err error) {
	if c.AS == 0 {
		return nil, fmt.Errorf("you need to provide AS")
	}

	if len(c.Peers) == 0 {
		return nil, fmt.Errorf("you need to provide at least one peer")
	}

	b = &bgpServer{
		s: gobgp.NewBgpServer(),
		c: c,
	}
	go b.s.Serve()

	if err = b.s.StartBgp(context.Background(), &api.StartBgpRequest{
		Global: &api.Global{
			Asn:        c.AS,
			RouterId:   c.RouterID,
			ListenPort: -1,
		},
	}); err != nil {
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
			return fmt.Errorf("unable to parse port '%s' as int: %s", t[1], err)
		}
	}

	p := &api.Peer{
		Conf: &api.PeerConf{
			NeighborAddress: addr,
			PeerAsn:         b.c.AS,
		},

		AfiSafis: []*api.AfiSafi{
			{
				Config: &api.AfiSafiConfig{
					Family: &api.Family{
						Afi:  api.Family_AFI_IP,
						Safi: api.Family_SAFI_UNICAST,
					},
					Enabled: true,
				},
			},
			{
				Config: &api.AfiSafiConfig{
					Family: &api.Family{
						Afi:  api.Family_AFI_IP6,
						Safi: api.Family_SAFI_UNICAST,
					},
					Enabled: true,
				},
			},
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

	return b.s.AddPeer(context.Background(), &api.AddPeerRequest{
		Peer: p,
	})
}

func (b *bgpServer) getPath(ip net.IP) *api.Path {
	var nh string
	var pfxLen uint32 = 32
	if ip.To4() == nil {
		if !b.c.IPv6 {
			return nil
		}
		pfxLen = 128
	}

	nlri, _ := anypb.New(&api.IPAddressPrefix{
		Prefix:    ip.String(),
		PrefixLen: pfxLen,
	})

	a1, _ := anypb.New(&api.OriginAttribute{
		Origin: 0,
	})

	if ip.To4() == nil {
		v6Family := &api.Family{
			Afi:  api.Family_AFI_IP6,
			Safi: api.Family_SAFI_UNICAST,
		}

		if b.c.NextHopIPv6 != "" {
			nh = b.c.NextHopIPv6
		} else {
			nh = "fd00::1"
		}

		v6Attrs, _ := anypb.New(&api.MpReachNLRIAttribute{
			Family:   v6Family,
			NextHops: []string{nh},
			Nlris:    []*any.Any{nlri},
		})

		return &api.Path{
			Family: v6Family,
			Nlri:   nlri,
			Pattrs: []*any.Any{a1, v6Attrs},
		}
	} else {
		if b.c.NextHop != "" {
			nh = b.c.NextHop
		} else if b.c.SourceIP != "" {
			nh = b.c.SourceIP
		} else {
			nh = b.c.RouterID
		}

		a2, _ := anypb.New(&api.NextHopAttribute{
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
