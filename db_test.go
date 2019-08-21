package main

import (
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_DB(t *testing.T) {
	f := "__test.db"

	db, err := newDB(f)
	assert.Nil(t, err)

	e := &cacheEntry{
		IP:     net.ParseIP("1.2.3.4"),
		Domain: "test.foo",
		TS:     time.Now(),
	}

	err = db.add(e)
	assert.Nil(t, err)

	ee, err := db.fetchAll()
	assert.Nil(t, err)
	assert.Equal(t, ee[0].Domain, e.Domain)
	assert.Equal(t, ee[0].IP, e.IP)

	err = db.del(e.IP)
	assert.Nil(t, err)

	err = db.close()
	assert.Nil(t, err)
	os.Remove(f)
}
