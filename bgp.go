package main

import (
	"context"
	"log"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	api "github.com/osrg/gobgp/api"
	gobgp "github.com/osrg/gobgp/pkg/server"
)

type bgpServer struct {
	s   *gobgp.BgpServer
	rid string
}

func newBgp(peer string, routerID string, as int) (b *bgpServer, err error) {
	b = &bgpServer{
		s:   gobgp.NewBgpServer(),
		rid: routerID,
	}
	go b.s.Serve()

	if err = b.s.StartBgp(context.Background(), &api.StartBgpRequest{
		Global: &api.Global{
			As:         uint32(as),
			RouterId:   routerID,
			ListenPort: -1,
		},
	}); err != nil {
		return
	}

	if err = b.s.MonitorPeer(context.Background(), &api.MonitorPeerRequest{}, func(p *api.Peer) { log.Println(p) }); err != nil {
		return
	}

	n := &api.Peer{
		Conf: &api.PeerConf{
			NeighborAddress: peer,
			PeerAs:          uint32(as),
		},
	}

	if err = b.s.AddPeer(context.Background(), &api.AddPeerRequest{
		Peer: n,
	}); err != nil {
		return
	}

	return
}

func (b *bgpServer) stop() error {
	ctx, cf := context.WithTimeout(context.Background(), 5*time.Second)
	defer cf()
	return b.s.StopBgp(ctx, &api.StopBgpRequest{})
}

func (b *bgpServer) getPath(ip string) *api.Path {
	nlri, _ := ptypes.MarshalAny(&api.IPAddressPrefix{
		Prefix:    ip,
		PrefixLen: 32,
	})

	a1, _ := ptypes.MarshalAny(&api.OriginAttribute{
		Origin: 0,
	})

	a2, _ := ptypes.MarshalAny(&api.NextHopAttribute{
		NextHop: b.rid,
	})

	return &api.Path{
		Family: &api.Family{Afi: api.Family_AFI_IP, Safi: api.Family_SAFI_UNICAST},
		Nlri:   nlri,
		Pattrs: []*any.Any{a1, a2},
	}
}

func (b *bgpServer) addHost(ip string) (err error) {
	_, err = bgp.s.AddPath(context.Background(), &api.AddPathRequest{
		Path: b.getPath(ip),
	})

	return
}

func (b *bgpServer) delHost(ip string) (err error) {
	return bgp.s.DeletePath(context.Background(), &api.DeletePathRequest{
		Path: b.getPath(ip),
	})
}
