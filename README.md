goq: a queuing and job management system fit for the cloud. Written in Go (golang).
-----------------------------------------------------------------------------------


Goq (Go-queue, or nothing to gawk at!) is a replacement for job management systems such as Sun GridEngine. (Yeah! No Reverse DNS hell on setup!)

Goq's source is small, compact, and easily modified to do your bidding. The main file is goq.go. The main dispatch logic is in the Start() routine, which is less than 200 lines long. All together it is less than 5k lines of code. This compactness is a tribute to Go.

Goq Features: 

 * simple : the system is easy to setup and use. The three roles are server, submitter, and worker. Each is trivial to deploy.

   a) server: On your master node, set the env variable GOQ_HOME to your home directory (where two subdirectories will be created: o and .goq). Then just do 'goq init' and 'goq serve &'.

   b) job submission: 'goq sub mycommand myarg1 myarg2 ...' will submit a job. 

   c) workers: Start workers on compute nodes by copying the .goq directory to them, setting GOQ_HOME in the env/your .bashrc. Then launch one worker per cpu with: 'goq work forever &'.

   Easy peasy.

 * secure  : Unlike most parallel job management systems, Goq actually uses strong AES encryption for all communications. This is equivalent to (or better than) the encryption that ssh gives you. You simply manually use ssh initially to distribute the .goq directory (which contains the encryption keys created by 'goq init') to all your worker nodes, and then there is no need for key exchange. This allows you to create images for cloud use that are ready-to-go on bootup. Only hosts on which you have copied the .goq directory to can submit or perform work for the cluster.

 * fast scheduling : unlike other queuing systems (I'm looking at you, gridengine and torque!?!), you don't have wait minutes for your jobs to start. Workers started with 'goq work forever' are waiting to receive work, and start processing instantly when work is submitted. If you want your workers to stop after all jobs are done, just leave off the 'forever' and they will exit after 1000 msec without work.

 * central collection of output  : stdout and stderr from finished jobs is returned to the master-server, in the directory you sepcify with GOQ_ODIR (output directory). This is ./o, by default.


notes on the library we build on
-------------------------

Goq uses a messaging system based 
on the nanocap transport, our term for a combination of the 
nanomsg[1] and Cap'n Proto[2] technologies. Nanomsg is a pre-requisite
that must be installed prior to being able to build Goq.

'make installation' should build and do a local install of nanomsg into
the vendor/install directory. Adjust your LD_LIBRARY_PATH accordingly.

[Note: If you aren't doing development (where you re-compile the schema/zjob.capnp file),
then you should not need to install capnproto. You can just use the pre-compiled
schema.zjob.capnp.go file and the github.com/glycerine/go-capnproto module alone. In
this case, no c++11 compiler should be needed.] If you want to hack on the schema
used for transport, get a c++11 compiler install and then install capnproto[2]. Presto!
Blazingly fast serialization.

[1] nanomsg: http://nanomsg.org/

[2] Cap'n Proto: http://kentonv.github.io/capnproto/



compiling the source
------------

to build:


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

Goq was built using BDD, so the test suite has good coverage. If go test -v reports *any* failures, please file an issue.



usage
-----

[Status] working and useful.

Setup:

 * on your master-server (in your home dir): add 'export GOQ_HOME=/home/yourusername' (without quotes) to your .bashrc / whatever startup file for the shell use you.

   The GOQ_HOME env variable tells Goq where to find your encryption keys. They are stored in $GOQ_HOME/.goq

 * also in your master-server's .bashrc: If the default ip address or default port 1776 don't work for you, simply set the values of GOQ_JSERV_IP and GOQ_JSERV_PORT respectively.

 * goq init : run this once on your server (not on workers) to create your encryption keys. Then use a secure channel (scp) to copy the $GOQ_HOME/.goq directory to all work and submit hosts.

 * on all workers: Once you've securely copied the .goq directory to the worker, also replicate your settings of GOQ_HOME, GOQ_JSERV_IP and GOQ_JSERV_PORT to your worker's .bashrc. (If your workers share a home directory with your server then obviously this copy step is omitted.)

That's all!


Use:

There are three fundamental commands, corresponding to the three roles in the queuing system.

 * goq serve : starts a jobs server, by default on port 1776. Generally you only start one server; only one is needed for most purposes. Of course with a distinct GOQ_HOME and GOQ_JSERV_PORT, you can run as many separate servers as you wish.

 * goq sub *command* {*arguments*}*: submits a job to the job server for queuing. You can 'goq sub' from anywhere, assuming that the environment variables (below) are configured.

 * goq work {forever} : request a job from the job server and executes it, returning the result to the server. Wash, rinse, repeat. A worker will loop forever if started with 'goq work forever'. Otherwise it will work until there are no more jobs, then stop after 1000 msec of inactivity.  Generally you'll want to start a forever worker on each cpu of each compute node in your cluster.

Additional useful commands

 * goq kill *jobid* : kills a previously submitted jobid

 * goq stat : shows a snapshot of the server's internal state

 * goq shutdown : shuts down the job server

 * goq wait *jobid* : waits until the specified job has finished.

configuration
-------------

Configuration is controlled by these environment variables. Only the GOQ_HOME variable is mandatory. The rest have reasonable defaults.

 * GOQ_HOME = tells goq processes where to find their .goq directory of credentials. (required)

 * GOQ_JSERV_IP = the ipv4 address of the server. Default: the first external facing interface discovered.

 * GOQ_JSERV_PORT = the port number the server is listening on (defaults to 1776).

 * GOQ_ODIR = the output directory where the server will write job output. Default: ./o

 * GOQ_SENDTIMEOUT_MSEC = milliseconds of wait before timing-out various network communications (you shouldn't need to adjust this, unless traffic is super heavy and your workers aren't receiving jobs). The current default is 1000 msec.


author: Jason E. Aten, Ph.D. <j.e.aten@gmail.com>.
