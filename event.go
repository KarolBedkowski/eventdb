// Copyright (C) 2017 Karol Będkowski
//
// Distributed under terms of the GPLv3 license.
package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/prometheus/common/log"
	"hash/adler32"
	"strings"
	"time"
)

var defaultBucket = []byte("__default__")

// ErrDecodeError when unmarshaling data
var ErrDecodeError = errors.New("decode error")

// AnyBucket means select all buckets
const AnyBucket = "_any_"

func init() {
}

// SetTags parse string into tags list
func (e *Event) SetTags(t string) {
	var tags []string
	for _, ts1 := range strings.Split(t, " ") {
		for _, ts2 := range strings.Split(ts1, ",") {
			ts2 = strings.TrimSpace(ts2)
			if ts2 != "" {
				tags = append(tags, ts2)
			}
		}
	}
	e.Tags = tags
}

// Decode event
func (e *Event) unmarshal(data []byte) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = ErrDecodeError
		}
	}()

	_, err = e.Unmarshal(data[1:])
	return err
}

// decode event ts - legacy
func unmarshalTSlegacy(data []byte) (int64, error) {
	var ts int64
	buf := bytes.NewReader(data[:8])
	err := binary.Read(buf, binary.BigEndian, &ts)
	return ts, err
}

// decode event ts
func unmarshalTS(data []byte) (int64, error) {
	if len(data) < 8 {
		return 0, ErrDecodeError
	}
	ts := int64(0)
	ts |= int64(data[0]) << 56
	ts |= int64(data[1]) << 48
	ts |= int64(data[2]) << 40
	ts |= int64(data[3]) << 32
	ts |= int64(data[4]) << 24
	ts |= int64(data[5]) << 16
	ts |= int64(data[6]) << 8
	ts |= int64(data[7])
	return ts, nil
}

// marshal event ts
func marshalTS(ts int64, data []byte) ([]byte, error) {
	buf := make([]byte, 8)
	buf[0] = byte((ts >> 56) & 0xff)
	buf[1] = byte((ts >> 48) & 0xff)
	buf[2] = byte((ts >> 40) & 0xff)
	buf[3] = byte((ts >> 32) & 0xff)
	buf[4] = byte((ts >> 24) & 0xff)
	buf[5] = byte((ts >> 16) & 0xff)
	buf[6] = byte((ts >> 8) & 0xff)
	buf[7] = byte(ts & 0xff)
	if data != nil {
		hash := adler32.Checksum(data)
		buf = append(buf,
			byte((hash>>3)&0xff),
			byte((hash>>2)&0xff),
			byte((hash>>1)&0xff),
			byte(hash&0xff),
		)
	}
	return buf, nil
}

// marshal event - legacy
func marshalTSlegacy(ts int64, data []byte) ([]byte, error) {
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

// encode (marshal) Event
func (e *Event) marshal() ([]byte, []byte, error) {
	// KEY: ts(int64)crc(4) (12bytes)
	buf, err := e.Marshal(nil)
	if err != nil {
		return nil, nil, err
	}

	key, err := marshalTS(e.Time, buf)

	if err == nil {
		// prefix by version
		buf = append([]byte{1}, buf...)
	}

	return buf, key, err
}

// CheckTags check if event has all `tags`
func (e *Event) CheckTags(tags []string) bool {
	if tags == nil || len(tags) == 0 {
		return true
	}

	if e.Tags == nil || len(e.Tags) == 0 {
		return false
	}

	for _, t := range tags {
		found := false
		for _, et := range e.Tags {
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

// SaveEvent to database
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
		data, key, err := e.marshal()
		if err == nil {
			return b.Put(key, data)
		}

		return err
	})
}

func getEventsFromBucket(f, t int64, b *bolt.Bucket, bname []byte) []*Event {
	fkey, err := marshalTS(f, nil)
	if err != nil {
		log.Errorf("ERROR: marshalTS for %v error: %s", t, err)
		fkey = []byte{0}
	}

	c := b.Cursor()
	var events []*Event

	for k, v := c.Seek(fkey); k != nil; k, v = c.Next() {
		if ts, err := unmarshalTS(k); err != nil {
			log.Errorf("ERROR: decode event ts error: %s", err.Error())
		} else if ts >= f && ts <= t {
			var err error
			e := &Event{}

			if v == nil || len(v) < 2 {
				err = fmt.Errorf("invalid data")
			} else {
				switch v[0] {
				case 1:
					err = e.unmarshal(v)
				default:
					err = fmt.Errorf("invalid version: %v", v[0])
				}
			}

			if err == nil && e != nil {
				events = append(events, e)
			} else {
				log.Errorf("ERROR: decode event ts: %v/%v error: %s", k, ts, err)
			}
		}
	}

	return events
}

// GetEvents from database according to `from`-`to` time range and bucket `name`
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

		b := tx.Bucket(bname)
		if b == nil {
			log.Infof("unknown bucket name: %v", name)
			return nil
		}

		events = getEventsFromBucket(f, t, b, bname)
		return nil
	})

	return events, err
}

func getEventsKeyFromBucket(f, t int64, b *bolt.Bucket) [][]byte {
	fkey, err := marshalTS(f, nil)
	if err != nil {
		log.Errorf("ERROR: marshalTS for %v error: %s", t, err)
		fkey = []byte{0}
	}

	c := b.Cursor()
	var keys [][]byte

	for k, _ := c.Seek(fkey); k != nil; k, _ = c.Next() {
		if ts, err := unmarshalTS(k); err != nil {
			log.Errorf("ERROR: decode event error: %v", err)
		} else if ts >= f && ts <= t {
			keys = append(keys, k)
		}
	}

	return keys
}

// DeleteEvents from database according to `from`-`to` time range and bucket `name`
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

		b := tx.Bucket(bname)
		if b == nil {
			log.Infof("unknown bucket name: %v", name)
			return nil
		}
		{
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
