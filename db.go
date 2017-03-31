// Copyright (C) 2017 Karol Będkowski
//
// Distributed under terms of the GPLv3 license.
package main

import (
	"encoding/json"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/boltdb/boltd"
	p "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"net/http"
	"os"
	"strconv"
	"time"
)

type (
	DB struct {
		dbFilename string
		db         *bolt.DB
		stats      bolt.Stats
		statsDiff  bolt.Stats

		metrics *boltMetrics
	}
)

func DBOpen(filename string) (*DB, error) {
	bdb, err := bolt.Open(filename, 0600, &bolt.Options{Timeout: 10 * time.Second})
	if err != nil {
		return nil, err
	}

	err = bdb.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(defaultBucket); err != nil {
			return fmt.Errorf("db create bucket error: %s", err.Error())
		}
		return nil
	})
	if err != nil {
		bdb.Close()
		return nil, err
	}

	db := &DB{
		dbFilename: filename,
		db:         bdb,
		metrics:    newBoltMetrics(bdb),
		stats:      bdb.Stats(),
	}
	p.MustRegister(db.metrics)

	return db, nil
}

func (db *DB) Close() error {
	if db.db != nil {
		p.Unregister(db.metrics)
		db.metrics = nil
		db.db.Close()
		db.db = nil
	}
	return nil
}

func (db *DB) NewInternalsHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/backup", db.backupHandler)
	mux.HandleFunc("/stats", db.statsHandler)
	// Tests
	mux.Handle("/introspection/", http.StripPrefix("/introspection", boltd.NewHandler(db.db)))
	return mux
}

func (db *DB) backupHandler(w http.ResponseWriter, r *http.Request) {
	l := log.With("remote", r.RemoteAddr).With("req", r.RequestURI).
		With("action", "db.backupHandler")

	l.Debugf("start backup")
	err := db.db.View(func(tx *bolt.Tx) error {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", `attachment; filename="`+db.dbFilename+`"`)
		w.Header().Set("Content-Length", strconv.Itoa(int(tx.Size())))
		_, err := tx.WriteTo(w)
		return err
	})
	if err != nil {
		l.Errorf("backup error: %s", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	l.Debugf("backup finished")
}

func (db *DB) statsHandler(w http.ResponseWriter, r *http.Request) {
	l := log.With("remote", r.RemoteAddr).With("req", r.RequestURI).
		With("action", "db.statsHandler")
	l.Debugf("get stats")
	stats := db.db.Stats()
	db.statsDiff = db.stats.Sub(&db.stats)
	db.stats = stats
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(db.statsDiff)
}

type boltMetrics struct {
	instance *p.Desc
	fileSize *p.Desc

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

	db *bolt.DB
}

func newBoltMetrics(db *bolt.DB) *boltMetrics {
	bucketLabels := []string{"bucket"}

	return &boltMetrics{
		instance: p.NewDesc("boltdb_instance", "boltdb instance info", []string{"path"}, nil),
		fileSize: p.NewDesc("boltdb_db_file_size", "boltdb database file size", nil, nil),

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

		db: db,
	}
}

func (m *boltMetrics) Describe(ch chan<- *p.Desc) {
	ch <- m.instance
	ch <- m.fileSize

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

func (m *boltMetrics) Collect(ch chan<- p.Metric) {
	if m.db == nil {
		return
	}
	ch <- p.MustNewConstMetric(m.instance, p.GaugeValue, float64(1), m.db.Path())

	if stat, err := os.Lstat(m.db.Path()); err == nil {
		ch <- p.MustNewConstMetric(m.fileSize, p.GaugeValue, float64(stat.Size()))
	}

	stats := m.db.Stats()
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

	m.db.View(func(tx *bolt.Tx) error {
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
