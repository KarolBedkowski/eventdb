// Copyright (C) 2017 Karol Będkowski
//
// Distributed under terms of the GPLv3 license.
package main

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"github.com/boltdb/bolt"
	p "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"net/http"
	"strconv"
	"time"
)

type (
	DB struct {
		dbFilename string
		db         *bolt.DB
		stats      bolt.Stats
		statsDiff  bolt.Stats
	}

	Event struct {
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
)

var (
	db = &DB{}

	defaultBucket = []byte("__default__")
)

const AnyBucket = "_any_"

func init() {
	gob.Register(&Event{})
	p.MustRegister(NewMetricsCollector())
}

func DBOpen(filename string) (err error) {
	log.Debugf("globals.openDatabases START %s", filename)

	db.dbFilename = filename

	bdb, err := bolt.Open(filename, 0600,
		&bolt.Options{Timeout: 10 * time.Second})

	if err != nil {
		log.Error("DB.open db error: %v", err)
		panic("DB.open  db error " + err.Error())
	}

	db.db = bdb

	err = bdb.Update(func(tx *bolt.Tx) error {
		var err error
		_, err = tx.CreateBucketIfNotExists(defaultBucket)
		if err != nil {
			panic("DB.open create bucket error" + err.Error())
		}
		return err
	})

	db.stats = db.db.Stats()

	log.Debug("DB.openDatabases DONE")
	return
}

func DBClose() error {
	log.Info("DB.Close")
	if db.db != nil {
		db.db.Close()
		db.db = nil
	}
	log.Info("DB.Close DONE")
	return nil
}

func decodeEvent(e []byte) (*Event, error) {
	ev := &Event{}
	r := bytes.NewBuffer(e)
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
		log.Errorf("decodeEventTS failed:", err)
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
	return r.Bytes(), key, err
}

func SaveEvent(e *Event) error {
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

func GetEvents(from, to time.Time, name string) []*Event {
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
		} else {
			bname := defaultBucket
			if name != "" {
				bname = []byte(name)
			}

			b := tx.Bucket(bname)
			if b == nil {
				return fmt.Errorf("unknown bucket name: %v", name)
			}
			events = getEventsFromBucket(f, t, b, bname)
		}
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

func DeleteEvents(from, to time.Time, name string) int {
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

func NewDBInternalPagesHandler() http.Handler {
	h := &dbInternalHandler{}
	mux := http.NewServeMux()
	mux.HandleFunc("/backup", h.backup)
	mux.HandleFunc("/stats", h.stats)
	return mux
}

type dbInternalHandler struct {
}

func (d *dbInternalHandler) backup(w http.ResponseWriter, req *http.Request) {
	err := db.db.View(func(tx *bolt.Tx) error {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", `attachment; filename="`+db.dbFilename+`"`)
		w.Header().Set("Content-Length", strconv.Itoa(int(tx.Size())))
		_, err := tx.WriteTo(w)
		return err
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (d *dbInternalHandler) stats(w http.ResponseWriter, req *http.Request) {
	stats := db.db.Stats()
	db.statsDiff = db.stats.Sub(&db.stats)
	db.stats = stats
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(db.statsDiff)
}

type metricsCollector struct {
	freePageN     *p.Desc
	pendigPageN   *p.Desc
	freeAlloc     *p.Desc
	freelistInuse *p.Desc
	txN           *p.Desc
	openTxN       *p.Desc
	pageCount     *p.Desc
	pageAlloc     *p.Desc
	cursorCount   *p.Desc
	nodeCount     *p.Desc
	nodeDeref     *p.Desc
	rebalance     *p.Desc
	rebalanceTime *p.Desc
	split         *p.Desc
	spill         *p.Desc
	spillTime     *p.Desc
	write         *p.Desc
	writeTime     *p.Desc

	bucketBranchPageN       *p.Desc
	bucketBranchOverflowN   *p.Desc
	bucketLeafPageN         *p.Desc
	bucketLeafOverflowN     *p.Desc
	bucketKeyN              *p.Desc
	bucketDepth             *p.Desc
	bucketBranchAlloc       *p.Desc
	bucketBranchInuse       *p.Desc
	bucketLeafAlloc         *p.Desc
	bucketLeafInuse         *p.Desc
	bucketBucketN           *p.Desc
	bucketInlineBucketN     *p.Desc
	bucketInlineBucketInuse *p.Desc
	bucketSequence          *p.Desc
}

func NewMetricsCollector() p.Collector {

	bucketLabels := []string{"bucket"}

	return &metricsCollector{
		freePageN:     p.NewDesc("boltdb_freePageN", "boltdb total number of free pages on the freelist", nil, nil),
		pendigPageN:   p.NewDesc("boltdb_pendigPageN", "boltdb total number of pending pages on the freelist", nil, nil),
		freeAlloc:     p.NewDesc("boltdb_freeAlloc", "boltdb total bytes allocated in free pages", nil, nil),
		freelistInuse: p.NewDesc("boltdb_freelistInuse", "boltdb total bytes used by the freelist", nil, nil),
		txN:           p.NewDesc("boltdb_txN", "boltdb total number of started read transactions", nil, nil),
		openTxN:       p.NewDesc("boltdb_openTxN", "boltdb number of currently open read transactions", nil, nil),
		pageCount:     p.NewDesc("boltdb_pageCount", "boltdb number of page allocations", nil, nil),
		pageAlloc:     p.NewDesc("boltdb_pageAlloc", "boltdb total bytes allocated", nil, nil),
		cursorCount:   p.NewDesc("boltdb_cursorCount", "boltdb number of cursors created", nil, nil),
		nodeCount:     p.NewDesc("boltdb_nodeCount", "boltdb number of node allocations", nil, nil),
		nodeDeref:     p.NewDesc("boltdb_nodeDeref", "boltdb number of node dereferences", nil, nil),
		rebalance:     p.NewDesc("boltdb_rebalance", "boltdb number of node rebalances", nil, nil),
		rebalanceTime: p.NewDesc("boltdb_rebalanceTime", "boltdb total time spent rebalancing", nil, nil),
		split:         p.NewDesc("boltdb_split", "boltdb number of nodes split", nil, nil),
		spill:         p.NewDesc("boltdb_spill", "boltdb number of nodes spilled", nil, nil),
		spillTime:     p.NewDesc("boltdb_spillTime", "boltdb total time spent spilling", nil, nil),
		write:         p.NewDesc("boltdb_write", "boltdb number of writes performed", nil, nil),
		writeTime:     p.NewDesc("boltdb_writeTime", "boltdb total time spent writing to disk", nil, nil),

		bucketBranchPageN:       p.NewDesc("boltdb_bucket_BranchPageN", "number of logical branch pages", bucketLabels, nil),
		bucketBranchOverflowN:   p.NewDesc("boltdb_bucket_BranchOverflowN", "number of physical branch overflow pages", bucketLabels, nil),
		bucketLeafPageN:         p.NewDesc("boltdb_bucket_LeafPageN", "number of logical leaf pages", bucketLabels, nil),
		bucketLeafOverflowN:     p.NewDesc("boltdb_bucket_LeafOverflowN", "number of physical leaf overflow pages", bucketLabels, nil),
		bucketKeyN:              p.NewDesc("boltdb_bucket_KeyN", "number of keys/value pairs", bucketLabels, nil),
		bucketDepth:             p.NewDesc("boltdb_bucket_Depth", "number of levels in B+tree", bucketLabels, nil),
		bucketBranchAlloc:       p.NewDesc("boltdb_bucket_BranchAlloc", "bytes allocated for physical branch pages", bucketLabels, nil),
		bucketBranchInuse:       p.NewDesc("boltdb_bucket_BranchInuse", "bytes actually used for branch data", bucketLabels, nil),
		bucketLeafAlloc:         p.NewDesc("boltdb_bucket_LeafAlloc", "bytes allocated for physical leaf pages", bucketLabels, nil),
		bucketLeafInuse:         p.NewDesc("boltdb_bucket_LeafInuse", "bytes actually used for leaf data", bucketLabels, nil),
		bucketBucketN:           p.NewDesc("boltdb_bucket_BucketN", "total number of buckets including the top bucket", bucketLabels, nil),
		bucketInlineBucketN:     p.NewDesc("boltdb_bucket_InlineBucketN", "total number on inlined buckets", bucketLabels, nil),
		bucketInlineBucketInuse: p.NewDesc("boltdb_bucket_InlineBucketInuse", "bytes used for inlined buckets (also accounted for in LeafInuse)", bucketLabels, nil),
		bucketSequence:          p.NewDesc("boltdb_bucket_sequence", "current integer for the bucket", bucketLabels, nil),
	}
}

func (m *metricsCollector) Describe(ch chan<- *p.Desc) {
	ch <- m.freePageN
	ch <- m.pendigPageN
	ch <- m.freeAlloc
	ch <- m.freelistInuse
	ch <- m.txN
	ch <- m.openTxN
	ch <- m.pageCount
	ch <- m.pageAlloc
	ch <- m.cursorCount
	ch <- m.nodeCount
	ch <- m.nodeDeref
	ch <- m.rebalance
	ch <- m.rebalanceTime
	ch <- m.split
	ch <- m.spill
	ch <- m.spillTime
	ch <- m.write
	ch <- m.writeTime

	ch <- m.bucketBranchPageN
	ch <- m.bucketBranchOverflowN
	ch <- m.bucketLeafPageN
	ch <- m.bucketLeafOverflowN
	ch <- m.bucketKeyN
	ch <- m.bucketDepth
	ch <- m.bucketBranchAlloc
	ch <- m.bucketBranchInuse
	ch <- m.bucketLeafAlloc
	ch <- m.bucketLeafInuse
	ch <- m.bucketBucketN
	ch <- m.bucketInlineBucketN
	ch <- m.bucketInlineBucketInuse
	ch <- m.bucketSequence
}

func (m *metricsCollector) Collect(ch chan<- p.Metric) {
	if db == nil || db.db == nil {
		return
	}
	stats := db.db.Stats()
	ch <- p.MustNewConstMetric(m.freePageN, p.CounterValue, float64(stats.FreePageN))
	ch <- p.MustNewConstMetric(m.pendigPageN, p.CounterValue, float64(stats.PendingPageN))
	ch <- p.MustNewConstMetric(m.freeAlloc, p.CounterValue, float64(stats.FreeAlloc))
	ch <- p.MustNewConstMetric(m.freelistInuse, p.CounterValue, float64(stats.FreelistInuse))
	ch <- p.MustNewConstMetric(m.txN, p.CounterValue, float64(stats.TxN))
	ch <- p.MustNewConstMetric(m.openTxN, p.GaugeValue, float64(stats.OpenTxN))
	ch <- p.MustNewConstMetric(m.pageCount, p.GaugeValue, float64(stats.TxStats.PageCount))
	ch <- p.MustNewConstMetric(m.pageAlloc, p.CounterValue, float64(stats.TxStats.PageAlloc))
	ch <- p.MustNewConstMetric(m.cursorCount, p.CounterValue, float64(stats.TxStats.CursorCount))
	ch <- p.MustNewConstMetric(m.nodeCount, p.CounterValue, float64(stats.TxStats.NodeCount))
	ch <- p.MustNewConstMetric(m.nodeDeref, p.CounterValue, float64(stats.TxStats.NodeDeref))
	ch <- p.MustNewConstMetric(m.rebalance, p.CounterValue, float64(stats.TxStats.Rebalance))
	ch <- p.MustNewConstMetric(m.rebalanceTime, p.CounterValue, float64(stats.TxStats.RebalanceTime))
	ch <- p.MustNewConstMetric(m.split, p.CounterValue, float64(stats.TxStats.Split))
	ch <- p.MustNewConstMetric(m.spill, p.CounterValue, float64(stats.TxStats.Spill))
	ch <- p.MustNewConstMetric(m.spillTime, p.CounterValue, float64(stats.TxStats.SpillTime))
	ch <- p.MustNewConstMetric(m.write, p.CounterValue, float64(stats.TxStats.Write))
	ch <- p.MustNewConstMetric(m.writeTime, p.CounterValue, float64(stats.TxStats.WriteTime))

	db.db.View(func(tx *bolt.Tx) error {
		tx.ForEach(func(name []byte, b *bolt.Bucket) error {
			stats := b.Stats()
			bucket := string(name)
			ch <- p.MustNewConstMetric(m.bucketBranchPageN, p.GaugeValue, float64(stats.BranchPageN), bucket)
			ch <- p.MustNewConstMetric(m.bucketBranchOverflowN, p.GaugeValue, float64(stats.BranchOverflowN), bucket)
			ch <- p.MustNewConstMetric(m.bucketLeafPageN, p.GaugeValue, float64(stats.LeafPageN), bucket)
			ch <- p.MustNewConstMetric(m.bucketLeafOverflowN, p.GaugeValue, float64(stats.LeafOverflowN), bucket)
			ch <- p.MustNewConstMetric(m.bucketKeyN, p.GaugeValue, float64(stats.KeyN), bucket)
			ch <- p.MustNewConstMetric(m.bucketDepth, p.GaugeValue, float64(stats.Depth), bucket)
			ch <- p.MustNewConstMetric(m.bucketBranchAlloc, p.GaugeValue, float64(stats.BranchAlloc), bucket)
			ch <- p.MustNewConstMetric(m.bucketBranchInuse, p.GaugeValue, float64(stats.BranchInuse), bucket)
			ch <- p.MustNewConstMetric(m.bucketLeafAlloc, p.GaugeValue, float64(stats.LeafAlloc), bucket)
			ch <- p.MustNewConstMetric(m.bucketLeafInuse, p.GaugeValue, float64(stats.LeafInuse), bucket)
			ch <- p.MustNewConstMetric(m.bucketBucketN, p.GaugeValue, float64(stats.BucketN), bucket)
			ch <- p.MustNewConstMetric(m.bucketInlineBucketN, p.GaugeValue, float64(stats.InlineBucketN), bucket)
			ch <- p.MustNewConstMetric(m.bucketInlineBucketInuse, p.GaugeValue, float64(stats.InlineBucketInuse), bucket)
			ch <- p.MustNewConstMetric(m.bucketSequence, p.GaugeValue, float64(b.Sequence()), bucket)
			return nil
		})
		return nil
	})
}
