package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"time"
)

type syncerCfg struct {
	Listen       string
	SyncInterval string
	Peers        []string
}

type getAllFunc func() []*cacheEntry
type addFunc func(*cacheEntry, bool) bool
type syncFunc func(string, int, error)

type syncer struct {
	s *http.Server
	c *http.Client

	syncInterval time.Duration
	peers        []string

	getAll getAllFunc
	add    addFunc
	syncCb syncFunc

	shutdown chan struct{}
}

func newSyncer(cf *syncerCfg, getAll getAllFunc, add addFunc, syncCb syncFunc) (s *syncer, err error) {
	s = &syncer{
		getAll:       getAll,
		add:          add,
		peers:        cf.Peers,
		syncCb:       syncCb,
		shutdown:     make(chan struct{}),
		syncInterval: 10 * time.Minute,
	}

	if cf.SyncInterval != "" {
		if s.syncInterval, err = time.ParseDuration(cf.SyncInterval); err != nil {
			return nil, fmt.Errorf("Unable to parse syncInterval: %s", err)
		}
	}

	if len(cf.Peers) > 0 {
		s.c = &http.Client{
			Timeout: 5 * time.Second,
		}
	}

	if s.syncInterval > 0 {
		go s.syncScheduler()
	}

	if cf.Listen == "" {
		return
	}

	addr, err := net.ResolveTCPAddr("tcp", cf.Listen)
	if err != nil {
		return
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return
	}

	s.s = &http.Server{}

	http.HandleFunc("/fetch", s.handleFetch)
	http.HandleFunc("/put", s.handlePut)

	go func() {
		if err := s.s.Serve(l); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	return
}

func (s *syncer) handleFetch(wr http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		wr.WriteHeader(400)
		return
	}

	json.NewEncoder(wr).Encode(s.getAll())
}

func (s *syncer) handlePut(wr http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	c := &cacheEntry{}
	if err := json.NewDecoder(r.Body).Decode(c); err != nil && err != io.EOF {
		wr.WriteHeader(400)
		fmt.Fprintf(wr, "Bad request: %s", err)
		return
	}

	s.add(c, false)
}

func (s *syncer) broadcast(e *cacheEntry) (err error) {
	if s.c == nil {
		return
	}

	for _, p := range s.peers {
		if err = s.send(e, p); err != nil {
			return
		}
	}

	return
}

func (s *syncer) callPeer(p, handler, method string, body io.ReadCloser) (resp *http.Response, err error) {
	u, _ := url.Parse(fmt.Sprintf("http://%s/%s", p, handler))
	r := &http.Request{
		Method: method,
		URL:    u,
		Body:   body,
	}

	if resp, err = s.c.Do(r); err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP Code not 200: %d", resp.StatusCode)
	}

	return
}

func (s *syncer) syncScheduler() {
	t := time.NewTicker(s.syncInterval)

	for {
		select {
		case <-t.C:
			s.syncAll()

		case <-s.shutdown:
			return
		}
	}
}

func (s *syncer) syncAll() {
	for _, p := range s.peers {
		new := 0

		es, err := s.fetchRemote(p)
		if err != nil {
			s.syncCb(p, 0, err)
			continue
		}

		for _, e := range es {
			if s.add(e, false) {
				new++
			}
		}

		log.Printf("Syncer: got %d (%d new) entries from peer %s", len(es), new, p)

		s.syncCb(p, new, nil)
	}

	return
}

func (s *syncer) fetchRemote(p string) (es []*cacheEntry, err error) {
	resp, err := s.callPeer(p, "fetch", "GET", nil)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if err = json.NewDecoder(resp.Body).Decode(&es); err != nil {
		return
	}

	return
}

func (s *syncer) send(e *cacheEntry, p string) (err error) {
	js, _ := json.Marshal(e)
	b := bytes.NewReader(js)
	resp, err := s.callPeer(p, "put", "PUT", ioutil.NopCloser(b))
	if err != nil {
		return
	}

	resp.Body.Close()
	return
}

func (s *syncer) close() error {
	close(s.shutdown)

	c, f := context.WithTimeout(context.Background(), 5*time.Second)
	defer f()

	return s.s.Shutdown(c)
}
