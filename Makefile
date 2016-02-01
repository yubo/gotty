OUTPUT_DIR = ./builds

gotty: tty/resource.go main.go tty/*.go rec/*.go
	go build

resource:  tty/resource.go

tty/resource.go: bindata/static/js/hterm.js bindata/static/js/gotty.js  bindata/static/index.html bindata/static/favicon.png
	go-bindata-assetfs -prefix bindata -pkg tty -ignore=\\.gitkeep -o tty/resource.go bindata/...
	gofmt -w tty/resource.go

bindata:
	mkdir bindata

bindata/static: bindata
	mkdir bindata/static

bindata/static/index.html: bindata/static resources/index.html
	cp resources/index.html bindata/static/index.html

bindata/static/favicon.png: bindata/static resources/favicon.png
	cp resources/favicon.png bindata/static/favicon.png

bindata/static/js: bindata/static
	mkdir -p bindata/static/js

bindata/static/js/hterm.js: bindata/static/js libapps/hterm/js/*.js
	cd libapps && \
	LIBDOT_SEARCH_PATH=`pwd` ./libdot/bin/concat.sh -i ./hterm/concat/hterm_all.concat -o ../bindata/static/js/hterm.js

bindata/static/js/gotty.js: bindata/static/js resources/gotty.js
	cp resources/gotty.js bindata/static/js/gotty.js

tools:
	go get github.com/tools/godep
	go get github.com/mitchellh/gox
	go get github.com/tcnksm/ghr
	go get github.com/elazarl/go-bindata-assetfs/...

deps:
	godep restore

test:
	if [ `go fmt ./... | wc -l` -gt 0 ]; then echo "go fmt error"; exit 1; fi

install:
	install -m 0755 -d /var/log/gotty
	install -m 0755 -d /etc/gotty
	install -m 0755 gotty /usr/sbin/
	install -m 0755 etc/init.d/gotty /etc/init.d/
	install -m 0644 etc/gotty/gotty.conf /etc/gotty
	/usr/sbin/update-rc.d gotty defaults
