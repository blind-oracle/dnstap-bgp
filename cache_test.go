package main

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_Cache(t *testing.T) {
	expired := 0
	cb := func(e *cacheEntry) {
		expired++
	}

	e := &cacheEntry{
		IP:     net.ParseIP("1.2.3.4"),
		Domain: "test.foo",
		TS:     time.Now(),
	}

	c := newCache(time.Millisecond, cb)
	c.add(e)
	assert.True(t, c.exists(e.IP, false))
	assert.Equal(t, 1, c.count())
	ee := c.getAll()
	assert.Equal(t, e, ee[0])
	time.Sleep(2 * time.Millisecond)
	c.cleanup()
	assert.Equal(t, 0, c.count())
	assert.Equal(t, 1, expired)
}
