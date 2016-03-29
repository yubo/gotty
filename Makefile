OUTPUT_DIR = ${PWD}/builds

gotty: tty/resource.go main.go tty/*.go rec/*.go
	go build -o gotty cmd/gotty/*.go

resource:  tty/resource.go

tty/resource.go: bindata/static/js/hterm.js bindata/static/js/gotty.js  bindata/static/index.html bindata/static/favicon.ico bindata/static/css/style.css bindata/static/css/bootstrap-theme.min.css bindata/static/css/bootstrap.min.css bindata/static/js/bootstrap.min.js bindata/static/js/jquery-1.12.1.js bindata/static/js/demo.js bindata/static/js/fetch.js bindata/static/font/UbuntuMono-BP.ttf bindata/static/font/UbuntuMono-RP.ttf
	go-bindata -prefix bindata -pkg tty -ignore=\\.gitkeep -o tty/resource.go bindata/...
	gofmt -w tty/resource.go

bindata:
	mkdir -p bindata

bindata/static: bindata
	mkdir -p bindata/static

bindata/static/index.html: bindata/static resources/index.html
	cp resources/index.html bindata/static/index.html

bindata/static/favicon.ico: bindata/static resources/favicon.ico
	cp resources/favicon.ico bindata/static/favicon.ico

bindata/static/js: bindata/static
	mkdir -p bindata/static/js

bindata/static/css: bindata/static
	mkdir -p bindata/static/css

bindata/static/font: bindata/static
	mkdir -p bindata/static/font

bindata/static/js/hterm.js: bindata/static/js resources/js/hterm.js
	cp resources/js/hterm.js bindata/static/js/hterm.js

resources/js/hterm.js: libapps/hterm/js/*.js
	cd libapps && \
	LIBDOT_SEARCH_PATH=`pwd` ./libdot/bin/concat.sh -i ./hterm/concat/hterm_all.concat -o ../resources/js/hterm.js

bindata/static/js/gotty.js: bindata/static/js resources/js/gotty.js
	cp resources/js/gotty.js bindata/static/js/gotty.js

bindata/static/js/bootstrap.min.js: bindata/static/js resources/js/bootstrap.min.js
	cp resources/js/bootstrap.min.js bindata/static/js/bootstrap.min.js

bindata/static/js/jquery-1.12.1.js: bindata/static/js resources/js/jquery-1.12.1.js
	cp resources/js/jquery-1.12.1.js bindata/static/js/jquery-1.12.1.js

bindata/static/js/demo.js: bindata/static/js resources/js/demo.js
	cp resources/js/demo.js bindata/static/js/demo.js

bindata/static/js/fetch.js: bindata/static/js resources/js/fetch.js
	cp resources/js/fetch.js bindata/static/js/fetch.js

bindata/static/css/bootstrap-theme.min.css: bindata/static/css resources/css/bootstrap-theme.min.css
	cp resources/css/bootstrap-theme.min.css bindata/static/css/bootstrap-theme.min.css

bindata/static/css/bootstrap.min.css: bindata/static/css resources/css/bootstrap.min.css
	cp resources/css/bootstrap.min.css bindata/static/css/bootstrap.min.css

bindata/static/css/style.css: bindata/static/css resources/css/style.css
	cp resources/css/style.css bindata/static/css/style.css

bindata/static/font/UbuntuMono-BP.ttf: bindata/static/font resources/font/UbuntuMono-BP.ttf
	cp resources/font/UbuntuMono-BP.ttf bindata/static/font/UbuntuMono-BP.ttf

bindata/static/font/UbuntuMono-RP.ttf: bindata/static/font resources/font/UbuntuMono-RP.ttf
	cp resources/font/UbuntuMono-RP.ttf bindata/static/font/UbuntuMono-RP.ttf

tools:
	go get github.com/tools/godep
	go get github.com/mitchellh/gox
	go get github.com/tcnksm/ghr

deps:
	godep restore

test:
	if [ `go fmt ./... | wc -l` -gt 0 ]; then echo "go fmt error"; exit 1; fi

cross_compile:
	cd cmd/gotty && GOARM=6 gox -os="linux darwin freebsd netbsd openbsd" -arch="386 amd64 arm" -output "${OUTPUT_DIR}/pkg/{{.OS}}_{{.Arch}}/{{.Dir}}"


targz:
	mkdir -p ${OUTPUT_DIR}/dist
	cd ${OUTPUT_DIR}/pkg/; for osarch in *; do (cd $$osarch; tar zcvf ../../dist/gotty_$$osarch.tar.gz ./*); done;

shasums:
	cd ${OUTPUT_DIR}/dist; shasum * > ./SHASUMS

release:
	ghr --delete --prerelease -u yubo -r gotty pre-release ${OUTPUT_DIR}/dist

install:
	install -m 0755 -d /var/log/gotty
	install -m 0755 -d /etc/gotty
	install -m 0755 gotty /usr/sbin/
	install -m 0755 etc/init.d/gotty /etc/init.d/
	if [ ! -e /etc/gotty/gotty.conf ]; then install -m 0644 etc/gotty/gotty.conf /etc/gotty; fi;
	/usr/sbin/update-rc.d gotty defaults

run:
	./gotty -D -logtostderr -v 3 -c ./etc/gotty/gotty.run.conf daemon
