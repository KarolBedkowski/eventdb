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
)

type (
	annotationReqRange struct {
		From string `json:"from"`
		To   string `json:"to"`
	}

	annotation struct {
		Datasource string `json:"datasource"`
		Enable     bool   `json:"enable"`
		Name       string `json:"name"`
	}

	annotationReq struct {
		Range      annotationReqRange `json:"range"`
		RangeRaw   annotationReqRange `json:"rangeRaw"`
		Annotation annotation         `json:"annotation"`
	}

	annotationResp struct {
		Annotation annotation `json:"annotation"`
		Title      string     `json:"title"`
		// Time in milliseconds
		Time int64  `json:"time"`
		Text string `json:"text"`
		Tags string `json:"tags"`
	}

	// AnnotationHandler for grafana annotations requests
	AnnotationHandler struct {
		DB *DB
	}
)

func (a *AnnotationHandler) onPost(w http.ResponseWriter, r *http.Request, l log.Logger) (int, interface{}) {
	l = l.With("action", "AnnotationHandler.onPost")

	ar := &annotationReq{}
	if err := json.NewDecoder(r.Body).Decode(ar); err != nil {
		l.Errorf("unmarshal error: %s", err)
		return 442, "bad request"
	}

	l.Debugf("annotation post %+v", ar)

	from, err := parseTime(ar.Range.From)
	if err != nil {
		l.Debugf("wrong from date: %s", err.Error())
		return http.StatusBadRequest, "wrong from date: " + err.Error()
	}
	to, err := parseTime(ar.Range.To)
	if err != nil {
		l.Debugf("wrong to date: %s", err.Error())
		return http.StatusBadRequest, "wrong to date: " + err.Error()
	}

	name, tags := parseName(ar.Annotation.Name)

	events, err := a.DB.GetEvents(from, to, name)
	if err != nil {
		l.Errorf("get events (%v, %v, %v) error: %s", from, to, name, err.Error())
		return http.StatusBadRequest, "error"
	}

	resp := make([]annotationResp, 0, len(events))
	for _, e := range events {
		if !e.CheckTags(tags) {
			continue
		}
		resp = append(resp, annotationResp{
			Annotation: ar.Annotation,
			Title:      e.Title,
			Time:       e.Time / 1000000,
			Text:       e.Text,
			Tags:       e.Tags,
		})
	}

	return http.StatusOK, resp
}

func (a *AnnotationHandler) onOptions(w http.ResponseWriter, r *http.Request, l log.Logger) (int, interface{}) {
	return http.StatusOK, ""
}

func (a AnnotationHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
