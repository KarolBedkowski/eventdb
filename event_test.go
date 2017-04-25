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
			Name:  "aaa",
			Title: "bbbbbbbbbbbbbbbb",
			Time:  int64(i),
			Text:  "ccccccccccccccccccccccccccccccccccccccccc",
			Tags:  []string{"10230", "9103", "90139", "01930"},
		})
	}
	return e
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
	if e.Name != e2.Name {
		t.Fatalf("name not match: %+v vs %+v", e, e2)
	}
	if e.Title != e2.Title {
		t.Fatalf("title not match: %+v vs %+v", e, e2)
	}
	if e.Time != e2.Time {
		t.Fatalf("time not match: %+v vs %+v", e, e2)
	}
	if e.Text != e2.Text {
		t.Fatalf("text not match: %+v vs %+v", e, e2)
	}
	for i, tag := range e.Tags {
		if tag != e2.Tags[i] {
			t.Fatalf("tags not match: %+v vs %+v", e, e2)
		}
	}
}

func TestMarshal(t *testing.T) {
	for i := 0; i < 1000; i++ {
		e := &Event{
			Name:  randomStr(0),
			Title: randomStr(0),
			Time:  int64(i),
			Text:  randomStr(0),
		}
		e.SetTags(randomStr(50))

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

var runes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890<>?:\"{}|_+,./;'[]\\-=!@#$%^&*()`~`' ")

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
