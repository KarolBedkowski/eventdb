// Copyright (C) 2017 Karol Będkowski
//
// Distributed under terms of the GPLv3 license.
package main

import (
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/prometheus/common/log"
	"hash/adler32"
	"sort"
	"strings"
	"time"
)

// ErrDecodeError when unmarshaling data
var ErrDecodeError = errors.New("decode error")

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

// ColumnValue get value for given column
func (e *Event) ColumnValue(col string) (v string, ok bool) {
	for _, c := range e.Cols {
		if c.Name == col {
			return c.Value, true
		}
	}
	return
}

// TS return event time as time.Time
func (e *Event) TS() time.Time {
	return time.Unix(0, e.Time)
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

// SaveEvent to database
func (db *DB) SaveEvent(e *Event) error {
	return db.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(e.Name))
		if err != nil {
			return err
		}

		b.FillPercent = 0.999

		data, key, err := e.marshal()
		if err == nil {
			return b.Put(key, data)
		}

		return err
	})
}

// DeleteEvents from database according to `from`-`to` time range and bucket `name`
func (db *DB) DeleteEvents(bucket string, from, to time.Time, filter func(*Event) bool) (deleted int, err error) {
	f := from.UnixNano()
	t := to.UnixNano()

	err = db.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return fmt.Errorf("unknown bucket name: %v", bucket)
		}

		b.FillPercent = 0.999

		{
			fkey, err := marshalTS(f, nil)
			if err != nil {
				log.Errorf("ERROR: marshalTS for %v error: %s", t, err)
				fkey = []byte{0}
			}

			c := b.Cursor()
			var keys [][]byte

			for k, v := c.Seek(fkey); k != nil; k, _ = c.Next() {
				if ts, err := unmarshalTS(k); err != nil {
					log.Errorf("ERROR: decode event error: %v", err)
				} else if ts >= f && ts <= t {
					if filter == nil {
						keys = append(keys, k)
					} else {
						e := &Event{}
						if err := e.unmarshal(v); err != nil {
							log.Errorf("ERROR: unmarshal  event error: %v", err)
							continue
						}
						if filter(e) {
							keys = append(keys, k)
						}
					}
				}
			}

			for _, k := range keys {
				if err := b.Delete(k); err != nil {
					return err
				}
			}

			deleted = len(keys)
		}

		return nil
	})

	return
}

func (db *DB) GetEvents(bucket string, from, to time.Time, filter func(*Event) bool) ([]*Event, error) {
	log.Debugf("GetEvents %s %s - %s", bucket, from, to)

	f := from.UnixNano()
	t := to.UnixNano()
	if t < f {
		return nil, fmt.Errorf("wrong time range (from > to)")
	}

	var events []*Event

	err := db.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return fmt.Errorf("unknown bucket name: %v", bucket)
		}

		fkey, err := marshalTS(f, nil)
		if err != nil {
			log.Errorf("ERROR: marshalTS for %v error: %s", t, err)
			fkey = []byte{0}
		}

		c := b.Cursor()

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
					if filter == nil || filter(e) {
						events = append(events, e)
					}
				} else {
					log.Errorf("ERROR: decode event ts: %v/%v error: %s", k, ts, err)
				}
			}
		}
		return nil
	})

	return events, err
}

type EventsByTime []*Event

func (a EventsByTime) Len() int           { return len(a) }
func (a EventsByTime) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a EventsByTime) Less(i, j int) bool { return a[i].Time < a[j].Time }

func SortEventsByTime(events []*Event) {
	sort.Sort(EventsByTime(events))
}

func (e EventCol) String() string {
	return fmt.Sprintf("%s:%s", e.Name, e.Value)
}
