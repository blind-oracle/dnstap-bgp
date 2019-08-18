package main

import (
	"sync"
	"time"
)

type cacheEntry struct {
	IP     string
	Domain string
	TS     time.Time
}

type cache struct {
	m   map[string]*cacheEntry
	ttl time.Duration
	cb  func(*cacheEntry)
	sync.RWMutex
}

func (c *cache) cleanup() {
	for {
		now := time.Now()

		c.Lock()
		for k, v := range c.m {
			if now.Sub(v.TS) >= c.ttl {
				delete(c.m, k)

				if c.cb != nil {
					c.cb(v)
				}
			}
		}
		c.Unlock()

		time.Sleep(time.Minute)
	}
}

func (c *cache) exists(ip string, update bool) bool {
	c.Lock()
	e, ok := c.m[ip]
	if ok && update {
		e.TS = time.Now()
	}
	c.Unlock()
	return ok
}

func (c *cache) add(e *cacheEntry) {
	c.Lock()
	c.m[e.IP] = e
	c.Unlock()
	return
}

func (c *cache) getAll() (es []*cacheEntry) {
	c.RLock()
	for _, e := range c.m {
		es = append(es, e)
	}
	c.RUnlock()
	return
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
