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

	AnnotationHandler struct {
		DB *DB
	}
)

func (a *AnnotationHandler) onPost(w http.ResponseWriter, r *http.Request) (int, interface{}) {
	ar := &annotationReq{}
	if err := json.NewDecoder(r.Body).Decode(ar); err != nil {
		log.Errorf("unmarshal error: %s", err)
		return 442, "bad request"
	}

	log.Debugf("annotation post %v %+v", r.URL, ar)

	from, _ := parseTime(ar.Range.From)
	to, _ := parseTime(ar.Range.To)

	name, tags := parseName(ar.Annotation.Name)

	events := a.DB.GetEvents(from, to, name)

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

func (a *AnnotationHandler) onOptions(w http.ResponseWriter, r *http.Request) (int, interface{}) {
	return http.StatusOK, ""
}

func (a AnnotationHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	code := http.StatusNotFound
	var data interface{}

	switch r.Method {
	case "POST":
		code, data = a.onPost(w, r)
	case "OPTIONS":
		code, data = a.onOptions(w, r)
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.Header().Add("Access-Control-Allow-Headers", "accept, content-type")
	w.Header().Add("Access-Control-Allow-Methods", "POST")
	w.Header().Add("Access-Control-Allow-Origin", "*")
	w.WriteHeader(code)

	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			log.Errorf("encoding error: %s", err)
		}
	}
}
