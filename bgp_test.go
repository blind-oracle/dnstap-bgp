package main

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_BGP(t *testing.T) {
	b, err := newBgp(&bgpCfg{
		AS:       65000,
		RouterID: "127.0.0.1",
		Peers: []string{
			"127.0.0.1",
		},
	})

	assert.Nil(t, err)
	err = b.addHost(net.ParseIP("1.2.3.4"))
	assert.Nil(t, err)

	err = b.delHost(net.ParseIP("1.2.3.4"))
	assert.Nil(t, err)

	err = b.close()
	assert.Nil(t, err)
}
