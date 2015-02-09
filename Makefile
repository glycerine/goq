
.PHONY: all test clean goq-build ship chk testbuild installation

all: goq-build testbuild

goq-build:
	# goq version gets its data here:
	/bin/echo "package main" > gitcommit.go
	/bin/echo "func init() { LASTGITCOMMITHASH = \"$(shell git rev-parse HEAD)\" }" >> gitcommit.go
	gcc -o bin/faulter bin/faulter.c
	cd schema; make
	go build
	go install

ship:
	# this works when we want to ship
	go build  --ldflags '-extldflags "-static -lanl -lpthread"'
	go install

testbuild:
	go test -c -gcflags "-N -l" -v

test: goq
	./goq --server &
	./goq

debug:
	# goq version gets its data here:
	/bin/echo "package main" > gitcommit.go
	/bin/echo "func init() { LASTGITCOMMITHASH = \"$(shell git rev-parse HEAD)\" }" >> gitcommit.go
	go build -gcflags "-N -l"
	go install -gcflags "-N -l"

clean:
	rm -f *~ goq goq.test

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

