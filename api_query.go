//
// annotations.go
// Copyright (C) 2017 Karol Będkowski
//
// Distributed under terms of the GPLv3 license.
//

package main

import (
	"encoding/json"
	"github.com/prometheus/common/log"
	"net/http"
	"time"
)

type (
	queryRangeReq struct {
		From string `json:"from"`
		To   string `json:"to"`
	}

	queryTargetReq struct {
		Target string `json:"target"`
		RefID  string `json:"refId"`
		Type   string `json:"type"`
	}

	queryReq struct {
		Range         *queryRangeReq    `json:"range"`
		Interval      string            `json:"interval"`
		Targets       []*queryTargetReq `json:"targets"`
		Format        string            `json:"format"`
		MaxDataPoints int               `json:"maxDataPoints"`
	}

	queryTargetResp struct {
		Target     string      `json:"target"`
		Datapoints [][]float64 `json:"datapoints"`

		prevTS int64
	}

	// QueryHandler for grafana query requests
	QueryHandler struct {
		DB            *DB
		Configuration *Configuration
	}
)

func (q *queryTargetResp) appendTS(ts int64, interval int64) {
	tts := ts
	if interval > 1 {
		tts = ts / interval
	}
	if q.prevTS == tts {
		last := len(q.Datapoints) - 1
		q.Datapoints[last][0]++
	} else {
		q.Datapoints = append(q.Datapoints, []float64{1, float64(ts / 1000000)})
		q.prevTS = tts
	}
}

func (a *QueryHandler) onPost(w http.ResponseWriter, r *http.Request, l log.Logger) (int, interface{}) {
	l = l.With("action", "QueryHandler.onPost")

	qr := &queryReq{}
	if err := json.NewDecoder(r.Body).Decode(qr); err != nil {
		l.Errorf("unmarshal error: %s", err)
		return 442, "bad request"
	}

	l.Debugf("query post %+v", qr)

	from, err := parseTime(qr.Range.From)
	if err != nil {
		l.Debugf("wrong from date: %s", err.Error())
		return http.StatusBadRequest, "wrong from date: " + err.Error()
	}
	to, err := parseTime(qr.Range.To)
	if err != nil {
		l.Debugf("wrong to date: %s", err.Error())
		return http.StatusBadRequest, "wrong to date: " + err.Error()
	}

	var interval int64
	if qr.Interval != "" {
		d, err := time.ParseDuration(qr.Interval)
		if err == nil {
			interval = int64(d.Seconds()) * 1000000000
		} else {
			l.Info("parse interval '%v' error: %s", qr.Interval, err)
		}
	}

	resp := make([]*queryTargetResp, len(qr.Targets))

	for i, target := range qr.Targets {

		l.Debugf("processing: %+v", target)

		if target.Type != "timeserie" {
			l.Info("invalid target type: %v", target.Type)
			continue
		}

		q, err := ParseQuery(target.Target)
		if err != nil {
			l.Infof("parse query error: %s", err.Error())
			return http.StatusBadRequest, "parse query error"
		}

		qtr := &queryTargetResp{
			Target:     target.Target,
			Datapoints: make([][]float64, 0),
		}

		events, _ := q.Execute(a.DB, from, to)

		SortEventsByTime(events)

		for _, e := range events {
			qtr.appendTS(e.Time, interval)
		}

		if qr.MaxDataPoints > 0 && len(qtr.Datapoints) > qr.MaxDataPoints {
			l.Debugf("limit datapoints for %s from %d to %d",
				target.Target, len(qtr.Datapoints), qr.MaxDataPoints)
			qtr.Datapoints = qtr.Datapoints[:qr.MaxDataPoints]
		}

		resp[i] = qtr
	}

	return http.StatusOK, resp
}

func (a *QueryHandler) onOptions(w http.ResponseWriter, r *http.Request, l log.Logger) (int, interface{}) {
	return http.StatusOK, ""
}

func (a QueryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	l := log.With("remote", r.RemoteAddr).With("req", r.RequestURI)

	code := http.StatusNotFound
	var data interface{}

	switch r.Method {
	case "POST":
		code, data = a.onPost(w, r, l)
	case "OPTIONS":
		code, data = a.onOptions(w, r, l)
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.Header().Add("Access-Control-Allow-Headers", "accept, content-type")
	w.Header().Add("Access-Control-Allow-Methods", "POST")
	w.Header().Add("Access-Control-Allow-Origin", "*")
	w.WriteHeader(code)

	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			l.Errorf("encoding error: %s", err)
		}
	}
}
