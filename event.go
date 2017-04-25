// Copyright (C) 2017 Karol Będkowski
//
// Distributed under terms of the GPLv3 license.
package main

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/prometheus/common/log"
	"hash/adler32"
	"strings"
	"time"
)

type Event struct {
	*EventBase

	// internals
	key    []byte
	bucket []byte
}

var defaultBucket = []byte("__default__")

const AnyBucket = "_any_"

func init() {
	gob.Register(&EventBase{})
}

func decodeEventGOB(e []byte) (*Event, error) {
	evb := &EventBase{}
	r := bytes.NewBuffer(e)
	dec := gob.NewDecoder(r)
	if err := dec.Decode(evb); err != nil {
		return nil, err
	}

	ev := &Event{
		EventBase: evb,
	}
	return ev, nil
}

var DecodeError = errors.New("decode error")

func decodeEventG(evb *EventBase, e []byte) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = DecodeError
		}
	}()

	_, err = evb.Unmarshal(e)
	return
}

func decodeEvent(e []byte) (*Event, error) {
	evb := &EventBase{}
	var err error
	if err = decodeEventG(evb, e); err != nil {
		r := bytes.NewBuffer(e)
		dec := gob.NewDecoder(r)
		err = dec.Decode(evb)
	}

	if err != nil {
		return nil, err
	}

	ev := &Event{
		EventBase: evb,
	}
	return ev, nil
}

func decodeEventTS(k []byte) (int64, error) {
	var ts int64
	buf := bytes.NewReader(k[:8])
	err := binary.Read(buf, binary.BigEndian, &ts)
	return ts, err
}

func encodeEventTS(ts int64, data []byte) ([]byte, error) {
	key := new(bytes.Buffer)
	if err := binary.Write(key, binary.BigEndian, ts); err != nil {
		return nil, err
	}
	if data != nil {
		hash := adler32.Checksum(data)
		key.Write([]byte{
			byte((hash >> 3) & 0xff),
			byte((hash >> 2) & 0xff),
			byte((hash >> 1) & 0xff),
			byte(hash & 0xff),
		})
	}
	return key.Bytes(), nil
}

// legacy
func (e *Event) encodeGOB() ([]byte, []byte, error) {
	// KEY: ts(int64)crc(4) (12bytes)
	r := new(bytes.Buffer)
	enc := gob.NewEncoder(r)
	if err := enc.Encode(e.EventBase); err != nil {
		return nil, nil, err
	}

	key, err := encodeEventTS(e.Time, r.Bytes())
	e.key = key
	return r.Bytes(), key, err
}

func (e *Event) encode() ([]byte, []byte, error) {
	// KEY: ts(int64)crc(4) (12bytes)

	buf, err := e.EventBase.Marshal(nil)
	if err != nil {
		return nil, nil, err
	}

	key, err := encodeEventTS(e.Time, buf)
	e.key = key

	return buf, key, err
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
	return db.db.Update(func(tx *bolt.Tx) error {
		name := defaultBucket
		if e.Name != "" {
			name = []byte(e.Name)
		}

		b, err := tx.CreateBucketIfNotExists(name)
		if err != nil {
			return err
		}

		b.FillPercent = 0.99
		data, key, err := e.encode()
		if err == nil {
			return b.Put(key, data)
		}

		return err
	})
}

func getEventsFromBucket(f, t int64, b *bolt.Bucket, bname []byte) []*Event {
	fkey, err := encodeEventTS(f, nil)
	if err != nil {
		log.Errorf("ERROR: encodeEventTS for %v error: %s", t, err)
		fkey = []byte{0}
	}

	c := b.Cursor()
	var events []*Event

	for k, v := c.Seek(fkey); k != nil; k, v = c.Next() {
		if ts, err := decodeEventTS(k); err != nil {
			log.Errorf("ERROR: decode event ts error: %s", err.Error())
		} else if ts >= f && ts <= t {
			if e, err := decodeEvent(v); err == nil {
				e.key = k
				e.bucket = bname
				events = append(events, e)
			} else {
				log.Errorf("ERROR: decode event ts: %v/%v error: %s", k, ts, err.Error())
			}
		}
	}

	return events
}

func (db *DB) GetEvents(from, to time.Time, name string) ([]*Event, error) {
	log.Debugf("GetEvents %s - %s [%s]", from, to, name)

	f := from.UnixNano()
	t := to.UnixNano()
	if t < f {
		return nil, fmt.Errorf("wrong time range (from > to)")
	}

	var events []*Event

	err := db.db.View(func(tx *bolt.Tx) error {
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

		if b := tx.Bucket(bname); b == nil {
			return fmt.Errorf("unknown bucket name: %v", name)
		} else {
			events = getEventsFromBucket(f, t, b, bname)
		}

		return nil
	})

	return events, err
}

func getEventsKeyFromBucket(f, t int64, b *bolt.Bucket) [][]byte {
	fkey, err := encodeEventTS(f, nil)
	if err != nil {
		log.Errorf("ERROR: encodeEventTS for %v error: %s", t, err)
		fkey = []byte{0}
	}

	c := b.Cursor()
	var keys [][]byte

	for k, _ := c.Seek(fkey); k != nil; k, _ = c.Next() {
		if ts, err := decodeEventTS(k); err != nil {
			log.Errorf("ERROR: decode event error: %v", err)
		} else if ts >= f && ts <= t {
			keys = append(keys, k)
		}
	}

	return keys
}

func (db *DB) DeleteEvents(from, to time.Time, name string) (int, error) {
	f := from.UnixNano()
	t := to.UnixNano()

	deleted := 0

	err := db.db.Update(func(tx *bolt.Tx) error {
		if name == AnyBucket {
			return tx.ForEach(func(name []byte, b *bolt.Bucket) error {
				keys := getEventsKeyFromBucket(f, t, b)

				for _, k := range keys {
					if err := b.Delete(k); err != nil {
						return err
					}
				}

				deleted += len(keys)
				return nil
			})
		}

		bname := defaultBucket
		if name != "" {
			bname = []byte(name)
		}

		if b := tx.Bucket(bname); b == nil {
			return fmt.Errorf("unknown bucket name: %v", name)
		} else {
			keys := getEventsKeyFromBucket(f, t, b)
			for _, k := range keys {
				if err := b.Delete(k); err != nil {
					return err
				}
			}
			deleted += len(keys)
		}

		return nil
	})

	return deleted, err
}
