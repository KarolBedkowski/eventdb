//
// event_test.go
// Copyright (C) 2017 Karol Będkowski <Karol Będkowski@kntbk>
//
// Distributed under terms of the MIT license.
//

package main

import (
	"bytes"
	"log"
	"math/rand"
	"testing"
	"time"
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

func marshalTS() [][]byte {
	m := make([][]byte, 1000, 1000)
	for i := int64(0); i < 1000; i++ {
		m[i], _ = encodeEventTS((i*100)<<10, nil)
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
		e := &Event{}
		if err := e.decode(m); err != nil {
			b.Fatalf("unmarshal error: %s", err.Error())
		}
	}
}

func BenchmarkEncodeEventTS(b *testing.B) {
	b.StopTimer()
	data := generateEvents()
	b.ReportAllocs()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		e := data[i%1000]
		if _, err := encodeEventTS(e.Time, nil); err != nil {
			b.Fatalf("marshal error: %s", err.Error())
		}
	}
}

func BenchmarkEncodeEventTS2(b *testing.B) {
	b.StopTimer()
	data := generateEvents()
	b.ReportAllocs()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		e := data[i%1000]
		if _, err := encodeEventTS2(e.Time, nil); err != nil {
			b.Fatalf("marshal error: %s", err.Error())
		}
	}
}

func BenchmarkDecodeEventTS(b *testing.B) {
	b.StopTimer()
	data := marshalTS()
	b.ReportAllocs()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		e := data[i%1000]
		if _, err := decodeEventTS(e); err != nil {
			b.Fatalf("decodeEventTS error: %s", err.Error())
		}
	}
}

func BenchmarkDecodeEventTS2(b *testing.B) {
	b.StopTimer()
	data := marshalTS()
	b.ReportAllocs()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		e := data[i%1000]
		if _, err := decodeEventTS2(e); err != nil {
			b.Fatalf("decodeEventTS2 error: %s", err.Error())
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

		e2 := &Event{}
		err = e2.decode(data)
		if err != nil {
			t.Fatalf("decode error: %s (%+v)", err, e)
		}
		eventsCompare(e, e2, t)
	}
}

func TestSetTags(t *testing.T) {
	e := &Event{}
	e.SetTags("tag1")
	if len(e.Tags) != 1 || e.Tags[0] != "tag1" {
		t.Fatalf("invalid tags: %v", e.Tags)
	}

	e.SetTags("tag1 tag2")
	if len(e.Tags) != 2 || e.Tags[0] != "tag1" || e.Tags[1] != "tag2" {
		t.Fatalf("invalid tags: %v", e.Tags)
	}

	e.SetTags("tag1 tag2,  tag3,tag4 tag5")
	if len(e.Tags) != 5 || e.Tags[0] != "tag1" || e.Tags[1] != "tag2" ||
		e.Tags[2] != "tag3" || e.Tags[3] != "tag4" || e.Tags[4] != "tag5" {
		t.Fatalf("invalid tags: %v", e.Tags)
	}
}

func TestCheckTags(t *testing.T) {
	e := &Event{}
	e.SetTags("tag1")
	if !e.CheckTags(nil) {
		t.Fatalf("invalid tags: %v", e.Tags)
	}
	if !e.CheckTags([]string{"tag1"}) {
		t.Fatalf("invalid tags: %v", e.Tags)
	}
	if e.CheckTags([]string{"tag2"}) {
		t.Fatalf("invalid tags: %v", e.Tags)
	}

	e.SetTags("tag1 tag2")
	if !e.CheckTags([]string{"tag1", "tag2"}) {
		t.Fatalf("invalid tags: %v", e.Tags)
	}
	if !e.CheckTags([]string{"tag2"}) {
		t.Fatalf("invalid tags: %v", e.Tags)
	}
	if e.CheckTags([]string{"tag3"}) {
		t.Fatalf("invalid tags: %v", e.Tags)
	}
}

