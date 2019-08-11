package main

import (
	"sync"
	"time"
)

type cacheEntry struct {
	ip     string
	domain string
	ts     time.Time
}

type cache struct {
	m   map[string]*cacheEntry
	ttl time.Duration
	cb  func(*cacheEntry)
	sync.Mutex
}

func (c *cache) cleanup() {
	for {
		now := time.Now()

		c.Lock()
		for k, v := range c.m {
			if now.Sub(v.ts) >= c.ttl {
				delete(c.m, k)
				c.cb(v)
			}
		}
		c.Unlock()

		time.Sleep(time.Minute)
	}
}

func (c *cache) exists(ip string) bool {
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
		ip:     ip,
		domain: domain,
		ts:     time.Now(),
	}
	c.Unlock()
}

func (c *cache) count() int {
	c.Lock()
	defer c.Unlock()
	return len(c.m)
}

func newCache(ttl time.Duration, callback func(*cacheEntry)) (c *cache) {
	c = &cache{
		m:   map[string]*cacheEntry{},
		cb:  callback,
		ttl: ttl,
	}

	go c.cleanup()
	return
}
