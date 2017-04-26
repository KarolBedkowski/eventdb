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

func prepareTestEvents() [][]byte {
	m := make([][]byte, 1000, 1000)
	var err error
	for i, e := range generateEvents() {
		m[i], _, err = e.marshal()
		if err != nil {
			log.Fatalf("marshalGOBEvents error: %s", err)
		}
	}
	return m
}

func prepareTestTS() [][]byte {
	m := make([][]byte, 1000, 1000)
	for i := int64(0); i < 1000; i++ {
		m[i], _ = marshalTS((i*100)<<10, nil)
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
		if _, _, err := e.marshal(); err != nil {
			b.Fatalf("marshal error: %s", err.Error())
		}
	}
}

func BenchmarkUnmarshal(b *testing.B) {
	b.StopTimer()
	data := prepareTestEvents()
	b.ReportAllocs()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		m := data[i%1000]
		e := &Event{}
		if err := e.unmarshal(m); err != nil {
			b.Fatalf("unmarshal error: %s", err.Error())
		}
	}
}

func BenchmarkMarshallTS(b *testing.B) {
	b.StopTimer()
	data := generateEvents()
	b.ReportAllocs()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		e := data[i%1000]
		if _, err := marshalTS(e.Time, nil); err != nil {
			b.Fatalf("marshal error: %s", err.Error())
		}
	}
}

func BenchmarkMashalTSlegacy(b *testing.B) {
	b.StopTimer()
	data := generateEvents()
	b.ReportAllocs()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		e := data[i%1000]
		if _, err := marshalTSlegacy(e.Time, nil); err != nil {
			b.Fatalf("marshal error: %s", err.Error())
		}
	}
}

func BenchmarkUnmarshalTS(b *testing.B) {
	b.StopTimer()
	data := prepareTestTS()
	b.ReportAllocs()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		e := data[i%1000]
		if _, err := unmarshalTS(e); err != nil {
			b.Fatalf(" unmarshalTSerror: %s", err.Error())
		}
	}
}

func BenchmarkUnmarshalTSlegacy(b *testing.B) {
	b.StopTimer()
	data := prepareTestTS()
	b.ReportAllocs()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		e := data[i%1000]
		if _, err := unmarshalTSlegacy(e); err != nil {
			b.Fatalf("unmarshalTSlegacy error: %s", err.Error())
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

		data, _, err := e.marshal()
		if err != nil {
			t.Fatalf("marshal error: %s (%+v)", err, e)
		}

		e2 := &Event{}
		err = e2.unmarshal(data)
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

func TestMarshalTS(t *testing.T) {
	ts1, err := marshalTS(10, nil)
	if err != nil {
		t.Fatalf("marshalTS error: %s", err)
	}
	ts2, err := marshalTS(11, nil)
	if err != nil {
		t.Fatalf("marshalTS error: %s", err)
	}
	ts3, err := marshalTS(10<<3, nil)
	if err != nil {
		t.Fatalf("marshalTS error: %s", err)
	}
	ts4, err := marshalTS(10<<3+1, nil)
	if err != nil {
		t.Fatalf("marshalTS error: %s", err)
	}
	ts5, err := marshalTS(time.Now().Unix()-10, nil)
	if err != nil {
		t.Fatalf("marshalTS error: %s", err)
	}
	ts6, err := marshalTS(time.Now().Unix()+10, nil)
	if err != nil {
		t.Fatalf("marshalTS error: %s", err)
	}

	if r := bytes.Compare(ts1, ts2); r >= 0 {
		t.Fatalf("marshalTS failed compare ts1, ts2: %d", r)
	}
	if r := bytes.Compare(ts2, ts3); r >= 0 {
		t.Fatalf("marshalTS failed compare ts2, ts3: %d", r)
	}
	if r := bytes.Compare(ts3, ts4); r >= 0 {
		t.Fatalf("marshalTS failed compare ts3, ts4: %d", r)
	}
	if r := bytes.Compare(ts4, ts5); r >= 0 {
		t.Fatalf("marshalTS failed compare ts4, ts5: %d", r)
	}
	if r := bytes.Compare(ts5, ts6); r >= 0 {
		t.Fatalf("marshalTS failed compare ts5, ts6: %d", r)
	}
	if r := bytes.Compare(ts1, ts6); r >= 0 {
		t.Fatalf("marshalTS failed compare ts1, ts6: %d", r)
	}
}

func TestMarshalTSlegacy(t *testing.T) {
	ts1, err := marshalTSlegacy(10, []byte(randomStr(100)))
	if err != nil {
		t.Fatalf("marshalTS error: %s", err)
	}
	ts2, err := marshalTSlegacy(11, []byte(randomStr(100)))
	if err != nil {
		t.Fatalf("marshalTS error: %s", err)
	}
	ts3, err := marshalTSlegacy(10<<3, []byte(randomStr(100)))
	if err != nil {
		t.Fatalf("marshalTS error: %s", err)
	}

	if r := bytes.Compare(ts1, ts2); r >= 0 {
		t.Fatalf("marshalTS failed compare ts1, ts2: %d", r)
	}
	if r := bytes.Compare(ts2, ts3); r >= 0 {
		t.Fatalf("marshalTS failed compare ts2, ts3: %d", r)
	}
	if r := bytes.Compare(ts1, ts3); r >= 0 {
		t.Fatalf("marshalTS failed compare ts3, ts4: %d", r)
	}
}

func TestMarshalEventCompare(t *testing.T) {
	for i := uint(0); i < 56; i++ {
		for j := uint(1); j < 8; j++ {
			tsin := int64(j << i)
			ts1, _ := marshalTS(tsin, nil)
			ts2, _ := marshalTS(tsin, nil)
			if bytes.Compare(ts1, ts2) != 0 {
				t.Fatalf("marshalTS wrong values: %v != %v", ts1, ts2)
			}
		}
	}
}

func TestMarshalUnmarshalTS(t *testing.T) {
	for i := uint(0); i < 56; i++ {
		for j := uint(1); j < 8; j++ {
			tsin := int64(j << i)
			ts, _ := marshalTS(tsin, nil)
			tdec, _ := unmarshalTSlegacy(ts)
			if tsin != tdec {
				t.Fatalf("decode error: for %d << %d: %v != %v (%v)", i, j, tdec, tsin, ts)
			}
			tdec2, _ := unmarshalTS(ts)
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
