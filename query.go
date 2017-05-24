//
// query.go
// Copyright (C) Karol Będkowski, 2017
//

package main

import (
	"fmt"
	"github.com/prometheus/common/log"
	"strings"
	"time"
)

type condition interface {
	match(e *Event) bool
	String() string
}

type conditionTag struct {
	tag string
}

func (c conditionTag) match(e *Event) bool {
	for _, t := range e.Tags {
		if t == c.tag {
			return true
		}
	}
	return false
}

func (c conditionTag) String() string {
	return fmt.Sprintf("{conditionTag: %s}", c.tag)
}

type conditionCol struct {
	col string
	val string
}

func (c conditionCol) match(e *Event) bool {
	for _, o := range e.Cols {
		if o.Name == c.col {
			return o.Value == c.val
		}
	}
	return false
}

func (c conditionCol) String() string {
	return fmt.Sprintf("{conditionCol: %s=%s}", c.col, c.val)
}

type conditionMatchAll struct {
}

func (c conditionMatchAll) match(e *Event) bool {
	return true
}

func (c conditionMatchAll) String() string {
	return "{conditionMatchAll}"
}

type subquery struct {
	bucket string
	conds  [][]condition
}

func (s *subquery) match(e *Event) bool {
o:
	for _, c := range s.conds {
		for _, cc := range c {
			if !cc.match(e) {
				continue o
			}
		}
		return true
	}
	return false
}

func (s *subquery) matchAny() bool {
	for _, c := range s.conds {
		if len(c) == 1 {
			if _, ok := c[0].(*conditionMatchAll); ok {
				return true
			}
		}
	}
	return false
}

func (s *subquery) simplify() {
	if s.matchAny() {
		if len(s.conds) > 1 {
			log.Debugf("subquery %v simplfied to conditionMatchAll", s)
			s.conds = [][]condition{[]condition{&conditionMatchAll{}}}
		}
	}
}

func (s *subquery) String() string {
	return fmt.Sprintf("subquery{bucket=%v, cons=%v}", s.bucket, s.conds)
}

// Query execute selects & updates on database
type Query struct {
	RawQuery string

	queries []*subquery
}

// ParseQuery parse query as string and return Query object or error
func ParseQuery(query string) (q *Query, err error) {
	log.Debugf("ParseQuery: '%v'", query)

	// map bucket -> subquery
	qpb := make(map[string]*subquery, 0)

	// temporary map  query -> bool for removing doubled
	loadedQuery := make(map[string]bool)

	// subqueries
	for _, sq := range strings.Split(query, ";") {
		sq = strings.TrimSpace(sq)
		if sq == "" {
			continue
		}

		if _, ok := loadedQuery[sq]; ok {
			continue
		}
		loadedQuery[sq] = true

		log.Debugf("parse: '%v'", sq)

		var conds []condition
		p := strings.SplitN(sq, ":", 2)
		if len(p) > 1 && p[1] != "" {
			for _, pr := range strings.Split(p[1], ",") {
				kv := strings.SplitN(pr, "=", 2)
				if len(kv) != 2 {
					continue
				}
				k := strings.TrimSpace(kv[0])
				v := strings.TrimSpace(kv[1])
				switch k {
				case "_tag":
					conds = append(conds, &conditionTag{tag: v})
				default:
					conds = append(conds, &conditionCol{col: k, val: v})
				}
			}
		}

		if len(conds) == 0 {
			conds = []condition{&conditionMatchAll{}}
		}

		bucket := strings.TrimSpace(p[0])
		log.Debugf("cond for %s: %+v", bucket, conds)
		if qpbi, ok := qpb[bucket]; ok {
			qpbi.conds = append(qpbi.conds, conds)
		} else {
			qpb[bucket] = &subquery{
				bucket: bucket,
				conds:  [][]condition{conds},
			}
		}
	}

	rq := &Query{
		RawQuery: query,
	}

	for _, k := range qpb {
		k.simplify()
		rq.queries = append(rq.queries, k)
	}

	return rq, nil
}

// Execute receive events from database according to query
func (q *Query) Execute(db *DB, from, to time.Time) (result []*Event, err error) {
	for _, s := range q.queries {
		matchF := s.match
		if s.matchAny() {
			matchF = nil
			log.Debugf("Query %v executeDelete - match all", q)
		}
		events, e := db.GetEvents(s.bucket, from, to, matchF)
		if e != nil {
			return nil, e
		}
		result = append(result, events...)
	}
	return result, nil
}

// ExecuteDelete delete events according to query
func (q *Query) ExecuteDelete(db *DB, from, to time.Time) (deleted int, err error) {
	for _, s := range q.queries {
		matchF := s.match
		if s.matchAny() {
			matchF = nil
			log.Debugf("Query %v executeDelete - match all", q)
		}
		d, e := db.DeleteEvents(s.bucket, from, to, matchF)
		if e != nil {
			return 0, e
		}
		deleted += d
	}
	return
}

func (q *Query) String() string {
	return fmt.Sprintf("Query{RawQuery='%v', queries=%s}", q.RawQuery, q.queries)
}
