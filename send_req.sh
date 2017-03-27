#!/bin/bash
set -o nounset
set -o errexit

function send_event() {
	local now
	now=$(date +%s)

	cat <<EOF | curl -s --data-binary @- "http://localhost:9701/api/v1/event"
{
	"name": "a1",
	"title": "title $(date)",
	"tags": "t1, t2",
	"text": "text for event ${now}",
	"time": ${now}000000000
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

		cat <<EOF | curl -s --data-binary @- "http://localhost:9701/api/v1/event"
{
	"name": "testname",
	"title": "title $(date --date @$d)",
	"tags": "t${t1} v${v1}",
	"text": "text for event ${d}",
	"time": ${d}000000000
}
EOF
	done
}

function generate_events2() {
	local start end d local t1 v1
	now=$(date +%s)

	for (( d=0; d<10; d=d+1 )); do
		cat <<EOF | curl -s --data-binary @- "http://localhost:9701/api/v1/event"
{
	"name": "testname",
	"title": "title $now",
	"tags": "t$d",
	"text": "text for event ${d}",
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
		cat <<EOF | curl -s --data-binary @- "http://pi:9701/api/v1/event"
{
	"name": "test",
	"title": "title $(date --date @$d)",
	"tags": "",
	"text": "text for event ${d}",
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
    "name": "${2:-testname}"
  }
}
EOF
}

function get_events() {
	curl -X GET -s "http://localhost:9701/api/v1/event?name=_any_"
}


function get_last() {
	curl -X GET -s "http://localhost:9701/last"
}

function get_events_last10min() {
	local ts
	ts=$(date --date '10 minutes ago' +%s)
	curl -X GET -s "http://localhost:9701/api/v1/event?from=${ts}&name=_any_"
}


if [[ $# == 0 ]]; then
	cat <<EOF
Usage:
	$0 send_event
	$0 generate_events
	$0 get_ann
	$0 get_events
	$0 get_events_last10min
	$0 get_last
EOF
	exit -1
fi

$*
