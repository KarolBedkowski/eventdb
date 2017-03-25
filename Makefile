#
# Makefile
#

build:
	go build -v -o eventdb

run:
	#go run -v *.go -log.level debug
	go-reload *.go -log.level debug

# vim:ft=make
