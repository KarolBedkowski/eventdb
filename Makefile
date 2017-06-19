#
# Makefile
#
#
#
VERSION=dev
REVISION=`git describe --always`
DATE=`date +%Y%m%d%H%M%S`
USER=`whoami`
BRANCH=`git branch | grep '^\*' | cut -d ' ' -f 2`
LDFLAGS="\
	-X github.com/prometheus/common/version.Version='$(VERSION)' \
	-X github.com/prometheus/common/version.Revision='$(REVISION)' \
	-X github.com/prometheus/common/version.BuildDate='$(DATE)' \
	-X github.com/prometheus/common/version.BuildUser='$(USER)' \
	-X github.com/prometheus/common/version.Branch='$(BRANCH)'"
LDFLAGS_PI="\
	-w -s \
	-X github.com/prometheus/common/version.Version='$(VERSION)' \
	-X github.com/prometheus/common/version.Revision='$(REVISION)' \
	-X github.com/prometheus/common/version.BuildDate='$(DATE)' \
	-X github.com/prometheus/common/version.BuildUser='$(USER)' \
	-X github.com/prometheus/common/version.Branch='$(BRANCH)'"

build: *.schema.gen.go
	go build -v -o eventdb --ldflags $(LDFLAGS)

build_pi: *.schema.gen.go
	GOGCCFLAGS="-fPIC -O4 -Ofast -pipe -march=native -mcpu=arm1176jzf-s -mfpu=vfp -mfloat-abi=hard -marm -s" \
		GOARCH=arm GOARM=6 \
		go build -v -o eventdb-arm --ldflags $(LDFLAGS_PI)

install_pi: eventdb-arm
	ssh pi "systemctl --user stop eventdb"
	ssh pi "[ -f ~/eventdb/eventdb-arm ] && mv -f ~/eventdb/eventdb-arm ~/eventdb/eventdb-arm.old"
	scp eventdb-arm pi:eventdb/
	ssh pi "systemctl --user start eventdb"
	ssh pi "systemctl --user status eventdb"

run:
	#go run -v *.go -log.level debug
	go-reload `ls *.go | grep -v _test.go` -log.level debug

clean:
	rm -f eventdb eventdb-arm

%.schema.gen.go: %.schema
	gencode go -schema $< -package main

# vim:ft=make
