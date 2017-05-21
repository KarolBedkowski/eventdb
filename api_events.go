//
// events.go
// Copyright (C) Karol Będkowski, 2017
//
// Distributed under terms of the GPLv3 license.
//

package main

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strings"
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
		Name        string
		Summary     string
		Time        interface{}
		Description string
		Tags        string
		Labels      map[string]string
	}
)

func (e *eventsHandler) onPost(w http.ResponseWriter, r *http.Request, l log.Logger) (int, interface{}) {
	l = l.With("action", "eventsHandler.onPost")

	ev := &eventReq{}
	if err := json.NewDecoder(r.Body).Decode(ev); err != nil {
		l.Debugf("body decode error: %s", err)
		return 442, "bad request"
	}

	event := &Event{
		Name:        ev.Name,
		Summary:     ev.Summary,
		Description: ev.Description,
	}

	for k, v := range ev.Labels {
		event.Cols = append(event.Cols, EventCol{k, v})
	}

	if ev.Tags != "" {
		event.SetTags(ev.Tags)
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

	if err := e.DB.SaveEvent(event); err != nil {
		log.Errorf("save event error: %s", err.Error())
		eventAddError.Inc()
		return http.StatusInternalServerError, "error"
	}

	eventsAdded.WithLabelValues("api-v1-event-post").Inc()
	return http.StatusCreated, "ok"
}

type eventsOnGetRespHeader struct {
	From  time.Time
	To    time.Time
	Query string
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

	query := vars.Get("query")
	q, err := ParseQuery(query)
	if err != nil {
		l.Infof("parse query error: %s", err.Error())
		return http.StatusBadRequest, "parse query error"
	}

	events, _ := q.Execute(e.DB, from, to)

	response := &eventsOnGetResp{
		Header: &eventsOnGetRespHeader{
			From:  from,
			To:    to,
			Query: query,
		},
		Events: events,
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

	query := vars.Get("query")
	q, err := ParseQuery(query)
	if err != nil {
		l.Infof("parse query error: %s", err.Error())
		return http.StatusBadRequest, "parse query error"
	}

	res := &struct {
		Deleted int
	}{}

	if deleted, err := q.ExecuteDelete(e.DB, from, to); err == nil {
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

const defaultTSFormat = "2006-01-02 15:04:05"

func (h humanEventsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	l := log.With("remote", r.RemoteAddr).
		With("req", r.RequestURI).
		With("action", "humanEventsHandler.ServeHTTP")

	r.ParseForm()
	vars := r.Form

	var toT, fromT time.Time
	var err error

	to := vars.Get("to")
	if to == "" {
		toT = time.Now()
		to = toT.Format(defaultTSFormat)
	} else {
		toT, err = parseTime(to)
		if err != nil {
			l.Errorf("parse to error: %s", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("bad to to date"))
			return
		}
	}

	from := vars.Get("from")
	if from == "" {
		fromT = toT.Add(time.Duration(-1) * time.Hour)
		from = fromT.Format(defaultTSFormat)
	} else {
		fromT, err = parseTime(from)
		if err != nil {
			l.Errorf("parse from error: %s", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("bad to from date"))
			return
		}
	}

	query := vars.Get("query")
	if query == "" {
		buckets, _ := h.DB.Buckets()
		query = strings.Join(buckets, ";")
	}

	q, err := ParseQuery(query)
	if err != nil {
		l.Infof("parse query error: %s", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	events, err := q.Execute(h.DB, fromT, toT)
	if err != nil {
		l.Errorf("get events error: %s", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	SortEventsByTime(events)

	t, err := template.New("webpage").Parse(tpl)
	if err != nil {
		l.Errorf("template parse error: %s", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	data := &struct {
		Events []*Event
		Query  string
		From   string
		To     string
	}{
		Events: events,
		Query:  query,
		From:   from,
		To:     to,
	}

	err = t.Execute(w, data)
}

type bucketsHandler struct {
	Configuration *Configuration
	DB            *DB
}

func (b *bucketsHandler) onGet(w http.ResponseWriter, r *http.Request, l log.Logger) (int, interface{}) {
	l = l.With("action", "bucketsHandler.onGet")

	buckets, err := b.DB.Buckets()
	if err != nil {
		l.Errorf("get events error: %s", err.Error())
		return http.StatusInternalServerError, nil
	}

	return http.StatusOK, buckets
}

func (b bucketsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	l := log.With("remote", r.RemoteAddr).With("req", r.RequestURI)

	code := http.StatusNotFound
	var data interface{}

	switch r.Method {
	case "GET":
		code, data = b.onGet(w, r, l)
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(code)
	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			l.Errorf("encoding result error: %s", err)
		}
	}
}

const tpl = `
<!DOCTYPE HTML>
<html>
<head>
	<meta charset="utf-8">
	<title>EventDB</title>
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<style type="text/css">body{margin:20px auto;max-width:1050px;line-height:1.6;font-size:12px;color:#444;padding:0 10px}h1,h2,h3{line-height:1.2}</style>
</head>
<body>
	<h1>EventDB</h1>
	<h2>Query</h2>
	<form>
		<label for="query">Query</label><br/>
		<textarea id="query" name="query" cols="80" rows="5">{{ .Query }}</textarea><br/>
		<label for="from">From:</label><br/>
		<input id="from" name="from" value="{{ .From }}" /></br>
		<label for="to">To:</label><br/>
		<input id="to" name="to" value="{{ .To }}" /></br>
		<button type="submit">Send</button>
	</form>
	<br/>
	<table border="1" cellspacing="0">
	<thead>
		<tr>
			<th>Name</th><th>TS</th><th>Summary</th><th>Description</th><th>Cols</th><th>Tags</th>
		</tr>
	</thead>
		{{range .Events}}
		<tr>
			<td>{{ .Name }}</td>
			<td>{{ .TS }}</td>
			<td>{{ .Summary  }}</td>
			<td>{{ .Description  }}</td>
			<td>{{ .Cols  }}</td>
			<td>{{ .Tags  }}</td>
		</tr>
		{{else}}
		<tr>
			<td colspan="6">No result</td>
		</tr>
		{{end}}
	</body>
</html>`
