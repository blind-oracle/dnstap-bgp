package main

import (
	"fmt"
	"math/rand"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_syncer(t *testing.T) {
	e0 := &cacheEntry{
		IP:     net.ParseIP("1.2.3.4"),
		Domain: "foo.bar",
		TS:     time.Now(),
	}

	ga := func() []*cacheEntry {
		return []*cacheEntry{
			e0,
		}
	}

	var e1 *cacheEntry
	ch := make(chan struct{})
	add := func(e *cacheEntry, b bool) bool {
		e1 = e
		close(ch)
		return false
	}

	var err2 error
	cb := func(s string, i int, err error) {
		if err != nil {
			err2 = err
			close(ch)
		}
	}

	port := rand.Intn(60000) + 2000
	s, err := newSyncer(&syncerCfg{
		Listen: fmt.Sprintf("0.0.0.0:%d", port),
		Peers:  []string{fmt.Sprintf("127.0.0.1:%d", port)},
	}, ga, add, cb)
	assert.Nil(t, err)

	e2 := &cacheEntry{
		IP:     net.ParseIP("4.3.2.1"),
		Domain: "bar.foo",
		TS:     time.Now(),
	}

	s.broadcast(e2)
	<-ch
	assert.Equal(t, e2.IP, e1.IP)
	assert.Equal(t, e2.Domain, e1.Domain)

	ch = make(chan struct{})
	s.syncAll()
	<-ch

	assert.Equal(t, e0.IP, e1.IP)
	assert.Equal(t, e0.Domain, e1.Domain)
	assert.Nil(t, err2)

	err = s.close()
	assert.Nil(t, err)
}
