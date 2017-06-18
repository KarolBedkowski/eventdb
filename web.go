//
// web.go
// Copyright (C) Karol Będkowski, 2017
//
// Distributed under terms of the GPLv3 license.
//

package main

import (
	"github.com/pkg/errors"
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

type queryPageData struct {
	Events []*Event
	Query  string
	From   string
	To     string
	Error  string

	toT   time.Time
	fromT time.Time
}

const defaultTSFormat = "2006-01-02 15:04:05 -0700"

func (h queryPageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	l := log.With("remote", r.RemoteAddr).
		With("req", r.RequestURI).
		With("action", "queryPageHandler.ServeHTTP")

	t, terr := template.New("webpage").Parse(tpl)
	if terr != nil {
		l.Errorf("template parse error: %s", terr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	r.ParseForm()

	var err error
	var data *queryPageData

	data, err = h.parseInput(r)
	if err == nil && r.Form.Get("query") != "" {
		err = data.loadEvents(&h)
	}

	if err != nil {
		l.Infof("parse & load data error: %s", err)
		data.Error = err.Error()
	}

	err = t.Execute(w, data)
	if err != nil {
		l.Errorf("template execute error: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (h *queryPageHandler) parseInput(r *http.Request) (data *queryPageData, err error) {
	vars := r.Form

	data = &queryPageData{
		To:    vars.Get("to"),
		From:  vars.Get("from"),
		Query: vars.Get("query"),
	}

	if data.To == "" {
		data.toT = time.Now()
	} else {
		data.toT, err = parseTime(data.To)
		if err != nil {
			return data, errors.Wrap(err, "parse TO error")
		}
	}

	if data.From == "" {
		data.fromT = data.toT.Add(time.Duration(-1) * time.Hour)
		data.From = data.fromT.Format(defaultTSFormat)
	} else {
		data.fromT, err = parseTime(data.From)
		if err != nil {
			return data, errors.Wrap(err, "parse FROM error")
		}
	}

	if data.Query == "" {
		buckets, _ := h.DB.Buckets()
		data.Query = strings.Join(buckets, ";")
	}

	return
}

func (d *queryPageData) loadEvents(h *queryPageHandler) (err error) {
	var q *Query
	q, err = ParseQuery(d.Query)
	if err != nil {
		return errors.Wrap(err, "parse query error")
	}

	d.Events, err = q.Execute(h.DB, d.fromT, d.toT)
	if err != nil {
		return errors.Wrap(err, "get events error")
	}

	SortEventsByTime(d.Events)
	return
}

const tpl = `
<!DOCTYPE HTML>
<html>
<head>
	<meta charset="utf-8">
	<title>EventDB</title>
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<style type="text/css">body{margin:20px auto;line-height:1.6;font-size:12px;color:#444;padding:0 10px}h1,h2,h3{line-height:1.2}</style>
</head>
<body>
	<h1>EventDB</h1>
	<h2>Query</h2>
	{{ with .Error }}<p><strong>{{ . }}</strong></p>{{ end  }}
	<form>
		<label for="query">Query</label><br/>
		<textarea id="query" name="query" cols="80" rows="5">{{ .Query }}</textarea><br/>
		<label for="from">From:</label><br/>
		<input id="from" name="from" value="{{ .From }}" /><br/>
		<label for="to">To:</label><br/>
		<input id="to" name="to" value="{{ .To }}" /><br/><br/>
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
