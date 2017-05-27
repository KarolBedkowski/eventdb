//
// common.go
// Copyright (C) 2017 Karol Będkowski
//
// Distributed under terms of the GPLv3 license.
//

package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func intToTime(ts int64) time.Time {
	if ts > 1000000000000000000 { // nanos
		return time.Unix(0, ts)
	} else if ts > 1000000000000000 { // micros
		return time.Unix(0, ts*1000)
	} else if ts > 1000000000000 { // milils
		return time.Unix(0, ts*1000000)
	}
	return time.Unix(ts, 0)
}

func parseTime(t string) (time.Time, error) {
	if t == "" {
		return time.Time{}, fmt.Errorf("missing value")
	}
	if ts, err := strconv.ParseFloat(t, 64); err == nil {
		return intToTime(int64(ts)), nil
	}
	if ts, err := strconv.ParseInt(t, 10, 64); err == nil {
		return intToTime(ts), nil
	}
	if ts, err := time.Parse(time.RFC3339Nano, t); err == nil {
		return ts, nil
	}
	if ts, err := time.Parse(time.RFC3339, t); err == nil {
		return ts, nil
	}
	if ts, err := time.Parse("2006-01-02T15:04:05.000Z", t); err == nil {
		return ts, nil
	}
	if ts, err := time.Parse("2006-01-02T15:04:05", t); err == nil {
		return ts, nil
	}
	if ts, err := time.Parse("2006-01-02 15:04:05 -0700", t); err == nil {
		return ts, nil
	}
	if ts, err := time.Parse("2006-01-02 15:04:05", t); err == nil {
		return ts, nil
	}
	return time.Parse("2006-01-02", t)
}

func numToUnixNano(ts int64) int64 {
	if ts > 1000000000000000000 { // nanos
		return ts
	} else if ts > 1000000000000000 { // micros
		return ts * 1000
	} else if ts > 1000000000000 { // milils
		return ts * 1000000
	}
	return ts * 1000000000
}

func parseName(n string) (name string, tags []string) {
	if n == "" {
		return "", nil
	}
	fields := strings.Split(n, ":")
	name = fields[0]
	if len(fields) > 1 {
		tags = fields[1:]
	}
	if len(tags) == 0 {
		tags = nil
	}
	return
}
