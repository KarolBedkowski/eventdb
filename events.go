//
// events.go
// Copyright (C) 2017 Karol Będkowski
//
// Distributed under terms of the GPLv3 license.
//

package main

import (
	"encoding/json"
	"net/http"
	_ "net/http/pprof"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

var (
	eventRequest = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "eventdb_request_total",
			Help: "Total numbers requests",
		},
	)
	eventPosts = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "eventdb_posts_total",
			Help: "Total number events posted",
		},
	)
	eventPostsErrors = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "eventdb_posts_errors_total",
			Help: "Total number errors in posts",
		},
	)
)

func init() {
	prometheus.MustRegister(eventRequest)
	prometheus.MustRegister(eventPosts)
	prometheus.MustRegister(eventPostsErrors)
}

type (
	eventsHandler struct {
		Configuration *Configuration
	}

	eventReq struct {
		Name  string
		Title string
		Time  interface{}
		Text  string
		Tags  string
	}
)

func numToTs(ts int64) int64 {
	if ts > 1000000000000000 { // nanos
		return ts
	} else if ts > 1000000000000 { // micros
		return ts * 1000
	} else if ts > 1000000000 { // milils
		return ts * 1000000
	}
	return ts * 1000000000
}

func (e *eventsHandler) onPost(w http.ResponseWriter, r *http.Request) (int, interface{}) {
	ev := &eventReq{}
	if err := json.NewDecoder(r.Body).Decode(ev); err != nil {
		log.Errorf("unmarshal error: %s", err)
		return 442, "bad request"
	}

	event := &Event{
		Name:  ev.Name,
		Title: ev.Title,
		Text:  ev.Text,
		Tags:  ev.Tags,
	}

	switch ev.Time.(type) {
	case int64:
		event.Time = numToTs(ev.Time.(int64))
	case float64:
		event.Time = numToTs(int64(ev.Time.(float64)))
	case string:
		if t, err := time.Parse(time.RFC3339Nano, ev.Time.(string)); err == nil {
			event.Time = t.UnixNano()
		} else {
			log.Errorf("Parsing %+v error %s", ev.Time, err)
		}
	}

	if event.Time == 0 {
		log.Errorf("wrong time %+v", ev.Time)
		return http.StatusBadRequest, "wrong time"
	}

	if err := SaveEvent(event); err == nil {
		return http.StatusCreated, "ok"
	}

	return http.StatusInternalServerError, "error"
}

func (e *eventsHandler) onGet(w http.ResponseWriter, r *http.Request) (int, interface{}) {
	r.ParseForm()
	vars := r.Form

	var from, to time.Time

	if fts, err := parseTime(vars.Get("from")); err == nil {
		from = fts
	} else {
		from = time.Unix(0, 0)
	}
	if tts, err := parseTime(vars.Get("to")); err == nil {
		to = tts
	} else {
		to = time.Now()
	}
	name := vars.Get("name")

	events := GetEvents(from, to, name)
	return http.StatusOK, events
}

func (e eventsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	code := http.StatusNotFound
	var data interface{}

	switch r.Method {
	case "POST":
		eventPosts.Inc()
		code, data = e.onPost(w, r)
		if code < 200 || code >= 300 {
			eventPostsErrors.Inc()
		}
	case "GET":
		code, data = e.onGet(w, r)
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(code)
	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			log.Errorf("encoding error: %s", err)
		}
	}
}
