// Copyright (C) 2017 Karol Będkowski
//
// Distributed under terms of the GPLv3 license.
package main

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/prometheus/common/log"
	"strings"
	"time"
)

type Event struct {
	Name  string
	Title string
	// Time in nanos
	Time int64
	Text string
	Tags string

	// internals
	key    []byte
	bucket []byte
}

var defaultBucket = []byte("__default__")

const AnyBucket = "_any_"

func init() {
	gob.Register(&Event{})
}

func decodeEvent(e []byte) (*Event, error) {
	/*
		b := bytes.NewReader(e)
		var r io.Reader
		if zr, err := zlib.NewReader(b); err == nil {
			defer zr.Close()
			r = zr
		} else {
			r = bytes.NewBuffer(e)
		}
	*/

	r := bytes.NewBuffer(e)

	ev := &Event{}
	dec := gob.NewDecoder(r)
	if err := dec.Decode(ev); err != nil {
		log.Warnf("decodeEvent decode error: %s", err)
		return nil, err
	}
	return ev, nil
}

func decodeEventTS(k []byte) (int64, error) {
	var ts int64
	buf := bytes.NewReader(k[:8])
	err := binary.Read(buf, binary.BigEndian, &ts)
	if err != nil {
		log.Errorf("decodeEventTS failed: %s", err)
		return 0, err
	}
	return ts, nil
}

func encodeEventTS(ts int64, data []byte) ([]byte, error) {
	key := new(bytes.Buffer)
	if err := binary.Write(key, binary.BigEndian, ts); err != nil {
		log.Errorf("event encode error: %s", err)
		return nil, err
	}
	if data == nil {
		key.Write([]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	} else {
		h := md5.New()
		h.Write(data)
		key.Write(h.Sum(nil))
	}
	return key.Bytes(), nil
}

func (e *Event) encode() ([]byte, []byte, error) {
	// KEY: ts(int64)md5sum(16)
	r := new(bytes.Buffer)
	enc := gob.NewEncoder(r)
	if err := enc.Encode(e); err != nil {
		log.Errorf("event encode error: %s", err)
		return nil, nil, err
	}

	key, err := encodeEventTS(e.Time, r.Bytes())
	e.key = key

	/*
		// compress content
		var b bytes.Buffer
		w, _ := zlib.NewWriterLevel(&b, zlib.BestCompression)
		w.Write(r.Bytes())
		w.Close()

		log.Debugf("encode %d -> %d", r.Len(), b.Len())


		return b.Bytes(), key, err
	*/
	return r.Bytes(), key, err
}

func (e *Event) CheckTags(tags []string) bool {
	if tags == nil || len(tags) == 0 {
		return true
	}
	if e.Tags == "" {
		return false
	}

	etags := strings.Split(e.Tags, " ")
	for _, t := range tags {
		found := false
		for _, et := range etags {
			if t == et {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

func (db *DB) SaveEvent(e *Event) error {
	log.Debugf("SaveEvent: %+v", e)
	return db.db.Update(func(tx *bolt.Tx) error {
		name := defaultBucket
		if e.Name != "" {
			name = []byte(e.Name)
		}
		b, err := tx.CreateBucketIfNotExists(name)
		if err != nil {
			return err
		}
		b.FillPercent = 0.95
		data, key, err := e.encode()
		if err == nil {
			return b.Put(key, data)
		}
		return err
	})
}

func getEventsFromBucket(f, t int64, b *bolt.Bucket, bname []byte) []*Event {
	events := make([]*Event, 0, 100)
	c := b.Cursor()
	fkey, _ := encodeEventTS(f, nil)

	for k, v := c.Seek(fkey); k != nil; k, v = c.Next() {
		if ts, err := decodeEventTS(k); err != nil || ts < f || ts > t {
			log.Debugf("e ts: %v", ts)
			continue
		}
		if e, err := decodeEvent(v); err == nil {
			e.key = k
			e.bucket = bname
			events = append(events, e)
		}
	}

	return events
}

func (db *DB) GetEvents(from, to time.Time, name string) []*Event {
	log.Debugf("GetEvents %s - %s [%s]", from, to, name)

	f := from.UnixNano()
	t := to.UnixNano()
	if t < f {
		return nil
	}

	events := make([]*Event, 0, 0)

	db.db.View(func(tx *bolt.Tx) error {
		if name == AnyBucket {
			return tx.ForEach(func(name []byte, b *bolt.Bucket) error {
				es := getEventsFromBucket(f, t, b, name)
				events = append(events, es...)
				return nil
			})
		}

		bname := defaultBucket
		if name != "" {
			bname = []byte(name)
		}

		b := tx.Bucket(bname)
		if b == nil {
			return fmt.Errorf("unknown bucket name: %v", name)
		}
		events = getEventsFromBucket(f, t, b, bname)
		return nil
	})

	return events
}

func getEventsKeyFromBucket(f, t int64, b *bolt.Bucket) [][]byte {
	keys := make([][]byte, 0, 100)
	c := b.Cursor()
	fkey, _ := encodeEventTS(f, nil)

	for k, _ := c.Seek(fkey); k != nil; k, _ = c.Next() {
		if ts, err := decodeEventTS(k); err != nil || ts < f || ts > t {
			continue
		}

		keys = append(keys, k)
	}

	return keys
}

func (db *DB) DeleteEvents(from, to time.Time, name string) int {
	f := from.UnixNano()
	t := to.UnixNano()

	deleted := 0

	err := db.db.Update(func(tx *bolt.Tx) error {

		if name == AnyBucket {
			tx.ForEach(func(name []byte, b *bolt.Bucket) error {
				keys := getEventsKeyFromBucket(f, t, b)
				deleted += len(keys)

				for _, k := range keys {
					if err := b.Delete(k); err != nil {
						return err
					}
				}

				return nil
			})
		} else {
			bname := defaultBucket
			if name != "" {
				bname = []byte(name)
			}
			b := tx.Bucket(bname)
			if b == nil {
				return fmt.Errorf("unknown bucket name: %v", name)
			}
			keys := getEventsKeyFromBucket(f, t, b)
			deleted += len(keys)
			for _, k := range keys {
				if err := b.Delete(k); err != nil {
					return err
				}
			}
		}

		return nil
	})

	if err != nil {
		log.Errorf("event delete error: %s", err.Error())
		return 0
	}

	return deleted
}
