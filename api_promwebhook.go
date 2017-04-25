//
// prom_webhook.go
// Copyright (C) 2017 Karol Będkowski
//
// Distributed under terms of the GPLv3 license.
//

package main

import (
	"encoding/json"
	"fmt"
	"github.com/prometheus/common/log"
	"net/http"
	"strings"
	"time"
)

type (
	kv map[string]string

	alert struct {
		Status       string    `json:"status"`
		Labels       kv        `json:"labels"`
		Annotations  kv        `json:"annotations"`
		StartsAt     time.Time `json:"startsAt"`
		EndsAt       time.Time `json:"endsAt"`
		GeneratorURL string    `json:"generatorURL"`
	}

	alerts []alert

	webhookMessage struct {
		Receiver string `json:"receiver"`
		Status   string `json:"status"`
		Alerts   alerts `json:"alerts"`

		GroupLabels       kv `json:"groupLabels"`
		CommonLabels      kv `json:"commonLabels"`
		CommonAnnotations kv `json:"commonAnnotations"`

		ExternalURL string `json:"externalURL"`

		Version  string `json:"version"`
		GroupKey uint64 `json:"groupKey"`
	}

	// PromWebHookHandler handle all request from AlertManager
	PromWebHookHandler struct {
		Configuration *Configuration
		DB            *DB
	}
)

func (k kv) String() string {
	out := make([]string, 0, len(k))
	for k, v := range k {
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if k != "" && v != "" {
			out = append(out, fmt.Sprintf("%s: %s", k, v))
		}
	}
	return strings.Join(out, "\n")
}

func (p *PromWebHookHandler) onPost(w http.ResponseWriter, r *http.Request, l log.Logger) (int, interface{}) {
	l = l.With("action", "PromWebHookHandler.onPost")

	m := &webhookMessage{}
	if err := json.NewDecoder(r.Body).Decode(m); err != nil {
		l.Debugf("decode body error: %s; %+v", err, r.Body)
		return 442, "bad request"
	}

	l.Debugf("new req from prom: %+v", m)

	minDate := time.Unix(0, 0)
	if p.Configuration.RetentionParsed != nil {
		minDate = time.Now().Add(-(*p.Configuration.RetentionParsed))
	}

	for _, a := range m.Alerts {
		if minDate.After(a.StartsAt) {
			l.Debugf("date %s before retention time - skipping", a.StartsAt)
			continue
		}

		e := &Event{
			Time: a.StartsAt.UnixNano(),
		}

		if v, ok := a.Annotations["summary"]; ok {
			e.Title = fmt.Sprintf("[%s] %s", a.Status, strings.TrimSpace(v))
		} else {
			e.Title = fmt.Sprintf("[%s]", a.Status)
		}
		if v, ok := a.Annotations["description"]; ok {
			e.Text = strings.TrimSpace(v)
		}
		if e.Text == "" {
			e.Text = a.Annotations.String()
		}
		e.Text += "\n\n" + a.Labels.String()
		if v, ok := a.Labels["tags"]; ok {
			e.SetTags(strings.TrimSpace(v))
		}
		if v, ok := a.Labels["name"]; ok {
			e.Name = strings.TrimSpace(v)
		}
		if err := p.DB.SaveEvent(e); err != nil {
			l.Errorf("save event error: %s", err)
			eventAddError.Inc()
		} else {
			eventsAdded.WithLabelValues("api-v1-promwebhook-post").Inc()
		}
	}

	return http.StatusOK, m
}

func (p PromWebHookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	l := log.With("remote", r.RemoteAddr).With("req", r.RequestURI)
	code := http.StatusNotFound
	var data interface{}

	switch r.Method {
	case "POST":
		code, data = p.onPost(w, r, l)
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(code)
	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			l.Errorf("encoding result error: %s", err)
		}
	}
}
