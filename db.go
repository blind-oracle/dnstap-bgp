package main

import (
	"bytes"
	"encoding/gob"
	"net"

	bolt "go.etcd.io/bbolt"
)

type db struct {
	h *bolt.DB
	b []byte
}

func newDB(path string) (d *db, err error) {
	d = &db{
		b: []byte("ipcache"),
	}

	if d.h, err = bolt.Open(path, 0666, nil); err != nil {
		return
	}

	return d, d.h.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(d.b)
		return err
	})
}

func (d *db) add(e *cacheEntry) (err error) {
	var b bytes.Buffer
	if err = gob.NewEncoder(&b).Encode(e); err != nil {
		return
	}

	return d.h.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(d.b).Put(e.IP, b.Bytes())
	})
}

func (d *db) del(ip net.IP) (err error) {
	return d.h.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(d.b).Delete(ip)
	})
}

func (d *db) fetchAll() (es []*cacheEntry, err error) {
	err = d.h.View(func(tx *bolt.Tx) error {
		return tx.Bucket(d.b).ForEach(func(k, v []byte) (err error) {
			e := &cacheEntry{}
			if err = gob.NewDecoder(bytes.NewBuffer(v)).Decode(e); err != nil {
				return err
			}

			es = append(es, e)
			return nil
		})
	})

	return es, err
}

func (d *db) close() error {
	return d.h.Close()
}
