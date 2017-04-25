//
// events.go
// Copyright (C) 2017 Karol Będkowski
//
// Distributed under terms of the GPLv3 license.
//

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

var (
	eventsAdded = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "eventdb_events_created_total",
			Help: "Total number events posted",
		},
		[]string{"src"},
	)
	eventAddError = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "eventdb_events_failed_total",
			Help: "Total number errors when creating events",
		},
	)
)

func init() {
	prometheus.MustRegister(eventsAdded)
	prometheus.MustRegister(eventAddError)
}

type (
	eventsHandler struct {
		Configuration *Configuration
		DB            *DB
	}

	eventReq struct {
		Name  string
		Title string
		Time  interface{}
		Text  string
		Tags  string
	}
)

func (e *eventsHandler) onPost(w http.ResponseWriter, r *http.Request, l log.Logger) (int, interface{}) {
	l = l.With("action", "eventsHandler.onPost")

	ev := &eventReq{}
	if err := json.NewDecoder(r.Body).Decode(ev); err != nil {
		l.Debugf("body decode error: %s", err)
		return 442, "bad request"
	}

	eventBase := &EventBase{
		Name:  ev.Name,
		Title: ev.Title,
		Text:  ev.Text,
		Tags:  ev.Tags,
	}

	event := &Event{
		EventBase: eventBase,
	}

	switch ev.Time.(type) {
	case int64:
		event.Time = numToUnixNano(ev.Time.(int64))
	case float64:
		event.Time = numToUnixNano(int64(ev.Time.(float64)))
	case string:
		if t, err := time.Parse(time.RFC3339Nano, ev.Time.(string)); err == nil {
			event.Time = t.UnixNano()
		} else {
			l.Debugf("parsing time %+v error %s", ev.Time, err)
			return http.StatusBadRequest, "wrong time"
		}
	}

	if event.Time == 0 {
		l.Debugf("wrong time %+v", ev.Time)
		return http.StatusBadRequest, "wrong time"
	}

	if e.Configuration.RetentionParsed != nil {
		minDate := time.Now().Add(-(*e.Configuration.RetentionParsed)).UnixNano()
		if minDate > event.Time {
			log.Debugf("date %s before retention time - skipping", ev.Time)
			return http.StatusNotModified, "not inserted due retention time"
		}
	}

	if err := e.DB.SaveEvent(event); err == nil {
		eventsAdded.WithLabelValues("api-v1-event-post").Inc()
		return http.StatusCreated, "ok"
	} else {
		log.Errorf("save event error: %s", err.Error())
	}

	eventAddError.Inc()
	return http.StatusInternalServerError, "error"
}

type eventsOnGetRespHeader struct {
	From time.Time
	To   time.Time
	Name string
	Tags []string
}

type eventsOnGetResp struct {
	Header *eventsOnGetRespHeader
	Events []*Event
}

func (e *eventsHandler) onGet(w http.ResponseWriter, r *http.Request, l log.Logger) (int, interface{}) {
	l = l.With("action", "eventsHandler.onGet")

	r.ParseForm()
	vars := r.Form

	to := time.Now()
	from := to.AddDate(0, 0, -1)

	if vfrom := vars.Get("from"); vfrom != "" {
		if fts, err := parseTime(vfrom); err == nil {
			from = fts
		} else {
			l.Debugf("wrong from date: %s", err.Error())
			return http.StatusBadRequest, "wrong from date"
		}
	}
	if vto := vars.Get("to"); vto != "" {
		if tts, err := parseTime(vto); err == nil {
			to = tts
		} else {
			l.Debugf("wrong to date: %s", err.Error())
			return http.StatusBadRequest, "wrong to date"
		}
	}

	name, tags := parseName(vars.Get("name"))

	events, _ := e.DB.GetEvents(from, to, name)
	if tags == nil || len(tags) == 0 {
		return http.StatusOK, events
	}

	filteredEvents := make([]*Event, 0, len(events))
	for _, e := range events {
		if e.CheckTags(tags) {
			filteredEvents = append(filteredEvents, e)
		}
	}

	response := &eventsOnGetResp{
		Header: &eventsOnGetRespHeader{
			From: from,
			To:   to,
			Name: name,
			Tags: tags,
		},
		Events: filteredEvents,
	}

	return http.StatusOK, response
}

func (e *eventsHandler) onDelete(w http.ResponseWriter, r *http.Request, l log.Logger) (int, interface{}) {
	l = l.With("action", "eventsHandler.onDelete")

	r.ParseForm()
	vars := r.Form

	var from, to time.Time

	if fts, err := parseTime(vars.Get("from")); err == nil {
		from = fts
	} else {
		l.Debugf("wrong from date: %s", err.Error())
		return http.StatusBadRequest, "wrong 'from' date"
	}
	if tts, err := parseTime(vars.Get("to")); err == nil {
		to = tts
	} else {
		l.Debugf("wrong to date: %s", err.Error())
		return http.StatusBadRequest, "wrong 'to' date"
	}

	if to.Before(from) {
		l.Debugf("wrong to dates to < from")
		return http.StatusBadRequest, "'to' < 'from'"
	}

	name := vars.Get("name")
	res := &struct {
		Deleted int
	}{}
	if deleted, err := e.DB.DeleteEvents(from, to, name); err == nil {
		res.Deleted = deleted
	} else {
		l.Errorf("delete error: %s", err.Error())
		return http.StatusInternalServerError, "delete error"
	}
	return http.StatusOK, res
}

func (e eventsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	l := log.With("remote", r.RemoteAddr).With("req", r.RequestURI)

	code := http.StatusNotFound
	var data interface{}

	switch r.Method {
	case "POST":
		code, data = e.onPost(w, r, l)
	case "GET":
		code, data = e.onGet(w, r, l)
	case "DELETE":
		code, data = e.onDelete(w, r, l)
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(code)
	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			l.Errorf("encoding result error: %s", err)
		}
	}
}

type humanEventsHandler struct {
	Configuration *Configuration
	DB            *DB
}

func (h humanEventsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	l := log.With("remote", r.RemoteAddr).
		With("req", r.RequestURI).
		With("action", "humanEventsHandler.ServeHTTP")

	r.ParseForm()
	vars := r.Form

	to := time.Now()
	from := to.Add(time.Duration(-2) * time.Hour)

	name, tags := parseName(vars.Get("name"))
	if name == "" {
		name = "_any_"
	}

	events, err := h.DB.GetEvents(from, to, name)
	if err != nil {
		l.Errorf("get events error: %s", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
	w.Write([]byte(fmt.Sprintf("Events for %s from %s to %s\n\n", name, from, to)))

	for i, e := range events {
		if !e.CheckTags(tags) {
			continue
		}
		ts := time.Unix(0, e.Time)
		w.Write([]byte(fmt.Sprintf("%d. %s   Name: %v\nTitle: %s\nText: %s\nTags: %s\n",
			(i + 1), ts, e.Name, e.Title, e.Text, e.Tags)))
		if h.Configuration.Debug {
			w.Write([]byte(fmt.Sprintf("bucket: %s   key: %x\n", string(e.bucket), e.key)))
		}
		w.Write([]byte{'\n', '\n'})
	}
}
