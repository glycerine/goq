goq: a queuing and job management system, written in Go (golang)
===


Goq (Go Queue, or something to Gawk at!) is a replacement for job management systems such as Sun GridEngine. (Yeah! No Reverse DNS hell on setup!)

And Goq's source is small, compact, and easily modifiable for your hacking pleasure. The main file is goq.go. The main dispatch logic is in the Start() routine, which is less than 200 lines long.


installation
------------

to build and install:


 * a) go get -u -t github.com/glycerine/goq 

 * b) cd github.com/glycerine/goq; make installation 

 * c) adjust your LD_LIBRARY_PATH to include $GOPATH/src/github.com/glycerine/goq/vendor/install/lib

   Details: include the nanomsg library directory (e.g. ${GOPATH}/src/github.com/glycerine/goq/vendor/install/lib) in your LD_LIBRARY_PATH, and include $GOPATH/bin in your $PATH. The test suite needs to be able to find goq in your $PATH.

   For example, if you installed nanomsg using 'make installation', then you would add lines like these to your ~/.bashrc (assumes GOPATH already set): 

        export LD_LIBRARY_PATH=${GOPATH}/src/github.com/glycerine/goq/vendor/install/lib:${LD_LIBRARY_PATH}

        export PATH=$GOPATH/bin:$PATH  # probably already done.

   Then save the .bashrc changes, and source them with 

    $ . ~/.bashrc # have changes take effect in the current shell

   The test suite ('go test -v' runs the test suite) depends on being able to shell out to 'goq', so it must be on your $PATH.

 * d) cd $GOPATH/src/github.com/glycerine/goq; make; go test -v

Goq was built using BDD, so the test suite has good coverage.


pre-requsites to install first
------------------------------

Goq uses a messaging system based 
on the nanocap transport, our term for a combination of the 
nanomsg[1] and Cap'n Proto[2] technologies. Nanomsg is a pre-requisite
that must be installed prior to being able to build Goq.

'make installation' should build and do a local install of nanomsg into
the vendor/install directory. Adjust your LD_LIBRARY_PATH accordingly.

[Note: If you aren't doing development (where you re-compile the schema/zjob.capnp file),
then you should not need to install capnproto. You can just use the pre-compiled
schema.zjob.capnp.go file and the github.com/glycerine/go-capnproto module alone. In
this case, no c++11 compiler should be needed.]

[1] nanomsg: http://nanomsg.org/

[2] Cap'n Proto: http://kentonv.github.io/capnproto/


usage
-----

[Status] working and useful.

There are three fundamental commands, corresponding to the three roles in the queuing system.

 * goq serve : starts a jobs server, by default on port 1776.

 * goq sub *command* {arguments}*: submits a job to the job server for queuing.

 * goq work {forever} : request a job from the job server and executes it, returning the result to the server. Wash, rinse, repeat. A worker will loop forever if started with 'goq work forever'. Otherwise it will work until there are no more jobs, then stop after 1000 msec of inactivity.

Additional useful commands

 * goq kill *jobid* : kills a previously submitted jobid

 * goq stat : shows a snapshot of the server's internal state

 * goq shutdown : shuts down the job server

configuration
-------------

Configuration is controlled by these environment variables:

 * GOQ_JSERV_IP = the ipv4 address of the server. Default: the first external facing interface discovered.

 * GOQ_JSERV_PORT = the port number the server is listening on (defaults to 1776)

 * GOQ_ODIR = the output directory where the server will write job output. Default: ./o

 * GOQ_CLUSTERID = secret shared amongst the cluster to reject jobs from the outside. Generate with 'goq clusterid', or one will be generated for you and written to the .goqclusterid file on first run. Thereafter the system will read the key from disk if present. The .goqclusterid overrides the environment.

 * GOQ_SENDTIMEOUT_MSEC = milliseconds of wait before timing-out various network communications (you shouldn't need to adjust this, unless traffic is super heavy and your workers aren't receiving jobs). The current default is 1000 msec.


author: Jason E. Aten, Ph.D. <j.e.aten@gmail.com>.
