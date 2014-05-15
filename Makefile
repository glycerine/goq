
.PHONY: all test clean goq-build ship chk testbuild

all: goq-build testbuild

goq-build:
	gcc -o bin/faulter bin/faulter.c
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
	go build -gcflags "-N -l"
	go install

clean:
	rm -f *~ goq goq.test

chk:
	netstat -an|grep 1776 | /usr/bin/tee
	netstat -nuptl|grep 1776 | /usr/bin/tee
