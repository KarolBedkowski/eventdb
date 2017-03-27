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

func parseTime(t string) (time.Time, error) {
	if t == "" {
		return time.Time{}, fmt.Errorf("empty time")
	}
	if ts, err := strconv.ParseFloat(t, 64); err == nil {
		if ts > 1000000000000000000 { // nanos
			fmt.Printf("try float nanos %v", ts)
			return time.Unix(0, int64(ts)), nil
		} else if ts > 1000000000000000 { // micros
			fmt.Printf("try float micros %v", ts)
			return time.Unix(0, int64(ts*1000)), nil
		} else if ts > 1000000000000 { // milils
			fmt.Printf("try float millis %v", ts)
			return time.Unix(0, int64(ts*1000000)), nil
		}
		return time.Unix(int64(ts), 0), nil
	}
	if ts, err := strconv.ParseInt(t, 10, 64); err == nil {
		if ts > 1000000000000000000 { // nanos
			return time.Unix(0, ts), nil
		} else if ts > 1000000000000000 { // micros
			return time.Unix(0, ts*1000), nil
		} else if ts > 1000000000000 { // milils
			return time.Unix(0, ts*1000000), nil
		}
		return time.Unix(ts, 0), nil
	}
	if ts, err := time.Parse(time.RFC3339Nano, t); err == nil {
		return ts, nil
	}
	if ts, err := time.Parse(time.RFC3339, t); err == nil {
		return ts, nil
	}
	return time.Parse("2006-01-02T15:04:05", t)
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
