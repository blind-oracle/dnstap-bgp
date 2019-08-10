package main

import (
    "github.com/dnstap/golang-dnstap"
    "github.com/golang/protobuf/proto"
    "github.com/miekg/dns"
    "github.com/golang/protobuf/ptypes"
    "github.com/golang/protobuf/ptypes/any"
    api "github.com/osrg/gobgp/api"
    gobgp "github.com/osrg/gobgp/pkg/server"
    "context"
    "regexp"
    "flag"
    "time"
    "bufio"
    "sync"
    "os"
    "log"
)

var (
    // yandex.ru.  217     IN      A       5.255.255.77
    rrRegex = regexp.MustCompile(`^(.*)\.\t\d+\tIN\tA\t(.*)$`)
    ips = map[string]bool{}
    zones = map[string]bool{}
    bgp *bgpServer
    ipCache *cache
)

type cacheEntry struct {
    ip string
    domain string
    ts time.Time
}

type cache struct {
    m map[string]*cacheEntry
    ttl time.Duration
    sync.Mutex
}

func (c *cache) cleanup() {
    for {
	now := time.Now()
	
	c.Lock()
	for k, v := range c.m {
	    if now.Sub(v.ts) >= c.ttl {
		delete(c.m, k)
		bgp.delHost(v.ip)
		log.Printf("%s (%s) expired", v.ip, v.domain)
	    }
	}
	c.Unlock()
	
	time.Sleep(time.Second)
    }
}

func (c *cache) exists(ip string) (bool) {
    c.Lock()
    e, ok := c.m[ip]
    if ok {
	e.ts = time.Now()
    }
    c.Unlock()
    return ok
}

func (c *cache) add(ip, domain string) {
    c.Lock()
    c.m[ip] = &cacheEntry{
	ip: ip,
	domain: domain,
	ts: time.Now(),
    }
    c.Unlock()
}

func newCache(ttl time.Duration) (c *cache) {
    c = &cache{
	m: map[string]*cacheEntry{},
	ttl: ttl,
    }

    go c.cleanup()
    return
}

type bgpServer struct {
    s *gobgp.BgpServer
    rid string
}

func newBgp(peer string, routerID string, as int) (b *bgpServer) {
    b = &bgpServer{
	s: gobgp.NewBgpServer(),
	rid: routerID,
    }
    go b.s.Serve()

    if err := b.s.StartBgp(context.Background(), &api.StartBgpRequest{
	Global: &api.Global{
	    As:         uint32(as),
	    RouterId:   routerID,
	    ListenPort: -1,
	},
    }); err != nil {
	log.Fatal(err)
    }
    
    // monitor the change of the peer state
    if err := b.s.MonitorPeer(context.Background(), &api.MonitorPeerRequest{}, func(p *api.Peer) { log.Println(p) }); err != nil {
	log.Fatal(err)
    }

    // neighbor configuration
    n := &api.Peer{
	Conf: &api.PeerConf{
	    NeighborAddress: peer,
	    PeerAs:          uint32(as),
	},
    }

    if err := b.s.AddPeer(context.Background(), &api.AddPeerRequest{
	Peer: n,
    }); err != nil {
	log.Fatal(err)
    }
    
    return
}

func (b *bgpServer) addHost(ip string) {
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

    _, err := bgp.s.AddPath(context.Background(), &api.AddPathRequest{
	Path: &api.Path{
	    Family: &api.Family{Afi: api.Family_AFI_IP, Safi: api.Family_SAFI_UNICAST},
	    Nlri:   nlri,
	    Pattrs: []*any.Any{a1, a2},
	},
    })

    if err != nil {
	log.Println(err)
    }
}

func (b *bgpServer) delHost(ip string) {
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

    err := bgp.s.DeletePath(context.Background(), &api.DeletePathRequest{
	Path: &api.Path{
	    Family: &api.Family{Afi: api.Family_AFI_IP, Safi: api.Family_SAFI_UNICAST},
	    Nlri:   nlri,
	    Pattrs: []*any.Any{a1, a2},
	},
    })

    if err != nil {
	log.Println(err)
    }

    return
}

func handleMsg(m *dns.Msg) {
    for _, a := range m.Answer {
	if s := rrRegex.FindStringSubmatch(a.String()); len(s) == 3 {
	    zone := s[1]
	    ip := s[2]
	    
	    if ok, _ := zones[zone]; !ok {
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

func handle(in chan []byte) {
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
	    log.Println("Unpack failed: %s", err)
	    continue
	}
	
	handleMsg(dnsMsg)
    }
}

func main() {
    socket := flag.String("s", "", "Socket")
    domainList := flag.String("d", "", "domain list")
    peer := flag.String("p", "", "bgp peer")
    routerID := flag.String("r", "", "bgp router id")
    as := flag.Int("a", 0, "bgp as")
    ttl := flag.Duration("t", 30 * time.Second, "ip ttl")

    flag.Parse()

    f, err := os.Open(*domainList)
    if err != nil {
	log.Fatalf("Unable to open file: %s", err)
    }

    sc := bufio.NewScanner(f)
    for sc.Scan() {
	d := sc.Text()
	zones[d] = true
    }

    if err = sc.Err(); err != nil {
	log.Fatalf("Unable to read domain list: %s", err)
    }

    log.Printf("Read %d domains", len(zones))
    
    bgp = newBgp(*peer, *routerID, *as)
    ipCache = newCache(*ttl)

    log.Printf("Using socket %s", *socket)
    i, err := dnstap.NewFrameStreamSockInputFromPath(*socket)
    if err != nil {
	log.Fatalf("FSTRM error: %s", err)
    }

    ch := make(chan []byte)
    go handle(ch)
    go i.ReadInto(ch)
    i.Wait()
}
