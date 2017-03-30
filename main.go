// Copyright (C) 2017 Karol Będkowski
//
// Distributed under terms of the GPLv3 license.
package main

import (
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
)

var (
	showVersion = flag.Bool("version", false, "Print version information.")
	configFile  = flag.String("config.file", "eventdb.yml",
		"Path to configuration file.")
	listenAddress = flag.String("web.listen-address", ":9701",
		"Address to listen on for web interface and telemetry.")
)

func init() {
	prometheus.MustRegister(version.NewCollector("eventdb"))
}

func main() {
	flag.Parse()

	if *showVersion {
		fmt.Fprintln(os.Stdout, version.Print("eventdb"))
		os.Exit(0)
	}

	log.Infoln("Starting eventdb", version.Info())
	log.Infoln("Build context", version.BuildContext())

	c, err := LoadConfiguration(*configFile)
	if err != nil {
		log.Fatalf("Error parsing config file: %s", err)
	}

	db, err := DBOpen(c.DBFile)
	if err != nil {
		panic(err)
	}

	defer db.Close()

	vw := vacuumWorker{Configuration: c, DB: db}
	vw.Start()

	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/" {
			http.NotFound(w, req)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	apiHandler := eventsHandler{Configuration: c, DB: db}
	http.Handle("/api/v1/event", prometheus.InstrumentHandler("api-v1-event", apiHandler))

	ah := AnnotationHandler{DB: db}
	http.Handle("/annotations", prometheus.InstrumentHandler("annotations", ah))

	pwh := PromWebHookHandler{DB: db}
	http.Handle("/api/v1/promwebhook", prometheus.InstrumentHandler("api-v1-promwebhook", pwh))

	hh := humanEventsHandler{Configuration: c, DB: db}
	http.Handle("/last", hh)

	http.Handle("/metrics", promhttp.Handler())

	// database endpoints
	http.Handle("/db/", http.StripPrefix("/db", db.NewInternalsHandler()))

	// handle hup for reloading configuration
	hup := make(chan os.Signal)
	signal.Notify(hup, syscall.SIGHUP)
	go func() {
		for {
			select {
			case <-hup:
				if newConf, err := LoadConfiguration(*configFile); err == nil {
					log.Debugf("new configuration: %+v", newConf)
					apiHandler.Configuration = newConf
					vw.Configuration = newConf
					hh.Configuration = newConf
					log.Info("configuration reloaded")
				} else {
					log.Errorf("reloading configuration err: %s", err)
					log.Errorf("using old configuration")
				}
			}
		}
	}()

	log.Infof("Listening on %s", *listenAddress)
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}

type vacuumWorker struct {
	Configuration *Configuration
	DB            *DB
}

func (v *vacuumWorker) Start() {
	deletedCntr := prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "eventdb_vacuum_events_deleted_total",
			Help: "Total number events deleted by vacuum worker",
		},
	)

	lastRun := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "eventdb_vacuum_last_run_unix",
			Help: "Last run of vacuum routine.",
		},
	)

	prometheus.MustRegister(deletedCntr)
	prometheus.MustRegister(lastRun)

	go func() {
		time.Sleep(1 * time.Minute)
		for {
			if v.Configuration.Retention != "" {
				retention, err := time.ParseDuration(v.Configuration.Retention)
				if err == nil {
					to := time.Now().Add(-retention)
					from := time.Time{}
					if deleted, err := v.DB.DeleteEvents(from, to, AnyBucket); err == nil {
						log.Infof("vacuum deleted %d to %s", deleted, to)
						deletedCntr.Add(float64(deleted))
					} else {
						log.Errorf("vacuum delete error: %s", err.Error())
					}
					lastRun.SetToCurrentTime()
				} else {
					log.Errorf("vacuumWorker parse duration error: %s", err.Error())
				}
				time.Sleep(3 * time.Hour)
			}
		}
	}()
}
