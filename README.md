goq: a queuing and job management system, written in Go (golang)
===


Goq (Go Queue, or something to Gawk at!) is a replacement for job management systems such as Sun GridEngine. (Yeah! No Reverse DNS hell on setup!)

And Goq is small, compact, and easily modifiable for your hacking pleasure. Go makes the source code very short and readable. The central dispatch function is JobServ::Start() circa line 248 of goq.go, and is just a little over a page long. The central file goq.go holds all of the logic for each of the three main commands: serve, sub, and worker. It is just over 1100 lines of code, including full comments and whitespace.

installation
------------

to build and install:

 * a) nanomsg installation: download (see https://github.com/nanomsg/nanomsg), compile, and install nanomsg [but see file VENDORED for notes; if you want the vendored versions instead, do step e) when you come to it.]
 * b) [for development only; not needed for just using the system] get a c++11 compiler on your system, and then do a capnproto installation: download (see http://kentonv.github.io/capnproto/ and https://github.com/kentonv/capnproto), compile, and install capnproto [likewise, check VENDORED first]
 * c) install testing library: go get -t github.com/smartystreets/goconvey (assuming go1.2; see https://github.com/smartystreets/goconvey for earlier)
 * d) go get -u -t github.com/glycerine/goq # -t to fetch the test dependencies (goconvey needs this) as well.

   [if you want/have to use the vendored projects instead of manual installs a) and b)
    then you would install them with e) and then add github.com/glycerine/goq/vendor/install/bin
    to your $PATH]

 * e) (optional) cd github.com/glycerine/goq; make installation 

 * f) cd github.com/glycerine/goq; make; go test -v

Goq was built using BDD, so the test suite has excellent coverage.


pre-requsites to install first
------------------------------

Goq uses a messaging system based 
on the nanocap transport, our term for a combination of the 
nanomsg[1] and Cap'n Proto[2] technologies. These are pre-requisite
packages, that must be installed prior to being able to build Goq.

[Update: these are now vendored into the repo, as a measure of future-proofing,
but you should know where to get their github sources for updates, if you wish. A quick description
of both of these dependencies, and pointers to their respective
locations follows. If you want to re-activate the dependent repos in order to
use git to update them, just rename dot.git.vendored to .git in each
of the vendor/$project directories.]

Nanomsg is the successor to ZeroMQ, and
provides multiple patterns for in-process, in-host,
and inter-host messaging. The peer-to-peer, publish-subscribe,
enterprise bus, pipeline, and surveyor protcols can be
leveraged for scalable messaging.

Note: If you aren't doing development (where you re-compile the schema/zjob.capnp file),
then you should not need to install the capnproto. You can just use the pre-compiled
schema.zjob.capnp.go file and the github.com/glycerine/go-capnproto module alone. In
this case, no c++11 compiler should be needed.

Cap'n Proto is the successor to ProtocolBuffers, and 
provides highly efficient encoding
and decoding of messages based on a strongly typed schema
language. It is blazing fast, and supplies very easy to 
text <-> binary conversion using the capnp tool. Capnp 
bindings are available for Golang, C++, and Python. 
We use the schema handling portion only,
as the RPC part of Cap'n Proto isn't released yet; we
use nanomsg instead. You'll need a C++11 compiler to build capnproto. g++-4.7.3 works fine.
Again, if you aren't doing development (where you re-compile the schema/zjob.capnp file),
then technically you should not need to install the full capnproto c++ installation.


[1] nanomsg: http://nanomsg.org/

[2] Cap'n Proto: http://kentonv.github.io/capnproto/

These two pre-requisite libraries should be downloaded and installed
prior to building Goq. The capnp tool should be placed in
your $PATH variable. Capnproto will require a C++11 compiler be
available, while nanomsg is pure C. Both utilize the CGO mechanism
to communicate with the Go code. See the makefile for the flags
to build one static binary for deployment (i.e. the make ship target). 
The schema/ subdirectory holds the capnproto schema file and its own
build dependencies.

usage
-----

[Status] proof of principle done and working, but not yet ready for real use.

There are three fundamental commands, corresponding to the three roles in the queuing system.

 * goq serve : starts a jobs server, by default on port 1776.

 * goq sub : submits a jobs to the job server.

 * goq work : request a job from the job server and executes it, returning the result to the server. Wash, rinse, repeat.




author: Jason E. Aten, Ph.D. <j.e.aten@gmail.com>.
