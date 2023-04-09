package main

import (
	"net"
	"sync"
	"time"
)

type cacheEntry struct {
	IP     net.IP
	Domain string
	TS     time.Time
}

type expireFunc func(*cacheEntry)

type cache struct {
	m        map[string]*cacheEntry
	ttl      time.Duration
	expireCb expireFunc
	sync.RWMutex
}

func (c *cache) cleanupScheduler() {
	for {
		c.cleanup()
		time.Sleep(time.Minute)
	}
}

func (c *cache) cleanup() {
	now := time.Now()
	c.Lock()
	for k, v := range c.m {
		if now.Sub(v.TS) >= c.ttl {
			delete(c.m, k)

			if c.expireCb != nil {
				c.expireCb(v)
			}
		}
	}
	c.Unlock()
}

func (c *cache) exists(ip net.IP, update bool) bool {
	c.Lock()
	e, ok := c.m[string(ip)]
	if ok && update {
		e.TS = time.Now()
	}
	c.Unlock()
	return ok
}

func (c *cache) add(e *cacheEntry) {
	c.Lock()
	c.m[string(e.IP)] = e
	c.Unlock()
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

func newCache(ttl time.Duration, expireCb expireFunc) (c *cache) {
	c = &cache{
		m:        map[string]*cacheEntry{},
		expireCb: expireCb,
		ttl:      ttl,
	}

	go c.cleanupScheduler()
	return
}
