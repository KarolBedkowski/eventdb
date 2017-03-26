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
	KV map[string]string

	Alert struct {
		Status       string    `json:"status"`
		Labels       KV        `json:"labels"`
		Annotations  KV        `json:"annotations"`
		StartsAt     time.Time `json:"startsAt"`
		EndsAt       time.Time `json:"endsAt"`
		GeneratorURL string    `json:"generatorURL"`
	}

	Alerts []Alert

	WebhookMessage struct {
		Receiver string `json:"receiver"`
		Status   string `json:"status"`
		Alerts   Alerts `json:"alerts"`

		GroupLabels       KV `json:"groupLabels"`
		CommonLabels      KV `json:"commonLabels"`
		CommonAnnotations KV `json:"commonAnnotations"`

		ExternalURL string `json:"externalURL"`

		Version  string `json:"version"`
		GroupKey uint64 `json:"groupKey"`
	}

	PromWebHookHandler struct {
	}
)

func (k KV) String() string {
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

func (p *PromWebHookHandler) onPost(w http.ResponseWriter, r *http.Request) (int, interface{}) {
	m := &WebhookMessage{}
	if err := json.NewDecoder(r.Body).Decode(m); err != nil {
		log.Errorf("unmarshal error: %s", err)
		return 442, "bad request"
	}

	log.Debugf("new req from prom: %+v", m)

	for _, a := range m.Alerts {
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
			e.Tags = strings.TrimSpace(v)
		}
		if v, ok := a.Labels["name"]; ok {
			e.Name = strings.TrimSpace(v)
		}
		if err := SaveEvent(e); err != nil {
			log.Errorf("save event error: %s", err)
			eventAddError.Inc()
		} else {
			eventsAdded.WithLabelValues("api-v1-promwebhook-post").Inc()
		}
	}

	return http.StatusOK, m
}

func (p PromWebHookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	code := http.StatusNotFound
	var data interface{}

	switch r.Method {
	case "POST":
		code, data = p.onPost(w, r)
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(code)
	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			log.Errorf("encoding error: %s", err)
		}
	}
}
