
.PHONY: all test clean goq-build ship chk testbuild installation

all: goq-build testbuild

#ref: https://www.cockroachlabs.com/docs/stable/secure-a-cluster.html
cert:
	rm -rf xrpc/certs xrpc/my-safe-directory
	mkdir -p xrpc/certs xrpc/my-safe-directory
	cockroach cert create-ca --certs-dir=xrpc/certs --ca-key=xrpc/my-safe-directory/ca.key
	cockroach cert create-client root --certs-dir=xrpc/certs --ca-key=xrpc/my-safe-directory/ca.key
	cockroach cert create-node localhost 127.0.0.1 $(hostname) --certs-dir=xrpc/certs --ca-key=xrpc/my-safe-directory/ca.key



goq-build:
	# goq version gets its data here:
	/bin/echo "package main" > gitcommit.go
	/bin/echo "func init() { LASTGITCOMMITHASH = \"$(shell git rev-parse HEAD)\" }" >> gitcommit.go
	gcc -o bin/faulter bin/faulter.c
	cd schema; make
	GO15VENDOREXPERIMENT=1 go build
	GO15VENDOREXPERIMENT=1 go install

ship:
	# this works when we want to ship
	GO15VENDOREXPERIMENT=1 go build  --ldflags '-extldflags "-static -lanl -lpthread"'
	GO15VENDOREXPERIMENT=1 go install

testbuild:
	GO15VENDOREXPERIMENT=1 go test -c -gcflags "-N -l" -v


debug:
	# goq version gets its data here:
	/bin/echo "package main" > gitcommit.go
	/bin/echo "func init() { LASTGITCOMMITHASH = \"$(shell git rev-parse HEAD)\" }" >> gitcommit.go
	GO15VENDOREXPERIMENT=1 go build -gcflags "-N -l"
	GO15VENDOREXPERIMENT=1 go install -gcflags "-N -l"

clean:
	rm -rf *~ goq goq.test tempgoqtestdir*

cleanup:
	rm -rf ~/.goq/ ./o/
	goq init

chk:
	netstat -an|grep 1776 | /usr/bin/tee
	netstat -nuptl|grep 1776 | /usr/bin/tee
	ps auxwwf| grep -v grep| grep goq


# installation is the one-time setup of the project, where we install a
#  library we use, nanomsg. You don't need to 'make installation' if
#  you already installed these manually.
installation:
	# install our depenencies, nanomsg and capnproto
	cd vendor/nanomsg; autoreconf -i && ./configure && sudo make install
	# if you must hack on the schema, then you'll need to install the capnproto compiler as well. This needs a c++11 compiler.
	# Note: on my mac, I had to unset the dynamic lib paths to get a clean build
	# cd vendor/capnproto/c++; unset LD_LIBRARY_PATH; unset DYLD_LIBRARY_PATH; ./setup-autotools.sh && autoreconf -i && ./configure --prefix=$$(pwd)/../../install && make install
	cd schema; go build; go install

