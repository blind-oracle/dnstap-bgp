package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/vishvananda/netns"
)

var (
	dTree   *domainTree
	bgp     *bgpServer
	ipCache *cache
)

type cfgRoot struct {
	Domains   string
	Namespace string
	TTL       string

	DNSTap *dnstapCfg
	BGP    *bgpCfg
}

func main() {
	var (
		err      error
		shutdown = make(chan struct{})
	)

	config := flag.String("config", "", "Path to a config file")
	flag.Parse()

	if *config == "" {
		log.Fatal("You need to specify path to a config file")
	}

	cfg := &cfgRoot{}
	if _, err = toml.DecodeFile(*config, &cfg); err != nil {
		log.Fatalf("Unable to parse config file '%s': %s", *config, err)
	}

	log.Printf("Configuration: %+v", cfg)

	if cfg.Domains == "" {
		log.Fatal("You need to specify path to a domain list")
	}

	ttl := 24 * time.Hour
	if cfg.TTL != "" {
		if ttl, err = time.ParseDuration(cfg.TTL); err != nil {
			log.Printf("Unable to parse TTL: %s", err)
		}
	}

	expireCb := func(c *cacheEntry) {
		log.Printf("%s (%s) expired", c.ip, c.domain)
		bgp.delHost(c.ip)
	}

	ipCache = newCache(ttl, expireCb)

	if cfg.Namespace != "" {
		nsh, err := netns.GetFromName(cfg.Namespace)
		if err != nil {
			log.Fatalf("Unable to find namespace '%s': %s", cfg.Namespace, err)
		}

		if err = netns.Set(nsh); err != nil {
			log.Fatalf("Unable to switch to namespace '%s': %s", cfg.Namespace, err)
		}

		log.Printf("Switched to namespace '%s'", cfg.Namespace)
	}

	dTree = newDomainTree()
	cnt, skip, err := dTree.loadFile(cfg.Domains)
	if err != nil {
		log.Fatalf("Unable to load domain list: %s", err)
	}

	log.Printf("Domains loaded: %d, skipped: %d", cnt, skip)

	if bgp, err = newBgp(cfg.BGP); err != nil {
		log.Fatalf("Unable to init BGP: %s", err)
	}

	addHostCb := func(ip, domain string) {
		if !dTree.has(domain) {
			return
		}

		if ipCache.exists(ip) {
			return
		}

		log.Printf("%s: %s", domain, ip)
		bgp.addHost(ip)
		ipCache.add(ip, domain)
	}

	dnsTapErrorCb := func(err error) {
		log.Println(err)
	}

	if _, err = newDnstapServer(cfg.DNSTap, addHostCb, dnsTapErrorCb); err != nil {
		log.Fatalf("Unable to init DNSTap: %s", err)
	}

	log.Printf("Created DNSTap socket %s", cfg.DNSTap.Socket)

	go func() {
		sigchannel := make(chan os.Signal, 1)
		signal.Notify(sigchannel, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGUSR1, os.Interrupt)

		for sig := range sigchannel {
			switch sig {
			case syscall.SIGHUP:
				if i, s, err := dTree.loadFile(cfg.Domains); err != nil {
					log.Printf("Unable to load file: %s", err)
				} else {
					log.Printf("Domains loaded: %d, skipped: %d", i, s)
				}

			case os.Interrupt, syscall.SIGTERM:
				close(shutdown)

			case syscall.SIGUSR1:
				log.Printf("IPs exported: %d, domains loaded: %d", ipCache.count(), dTree.count())
			}
		}
	}()

	<-shutdown
	bgp.stop()
}
