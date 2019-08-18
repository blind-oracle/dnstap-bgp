package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"
)

type syncerCfg struct {
	Listen string
	Peers  []string
}

type getAllFunc func() []*cacheEntry
type addFunc func(*cacheEntry, bool) bool
type syncFunc func(string, int, error)

type syncer struct {
	s *http.Server
	c *http.Client

	peers []string

	getAll getAllFunc
	add    addFunc
	syncCb syncFunc
}

func newSyncer(cf *syncerCfg, getAll getAllFunc, add addFunc, syncCb syncFunc) (s *syncer, err error) {
	s = &syncer{
		getAll: getAll,
		add:    add,
		peers:  cf.Peers,
		syncCb: syncCb,
	}

	if len(cf.Peers) > 0 {
		s.c = &http.Client{
			Timeout: 5 * time.Second,
		}
	}

	if cf.Listen == "" {
		return
	}

	s.s = &http.Server{
		Addr:    cf.Listen,
		Handler: http.DefaultServeMux,
	}

	http.HandleFunc("/fetch", s.fetch)
	http.HandleFunc("/put", s.put)

	ch := make(chan error)
	go func() {
		ch <- s.s.ListenAndServe()
	}()

	select {
	case err = <-ch:
		return nil, err
	case <-time.After(50 * time.Millisecond):
	}

	go s.syncScheduler()
	return
}

func (s *syncer) fetch(wr http.ResponseWriter, r *http.Request) {
	json.NewEncoder(wr).Encode(s.getAll())
}

func (s *syncer) put(wr http.ResponseWriter, r *http.Request) {
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
	for {
		s.syncAll()
		time.Sleep(time.Minute)
	}
}

func (s *syncer) syncAll() {
	for _, p := range s.peers {
		new := 0

		es, err := s.sync(p)
		if err != nil {
			s.syncCb(p, 0, err)
			continue
		}

		for _, e := range es {
			if s.add(e, false) {
				new++
			}
		}

		s.syncCb(p, new, nil)
	}

	return
}

func (s *syncer) sync(p string) (es []*cacheEntry, err error) {
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
	c, f := context.WithTimeout(context.Background(), 5*time.Second)
	defer f()
	return s.s.Shutdown(c)
}
