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
	searchReq struct {
		Target string `json:"target"`
	}

	// SearchHandler for grafana search tags requests
	SearchHandler struct {
		DB            *DB
		Configuration *Configuration
	}
)

func (s *SearchHandler) onPost(w http.ResponseWriter, r *http.Request, l log.Logger) (int, interface{}) {
	l = l.With("action", "SearchHandler.onPost")

	sr := &searchReq{}
	if err := json.NewDecoder(r.Body).Decode(sr); err != nil {
		l.Errorf("unmarshal error: %s", err)
		return 442, "bad request"
	}

	buckets, err := s.DB.Buckets()
	if err != nil {
		l.Info("get buckets error: %s", err)
		return http.StatusInternalServerError, err.Error()
	}
	return http.StatusOK, buckets
}

func (s *SearchHandler) onOptions(w http.ResponseWriter, r *http.Request, l log.Logger) (int, interface{}) {
	return http.StatusOK, ""
}

func (s SearchHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	l := log.With("remote", r.RemoteAddr).With("req", r.RequestURI)

	code := http.StatusNotFound
	var data interface{}

	switch r.Method {
	case "POST":
		code, data = s.onPost(w, r, l)
	case "OPTIONS":
		code, data = s.onOptions(w, r, l)
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
