goq: a queuing and job management system. Fit for the cloud. Written in Go (golang).
===


Goq (Go Queue, or nothing to gawk at!) is a replacement for job management systems such as Sun GridEngine. (Yeah! No Reverse DNS hell on setup!)

And Goq's source is small, compact, and easily modifiable for your hacking pleasure. The main file is goq.go. The main dispatch logic is in the Start() routine, which is less than 200 lines long.

Features: 

 * simple : the system is easy to setup and use. On your master node, set the env variables (below) to configure. Then just do 'goq init' and 'goq serve' and you are up and running. 'goq sub mycommand myarg1 myarg2 ...' will submit a job.

 * secure  : Unlike most parallel job management systems, Goq actually uses strong AES encryption for all communications. This is equivalent to (or better than) the encryption that ssh gives you. You simply manually use ssh initially to distribute the .goq directory (which contains encryption keys) to all your worker nodes, and then there is no need for key exchange. This allows you to create images for cloud use that are ready-to-go on bootup. Only hosts on which you have copied the .goq directory to can submit or perform work for the cluster.

 * fast scheduling : unlike other queuing systems (I'm looking at you, gridengine), you don't have wait minutes for your jobs to start. Workers started with 'goq work forever' are waiting to receive work, and start processing instantly when work is submitted.

 * easy to coordinate distributed jobs : the 'goq wait' command allows you to create arbitrary graphs of worker flows. Many processes can line up to wait for a single remote process to finish, allowing easy barrier synchronization in a distributed/cloud setting.

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

Setup:

 * on your master-server (in your home dir): add 'export GOQ_HOME=/home/yourusername' (without quotes) to your .bashrc / whatever startup file for the shell use you.

   The GOQ_HOME env variable tells Goq where to find your encryption keys. They are stored in $GOQ_HOME/.goq

 * also in your master-server's .bashrc: set the values of GOQ_JSERV_IP, GOQ_JSERV_PORT. These are described in detail below.

 * goq init : run this once on your server (not on workers) to create your encryption keys.

 * on all workers: copy the .goq directory to the worker and copy your settings of GOQ_HOME, GOQ_JSERV_IP and GOQ_JSERV_PORT to your worker's .bashrc. (If your workers share a home directory with your server then obviously this copy step is omitted.)



Use:

There are three fundamental commands, corresponding to the three roles in the queuing system.

 * goq serve : starts a jobs server, by default on port 1776. Generally you only start one server; only one is needed for most purposes.

 * goq sub *command* {*arguments*}*: submits a job to the job server for queuing. You can 'goq sub' from anywhere, assuming that the environment variables (below) are configured.

 * goq work {forever} : request a job from the job server and executes it, returning the result to the server. Wash, rinse, repeat. A worker will loop forever if started with 'goq work forever'. Otherwise it will work until there are no more jobs, then stop after 1000 msec of inactivity.  Generally you'll want to start a forever worker on each cpu of each compute node in your cluster.

Additional useful commands

 * goq kill *jobid* : kills a previously submitted jobid

 * goq stat : shows a snapshot of the server's internal state

 * goq shutdown : shuts down the job server

 * goq wait *jobid* : waits until the specified job has finished.

configuration
-------------

Configuration is controlled by these environment variables:

 * GOQ_JSERV_IP = the ipv4 address of the server. Default: the first external facing interface discovered.

 * GOQ_JSERV_PORT = the port number the server is listening on (defaults to 1776)

 * GOQ_ODIR = the output directory where the server will write job output. Default: ./o

 * GOQ_CLUSTERID = secret shared amongst the cluster to reject jobs from the outside. Will be generated for you and written to the .goq/ directory on first run. Thereafter the system will read the key from disk if present.

 * GOQ_SENDTIMEOUT_MSEC = milliseconds of wait before timing-out various network communications (you shouldn't need to adjust this, unless traffic is super heavy and your workers aren't receiving jobs). The current default is 1000 msec.

 * GOQ_DEBUGMODE = should be false or unset unless you are developing on the system.

author: Jason E. Aten, Ph.D. <j.e.aten@gmail.com>.
