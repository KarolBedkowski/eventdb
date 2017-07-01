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

	if qqc[0].kind != conditionAny {
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

	if qqc[0].kind != conditionTag {
		t.Fatalf("wrong qq.conds 1: %v", qqc)
	} else if qqc[0].val != "t1" {
		t.Errorf("wrong qq.conds 1 tag: %v", qqc[0])
	}

	if qqc[1].kind != conditionTag {
		t.Fatalf("wrong qq.conds 2: %v", qqc)
	} else if qqc[1].val != "t2" {
		t.Errorf("wrong qq.conds 2 tag: %v", qqc[1])
	}

	if qqc[2].kind != conditionCol {
		t.Fatalf("wrong qq.conds 3: %v", qqc)
	} else if qqc[2].arg != "col2" || qqc[2].val != "23" {
		t.Errorf("wrong qq.conds 3 vals: %v", qqc[2])
	}

	if qqc[3].kind != conditionCol {
		t.Fatalf("wrong qq.conds 4: %v", qqc)
	} else if qqc[3].arg != "col3" || qqc[3].val != "45" {
		t.Errorf("wrong qq.conds 5 vals: %v", qqc[3])
	}
}

func TestQueryParseMultiArgs(t *testing.T) {
	query := "testb:_tag=t1,_tag=t2,col2=23,col3=45;testb:_tag=t4,col1=11"
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

	if len(qq.conds) != 2 {
		t.Fatalf("wrong qq.conds: %v", qq.conds)
	}

	qqc := qq.conds[0]

	if len(qqc) != 4 {
		t.Fatalf("wrong qqc: %v", qq.conds)
	}

	if qqc[0].kind != conditionTag {
		t.Fatalf("wrong qq.conds 1: %v", qqc)
	} else if qqc[0].val != "t1" {
		t.Errorf("wrong qq.conds 1 tag: %v", qqc[0])
	}

	if qqc[1].kind != conditionTag {
		t.Fatalf("wrong qq.conds 2: %v", qqc)
	} else if qqc[1].val != "t2" {
		t.Errorf("wrong qq.conds 2 tag: %v", qqc[1])
	}

	if qqc[2].kind != conditionCol {
		t.Fatalf("wrong qq.conds 3: %v", qqc)
	} else if qqc[2].arg != "col2" || qqc[2].val != "23" {
		t.Errorf("wrong qq.conds 3 vals: %v", qqc[2])
	}

	if qqc[3].kind != conditionCol {
		t.Fatalf("wrong qq.conds 4: %v", qqc)
	} else if qqc[3].arg != "col3" || qqc[3].val != "45" {
		t.Errorf("wrong qq.conds 5 vals: %v", qqc[3])
	}

	qqc = qq.conds[1]

	if len(qqc) != 2 {
		t.Fatalf("wrong qqc: %v", qq.conds)
	}

	if qqc[0].kind != conditionTag {
		t.Fatalf("wrong qq.conds 1: %v", qqc)
	} else if qqc[0].val != "t4" {
		t.Errorf("wrong qq.conds 1 tag: %v", qqc[0])
	}

	if qqc[1].kind != conditionCol {
		t.Fatalf("wrong qq.conds 3: %v", qqc)
	} else if qqc[1].arg != "col1" || qqc[1].val != "11" {
		t.Errorf("wrong qq.conds 3 vals: %v", qqc[1])
	}
}

func TestQueryParseUnions(t *testing.T) {
	query := "testb:_tag=t1,col2=23;testc:col1=12"
	q, err := ParseQuery(query)
	if err != nil {
		t.Fatalf("Parse query error not expected: %s", err)
	}

	t.Logf("query: %+v", q)

	if q.RawQuery != query {
		t.Fatalf("wrong RawQuery %v != %v", "testb", q.RawQuery)
	}

	if len(q.queries) != 2 {
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

	if len(qqc) != 2 {
		t.Fatalf("wrong qqc: %v", qq.conds)
	}

	if qqc[0].kind != conditionTag {
		t.Fatalf("wrong qq.conds 1: %v", qqc)
	} else if qqc[0].val != "t1" {
		t.Errorf("wrong qq.conds 1 tag: %v", qqc[0])
	}

	if qqc[1].kind != conditionCol {
		t.Fatalf("wrong qq.conds 1: %v", qqc)
	} else if qqc[1].arg != "col2" || qqc[1].val != "23" {
		t.Errorf("wrong qq.conds 3 vals: %v", qqc[1])
	}

	qq = q.queries[1]
	if qq.bucket != "testc" {
		t.Fatalf("wrong qq.bucket: %v", qq.bucket)
	}

	if len(qq.conds) != 1 {
		t.Fatalf("wrong qq.conds: %v", qq.conds)
	}

	qqc = qq.conds[0]

	if len(qqc) != 1 {
		t.Fatalf("wrong qqc: %v", qq.conds)
	}

	if qqc[0].kind != conditionCol {
		t.Fatalf("wrong qq.conds 0: %v", qqc)
	} else if qqc[0].arg != "col1" || qqc[0].val != "12" {
		t.Errorf("wrong qq.conds 0 vals: %v", qqc[0])
	}
}