func TestEncodeEventTS1(t *testing.T) {
	ts1, err := encodeEventTS(10, nil)
	if err != nil {
		t.Fatalf("encodeEventTS error: %s", err)
	}
	ts2, err := encodeEventTS(11, nil)
	if err != nil {
		t.Fatalf("encodeEventTS error: %s", err)
	}
	ts3, err := encodeEventTS(10<<3, nil)
	if err != nil {
		t.Fatalf("encodeEventTS error: %s", err)
	}
	ts4, err := encodeEventTS(10<<3+1, nil)
	if err != nil {
		t.Fatalf("encodeEventTS error: %s", err)
	}
	ts5, err := encodeEventTS(time.Now().Unix()-10, nil)
	if err != nil {
		t.Fatalf("encodeEventTS error: %s", err)
	}
	ts6, err := encodeEventTS(time.Now().Unix()+10, nil)
	if err != nil {
		t.Fatalf("encodeEventTS error: %s", err)
	}

	if r := bytes.Compare(ts1, ts2); r >= 0 {
		t.Fatalf("encodeEventTS failed compare ts1, ts2: %d", r)
	}
	if r := bytes.Compare(ts2, ts3); r >= 0 {
		t.Fatalf("encodeEventTS failed compare ts2, ts3: %d", r)
	}
	if r := bytes.Compare(ts3, ts4); r >= 0 {
		t.Fatalf("encodeEventTS failed compare ts3, ts4: %d", r)
	}
	if r := bytes.Compare(ts4, ts5); r >= 0 {
		t.Fatalf("encodeEventTS failed compare ts4, ts5: %d", r)
	}
	if r := bytes.Compare(ts5, ts6); r >= 0 {
		t.Fatalf("encodeEventTS failed compare ts5, ts6: %d", r)
	}
	if r := bytes.Compare(ts1, ts6); r >= 0 {
		t.Fatalf("encodeEventTS failed compare ts1, ts6: %d", r)
	}
}

func TestEncodeEventTS2(t *testing.T) {
	ts1, err := encodeEventTS(10, []byte(randomStr(100)))
	if err != nil {
		t.Fatalf("encodeEventTS error: %s", err)
	}
	ts2, err := encodeEventTS(11, []byte(randomStr(100)))
	if err != nil {
		t.Fatalf("encodeEventTS error: %s", err)
	}
	ts3, err := encodeEventTS(10<<3, []byte(randomStr(100)))
	if err != nil {
		t.Fatalf("encodeEventTS error: %s", err)
	}

	if r := bytes.Compare(ts1, ts2); r >= 0 {
		t.Fatalf("encodeEventTS failed compare ts1, ts2: %d", r)
	}
	if r := bytes.Compare(ts2, ts3); r >= 0 {
		t.Fatalf("encodeEventTS failed compare ts2, ts3: %d", r)
	}
	if r := bytes.Compare(ts1, ts3); r >= 0 {
		t.Fatalf("encodeEventTS failed compare ts3, ts4: %d", r)
	}
}

func TestEncodeEventCompare(t *testing.T) {
	for i := uint(0); i < 56; i++ {
		for j := uint(1); j < 8; j++ {
			tsin := int64(j << i)
			ts1, _ := encodeEventTS(tsin, nil)
			ts2, _ := encodeEventTS(tsin, nil)
			if bytes.Compare(ts1, ts2) != 0 {
				t.Fatalf("encodeEventTS wrong values: %v != %v", ts1, ts2)
			}
		}
	}
}

func TestEncodeDecodeTS2(t *testing.T) {
	for i := uint(0); i < 56; i++ {
		for j := uint(1); j < 8; j++ {
			tsin := int64(j << i)
			ts, _ := encodeEventTS(tsin, nil)
			tdec, _ := decodeEventTS2(ts)
			if tsin != tdec {
				t.Fatalf("decode error: for %d << %d: %v != %v (%v)", i, j, tdec, tsin, ts)
			}
			tdec2, _ := decodeEventTS(ts)
			if tsin != tdec2 {
				t.Fatalf("decode 2 error: for %d << %d: %v != %v (%v)", i, j, tdec2, tsin, ts)
			}
		}
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
