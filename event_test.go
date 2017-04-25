//
// event_test.go
// Copyright (C) 2017 Karol Będkowski <Karol Będkowski@kntbk>
//
// Distributed under terms of the MIT license.
//

package main

import (
	"log"
	"math/rand"
	"testing"
)

func generateEvents() []*Event {
	e := make([]*Event, 0, 100)
	for i := 0; i < 1000; i++ {
		e = append(e, &Event{
			EventBase: &EventBase{
				Name:  "aaa",
				Title: "bbbbbbbbbbbbbbbb",
				Time:  int64(i),
				Text:  "ccccccccccccccccccccccccccccccccccccccccc",
				Tags:  "1023091039013901930",
			},
		})
	}
	return e
}

func marshalGOBEvents() [][]byte {
	m := make([][]byte, 1000, 1000)
	var err error
	for i, e := range generateEvents() {
		m[i], _, err = e.encodeGOB()
		if err != nil {
			log.Fatalf("marshalGOBEvents error: %s", err)
		}
	}
	return m
}

func marshalEvents() [][]byte {
	m := make([][]byte, 1000, 1000)
	var err error
	for i, e := range generateEvents() {
		m[i], _, err = e.encode()
		if err != nil {
			log.Fatalf("marshalGOBEvents error: %s", err)
		}
	}
	return m
}

func BenchmarkMarshalGOB(b *testing.B) {
	b.StopTimer()
	data := generateEvents()
	b.ReportAllocs()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		e := data[i%1000]
		if _, _, err := e.encodeGOB(); err != nil {
			b.Fatalf("marshal error: %s", err.Error())
		}
	}
}

func BenchmarkMarshal(b *testing.B) {
	b.StopTimer()
	data := generateEvents()
	b.ReportAllocs()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		e := data[i%1000]
		if _, _, err := e.encode(); err != nil {
			b.Fatalf("marshal error: %s", err.Error())
		}
	}
}

func BenchmarkUnmarshalGOB(b *testing.B) {
	b.StopTimer()
	data := marshalGOBEvents()
	b.ReportAllocs()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		m := data[i%1000]
		if _, err := decodeEventGOB(m); err != nil {
			b.Fatalf("unmarshal error: %s", err.Error())
		}
	}
}

func BenchmarkUnmarshal(b *testing.B) {
	b.StopTimer()
	data := marshalEvents()
	b.ReportAllocs()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		m := data[i%1000]
		if _, err := decodeEvent(m); err != nil {
			b.Fatalf("unmarshal error: %s", err.Error())
		}
	}
}

func eventsCompare(e, e2 *Event, t *testing.T) {
	if e.EventBase.Name != e2.EventBase.Name {
		t.Fatalf("name not match: %+v vs %+v", e, e2)
	}
	if e.EventBase.Title != e2.EventBase.Title {
		t.Fatalf("title not match: %+v vs %+v", e, e2)
	}
	if e.EventBase.Time != e2.EventBase.Time {
		t.Fatalf("time not match: %+v vs %+v", e, e2)
	}
	if e.EventBase.Text != e2.EventBase.Text {
		t.Fatalf("text not match: %+v vs %+v", e, e2)
	}
	if e.EventBase.Tags != e2.EventBase.Tags {
		t.Fatalf("tags not match: %+v vs %+v", e, e2)
	}
}

func TestMarshalGOB(t *testing.T) {
	for i := 0; i < 1000; i++ {
		e := &Event{
			EventBase: &EventBase{
				Name:  randomStr(0),
				Title: randomStr(0),
				Time:  int64(i),
				Text:  randomStr(0),
				Tags:  randomStr(0),
			},
		}

		data, _, err := e.encodeGOB()
		if err != nil {
			t.Fatalf("encode error: %s (%+v)", err, e)
		}

		e2, err := decodeEventGOB(data)
		if err != nil {
			t.Fatalf("decode error: %s (%+v)", err, e)
		}
		eventsCompare(e, e2, t)
	}
}

func TestMarshal(t *testing.T) {
	for i := 0; i < 1000; i++ {
		e := &Event{
			EventBase: &EventBase{
				Name:  randomStr(0),
				Title: randomStr(0),
				Time:  int64(i),
				Text:  randomStr(0),
				Tags:  randomStr(0),
			},
		}

		data, _, err := e.encode()
		if err != nil {
			t.Fatalf("encode error: %s (%+v)", err, e)
		}

		e2, err := decodeEvent(data)
		if err != nil {
			t.Fatalf("decode error: %s (%+v)", err, e)
		}
		eventsCompare(e, e2, t)
	}
}

var runes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890<>?:\"{}|_+,./;'[]\\-=!@#$%^&*()`~`'")

func randomStr(n int) string {
	if n <= 0 {
		n = rand.Intn(90) + 10
	}
	b := make([]rune, n)
	for i := range b {
		b[i] = runes[rand.Intn(len(runes))]
	}
	return string(b)
}
