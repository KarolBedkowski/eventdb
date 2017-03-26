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
LDFLAGS_PI="-w -s \
	-X github.com/prometheus/common/version.Version='$(VERSION)' \
	-X github.com/prometheus/common/version.Revision='$(REVISION)' \
	-X github.com/prometheus/common/version.BuildDate='$(DATE)' \
	-X github.com/prometheus/common/version.BuildUser='$(USER)' \
	-X github.com/prometheus/common/version.Branch='$(BRANCH)'"

build:
	go build -v -o eventdb --ldflags $(LDFLAGS)

build_pi:
	GOGCCFLAGS="-fPIC -O4 -Ofast -pipe -march=native -mcpu=arm1176jzf-s -mfpu=vfp -mfloat-abi=hard -s" \
		GOARCH=arm GOARM=6 \
		go build -v -o eventdb-arm --ldflags $(LDFLAGS_PI)

run:
	#go run -v *.go -log.level debug
	go-reload *.go -log.level debug

# vim:ft=make
