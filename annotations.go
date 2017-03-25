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
	}
)

func (a *AnnotationHandler) onPost(w http.ResponseWriter, r *http.Request) (int, interface{}) {
	ar := &annotationReq{}
	if err := json.NewDecoder(r.Body).Decode(ar); err != nil {
		log.Errorf("unmarshal error: %s", err)
		return 442, "bad request"
	}

	from, _ := parseTime(ar.Range.From)
	to, _ := parseTime(ar.Range.To)
	name := ar.Annotation.Name

	events := GetEvents(from, to, name)

	resp := make([]annotationResp, len(events))
	for i, e := range events {
		resp[i] = annotationResp{
			Annotation: ar.Annotation,
			Title:      e.Title,
			Time:       e.Time / 1000000,
			Text:       e.Text,
			Tags:       e.Tags,
		}
	}
	return http.StatusOK, resp
}

func (a AnnotationHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	code := http.StatusNotFound
	var data interface{}

	switch r.Method {
	case "POST":
		code, data = a.onPost(w, r)
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(code)
	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			log.Errorf("encoding error: %s", err)
		}
	}
}
