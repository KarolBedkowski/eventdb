#
# Makefile
#
#
#
LDFLAGS_PI="-w -s"

build:
	go build -v -o eventdb

build_pi:
	GOGCCFLAGS="-fPIC -O4 -Ofast -pipe -march=native -mcpu=arm1176jzf-s -mfpu=vfp -mfloat-abi=hard -s" \
		GOARCH=arm GOARM=6 \
		go build -v -o eventdb-arm --ldflags $(LDFLAGS_PI)

run:
	#go run -v *.go -log.level debug
	go-reload *.go -log.level debug

# vim:ft=make
