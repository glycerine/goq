
.PHONY: all test clean goq-build ship chk installation

goq-build:
	gcc -o bin/faulter bin/faulter.c
	cd schema; make
	go build
	go install

ship:
	# the --ldflags below work for when we want to ship, to get a standalone static binary.
	# NB that I'm getting this error: cgo_unix.go:53: warning: Using 'getaddrinfo' in statically linked applications requires at runtime the shared libraries from the glibc version used for linking, but the binary still works on all deployment targets I've tried.
	#
	go build  --ldflags '-extldflags "-static -lanl -lpthread"'
	go install

# installation is the one-time setup of the project, where we install the two
#  libraries we use, nanomsg and capnproto. You don't need to 'make installation' if
#  you already installed these manually.
# 
# todo with installation: figure out how to adjust flags to use the vendored versions when desired.
installation:
	# install our depenencies, nanomsg and capnproto
	cd vendor/nanomsg; autoreconf -i && ./configure --prefix=$$(pwd)/../install && make install
	# on my mac, I had to unset the dynamic lib paths to get a clean build
	# cd vendor/capnproto/c++; unset LD_LIBRARY_PATH; unset DYLD_LIBRARY_PATH; ./setup-autotools.sh && autoreconf -i && ./configure --prefix=$$(pwd)/../../install && make install
	cd schema; go build; go install

testbuild:
	go test -c -gcflags "-N -l" -v

test: goq
	./goq --server &
	./goq

debug:
	go build -gcflags "-N -l"
	go install

clean:
	rm -f *~ goq goq.test

chk:
	netstat -an|grep 1776 | /usr/bin/tee
	netstat -nuptl|grep 1776 | /usr/bin/tee
