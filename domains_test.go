package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_domainReverse(t *testing.T) {
	d := map[string]string{
		"a.b.c.d.e.f.g":    "g.f.e.d.c.b.a",
		"api.facebook.com": "com.facebook.api",
	}

	for k, v := range d {
		r := domainReverse(k)
		assert.Equal(t, r, v)
	}
}

func Test_domainTree(t *testing.T) {
	l := []string{
		"com.facebook",
	}

	dt := newDomainTree()
	dt.loadList(l)
	assert.True(t, dt.has("api.facebook.com"))
	assert.False(t, dt.has("facebookk.com"))
}

func Test_domainLevel(t *testing.T) {
	assert.Equal(t, 5, domainLevel("a.b.c.d.e"))
}
