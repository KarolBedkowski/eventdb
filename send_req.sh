#!/bin/bash
set -o nounset
set -o errexit

function send_event() {
	local now
	now=$(date +%s)

	cat <<EOF | curl -s --data-binary @- "http://localhost:9701/api/v2/event"
{
	"name": "a1",
	"summary": "summary $(date)",
	"tags": "t1, t2",
	"description": "description for event ${now}",
	"time": ${now}000000000,
	"labels": {"c1": "val1", "c2": "val2"}
}
EOF
}


function generate_events() {
	local start end d local t1 v1
	end=$(date +%s)
	start=$((end - 60000 ))

	for (( d=start; d<end; d=d+600 )); do
		t1=$(bc <<< "($d / 100) % 10")
		v1=$(bc <<< "($d / 100) % 15")

		cat <<EOF | curl -s --data-binary @- "http://localhost:9701/api/v2/event"
{
	"name": "testname",
	"summary": "summary $(date --date @$d)",
	"tags": "t${t1} v${v1}",
	"description": "description for event ${d}",
	"time": ${d}000000000,
	"labels": {"c1": "val_${t1}", "c2": "val_${v1}"}
}
EOF
	done
}

function generate_events2() {
	local start end d local t1 v1
	now=$(date +%s)

	for (( d=0; d<10; d=d+1 )); do
		cat <<EOF | curl -s --data-binary @- "http://localhost:9701/api/v2/event"
{
	"name": "testname",
	"summary": "summary $now",
	"tags": "t$d",
	"description": "description for event ${d}",
	"time": ${now}000000000
}
EOF
	done
}



function generate_events_pi() {
	local start end d
	end=$(date +%s)
	start=$((end - 60000 ))

	for (( d=start; d<end; d=d+600 )); do
		cat <<EOF | curl -s --data-binary @- "http://pi:9701/api/v2/event"
{
	"name": "test",
	"summary": "summary $(date --date @$d)",
	"tags": "",
	"description": "description for event ${d}",
	"time": ${d}000000000
}
EOF
	done
}


function get_ann() {
	cat <<EOF | curl -X POST -s --data-binary @- "http://${1:-localhost}:9701/annotations"
{
  "range": { "from": "2016-03-04T04:07:55.144Z", "to": "2018-03-04T07:07:55.144Z" },
  "rangeRaw": { "from": "", "to": "" },
  "annotation": {
    "datasource": "generic datasource",
    "enable": true,
    "name": "test",
    "query": "${2:-testname}"
  }
}
EOF
}

function get_events() {
	curl -X GET -s "http://localhost:9701/api/v2/event?query=testname"
}


function get_last() {
	curl -X GET -s "http://localhost:9701/last"
}

function get_events_last10min() {
	local ts
	ts=$(date --date '10 minutes ago' +%s)
	curl -X GET -s "http://localhost:9701/api/v2/event?from=${ts}&query=testname"
}

function get_events_last1h() {
	local ts query
	query=${1:-testname}
	ts=$(date --date '1 hour ago' +%s)
	curl -X GET -s -g -G \
		--data-urlencode "query=${query}" \
		"http://localhost:9701/api/v2/event?from=${ts}"
}


if [[ $# == 0 ]]; then
	cat <<EOF
Usage:
	$0 send_event
	$0 generate_events
	$0 get_ann
	$0 get_events
	$0 get_events_last10min
	$0 get_events_last1h
	$0 get_last
EOF
	exit -1
fi

$*
