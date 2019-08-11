package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"time"

	dnstap "github.com/dnstap/golang-dnstap"
)

var (
	// yandex.ru.  217     IN      A       5.255.255.77
	rrRegex = regexp.MustCompile(`^(.*)\.\t\d+\tIN\tA\t(.*)$`)
	dTree   *domainTree
	bgp     *bgpServer
	ipCache *cache
)

func main() {
	var err error

	socket := flag.String("s", "", "DNSTap UNIX socket path")
	domainList := flag.String("d", "", "Domain list (one per line)")
	peer := flag.String("p", "", "BGP peer")
	routerID := flag.String("r", "", "BGP router-id")
	as := flag.Int("a", 0, "BGP AS")
	ttl := flag.Duration("t", 24*time.Hour, "TTL to announce IPs")

	flag.Parse()

	if *socket == "" {
		log.Fatal("You need to specify DNSTap socket")
	}

	if *domainList == "" {
		log.Fatal("You need to specify domain list")
	}

	if *peer == "" {
		log.Fatal("You need to specify BGP peer")
	}

	if *routerID == "" {
		log.Fatal("You need to specify BGP router-id")
	}

	if *as == 0 {
		log.Fatal("You need to specify BGP AS")
	}

	dTree = newDomainTree()
	cnt, skip, err := dTree.loadFile(*domainList)
	if err != nil {
		log.Fatalf("Unable to load domain list: %s", err)
	}

	log.Printf("Domains loaded: %d, skipped: %d", cnt, skip)

	if bgp, err = newBgp(*peer, *routerID, *as); err != nil {
		log.Fatalf("Unable to init BGP: %s", err)
	}

	expire := func(c *cacheEntry) {
		log.Printf("%s (%s) expired", c.ip, c.domain)
		bgp.delHost(c.ip)
	}

	ipCache = newCache(*ttl, expire)

	i, err := dnstap.NewFrameStreamSockInputFromPath(*socket)
	if err != nil {
		log.Fatalf("FSTRM error: %s", err)
	}

	log.Printf("Created DNSTap socket %s", *socket)

	ch := make(chan []byte)
	go handleProtobuf(ch)
	go i.ReadInto(ch)

	go func() {
		sigchannel := make(chan os.Signal, 1)
		signal.Notify(sigchannel, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGUSR1, os.Interrupt)

		for sig := range sigchannel {
			switch sig {
			case syscall.SIGHUP:
				if i, s, err := dTree.loadFile(*domainList); err != nil {
					log.Printf("Unable to load file: %s", err)
				} else {
					log.Printf("Domains loaded: %d, skipped: %d", i, s)
				}

			case os.Interrupt, syscall.SIGTERM:
				bgp.stop()
				os.Exit(0)

			case syscall.SIGUSR1:
				log.Printf("IPs exported: %d, domains loaded: %d", ipCache.count(), dTree.count())
			}
		}
	}()

	i.Wait()
}
