//
// web.go
// Copyright (C) Karol Będkowski, 2017
//
// Distributed under terms of the GPLv3 license.
//

package main

import (
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/common/log"
)

type queryPageHandler struct {
	Configuration *Configuration
	DB            *DB
}

const defaultTSFormat = "2006-01-02 15:04:05 -0700"

func (h queryPageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	l := log.With("remote", r.RemoteAddr).
		With("req", r.RequestURI).
		With("action", "queryPageHandler.ServeHTTP")

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
