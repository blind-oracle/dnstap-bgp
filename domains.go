package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/armon/go-radix"
)

var (
	reDomain = regexp.MustCompile(`^([a-z0-9]+(-[a-z0-9]+)*\.)+[a-z]{2,}$`)
)

func domainReverse(d string) (dr string) {
	e := strings.Split(d, ".")
	for i := 0; i < len(e)/2; i++ {
		j := len(e) - i - 1
		e[i], e[j] = e[j], e[i]
	}

	return strings.Join(e, ".")
}

func domainLevel(d string) (l int) {
	for _, c := range d {
		if c == '.' {
			l++
		}
	}

	return l + 1
}

type domainTree struct {
	t *radix.Tree
	sync.RWMutex
}

func (d *domainTree) has(s string) (ok bool) {
	dr := domainReverse(s)

	d.RLock()
	pfx, _, ok := d.t.LongestPrefix(dr)
	d.RUnlock()

	if ok {
		if pfx == dr {
			return
		}

		if domainLevel(pfx) == domainLevel(dr) {
			return false
		}
	}

	return
}

func (d *domainTree) loadFile(path string) (i, s int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, fmt.Errorf("unable to open file: %w", err)
	}

	domains := []string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		dm := sc.Text()
		if !reDomain.MatchString(dm) {
			s++
			continue
		}

		domains = append(domains, domainReverse(dm))
	}

	if err = sc.Err(); err != nil {
		return 0, 0, fmt.Errorf("unable to read file: %w", err)
	}

	i, ss, err := d.loadList(domains)
	return i, s + ss, err
}

func (d *domainTree) loadList(domains []string) (i, s int, err error) {
	sort.Strings(domains)
	t := radix.New()

	for _, dm := range domains {
		sdm, _, ok := t.LongestPrefix(dm)
		if ok && domainLevel(dm) != domainLevel(sdm) {
			s++
			continue
		}

		t.Insert(dm, true)
		i++
	}

	if t.Len() == 0 {
		return 0, 0, fmt.Errorf("no domains loaded (%d skipped)", s)
	}

	d.Lock()
	d.t = t
	d.Unlock()
	return
}

func (d *domainTree) count() int {
	d.RLock()
	defer d.RUnlock()
	return d.t.Len()
}

func newDomainTree() (d *domainTree) {
	d = &domainTree{
		t: radix.New(),
	}

	return
}
