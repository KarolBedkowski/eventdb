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
	"time"
)

func parseTime(t string) (time.Time, error) {
	if t == "" {
		return time.Time{}, fmt.Errorf("empty time")
	}
	if ts, err := strconv.ParseFloat(t, 64); err == nil {
		if ts > 1000000000000000 { // nanos
			return time.Unix(0, int64(ts)), nil
		} else if ts > 1000000000000 { // micros
			return time.Unix(0, int64(ts*1000)), nil
		} else if ts > 1000000000 { // milils
			return time.Unix(0, int64(ts*1000000)), nil
		}
		return time.Unix(int64(ts), 0), nil
	}
	if ts, err := strconv.ParseInt(t, 10, 64); err == nil {
		if ts > 1000000000000000 { // nanos
			return time.Unix(0, ts), nil
		} else if ts > 1000000000000 { // micros
			return time.Unix(0, ts*1000), nil
		} else if ts > 1000000000 { // milils
			return time.Unix(0, ts*1000000), nil
		}
		return time.Unix(ts, 0), nil
	}
	return time.Parse(time.RFC3339Nano, t)
}
