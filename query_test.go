//
// event_test.go
// Copyright (C) 2017 Karol Będkowski <Karol Będkowski@kntbk>
//
// Distributed under terms of the MIT license.
//

package main

import (
	"testing"
)

func TestQueryParseSimple(t *testing.T) {
	q, err := ParseQuery("test")
	if err != nil {
		t.Fatalf("Parse query error not expected: %s", err)
	}

	t.Logf("query: %+v", q)

	if q.RawQuery != "test" {
		t.Fatalf("wrong RawQuery %v != %v", "test", q.RawQuery)
	}

	if len(q.queries) != 1 {
		t.Fatalf("wrong q.queries: %+v", q.queries)
	}

	qq := q.queries[0]
	if qq.bucket != "test" {
		t.Fatalf("wrong qq.bucket: %v", qq.bucket)
	}

	if len(qq.conds) != 1 {
		t.Fatalf("wrong qq.conds: %v", qq.conds)
	}

	qqc := qq.conds[0]

	if len(qqc) != 1 {
		t.Fatalf("wrong qqc: %v", qq.conds)
	}

	if _, ok := (qqc[0]).(*conditionMatchAll); !ok {
		t.Fatalf("wrong qq.conds: %v", qqc)
	}
}

func TestQueryParseSimpleArgs(t *testing.T) {
	query := "testb:_tag=t1,_tag=t2,col2=23,col3=45"
	q, err := ParseQuery(query)
	if err != nil {
		t.Fatalf("Parse query error not expected: %s", err)
	}

	t.Logf("query: %+v", q)

	if q.RawQuery != query {
		t.Fatalf("wrong RawQuery %v != %v", "testb", q.RawQuery)
	}

	if len(q.queries) != 1 {
		t.Fatalf("wrong q.queries: %+v", q.queries)
	}

	qq := q.queries[0]
	if qq.bucket != "testb" {
		t.Fatalf("wrong qq.bucket: %v", qq.bucket)
	}

	if len(qq.conds) != 1 {
		t.Fatalf("wrong qq.conds: %v", qq.conds)
	}

	qqc := qq.conds[0]

	if len(qqc) != 4 {
		t.Fatalf("wrong qqc: %v", qq.conds)
	}

	if m, ok := (qqc[0]).(*conditionTag); !ok {
		t.Fatalf("wrong qq.conds 1: %v", qqc)
	} else if m.tag != "t1" {
		t.Errorf("wrong qq.conds 1 tag: %v", m)
	}

	if m, ok := (qqc[1]).(*conditionTag); !ok {
		t.Fatalf("wrong qq.conds 2: %v", qqc)
	} else if m.tag != "t2" {
		t.Errorf("wrong qq.conds 2 tag: %v", m)
	}

	if m, ok := (qqc[2]).(*conditionCol); !ok {
		t.Fatalf("wrong qq.conds 3: %v", qqc)
	} else if m.col != "col2" || m.val != "23" {
		t.Errorf("wrong qq.conds 3 vals: %v", m)
	}

	if m, ok := (qqc[3]).(*conditionCol); !ok {
		t.Fatalf("wrong qq.conds 4: %v", qqc)
	} else if m.col != "col3" || m.val != "45" {
		t.Errorf("wrong qq.conds 5 vals: %v", m)
	}
}
