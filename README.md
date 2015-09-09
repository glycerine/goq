goq: a queuing and job management system fit for the cloud. Written in Go (golang).
-----------------------------------------------------------------------------------


Goq (Go-queue, or nothing to gawk at!) is a replacement for job management systems such as Sun GridEngine. (Yeah! No Reverse DNS hell on setup!)

Goq's source is small, compact, and easily extended and enhanced. The main file is goq.go. The main dispatch logic is in the Start() routine, which is less than 300 lines long. This compactness is a tribute to the design of Go, in particular to its uniquely expressive channel/select system.

Goq Features: 

 * simple : the system is easy to setup and use. The three roles are server, submitter, and worker. Each is trivial to deploy. See the deploy section below.

 * secure  : Unlike most parallel job management systems that have zero security, Goq uses strong AES encryption for all communications. This is equivalent to (or better than) the encryption that ssh gives you. You simply manually use scp initially to distribute the .goq directory (which contains the encryption keys created by 'goq init') to all your worker nodes, and then there is no need for key exchange. This allows you to create images for cloud use that are ready-to-go on bootup. Only hosts on which you have copied the .goq directory to can submit or perform work for the cluster.

 * fast scheduling : unlike other queuing systems (I'm looking at you, gridengine, Torque!?!), you don't have wait minutes for your jobs to start. Workers started with 'goq work forever' are waiting to receive work, and start processing immediately when work is submitted. If you want your workers to stop after all jobs are done, just leave off the 'forever' and they will exit after 1000 msec without work.

 * easy to setup fault tolerance : jobs are run in isolated process, and can be killed on command. Workers are monitored with heartbeats, and non-responsive workers have their jobs re-queued and re-run. The server can be restarted and the workers will just reconnect once the server comes back up.

 * few dependencies: now that we use mangos, Goq doesn't depend on setting up any C code or any 3rd party database. It is completely self-contained.

 * simple : didn't I say that already? It's worth saying it again. Goq is simple and predictable, so you can build on it.

status
------

Excellent. Working and useful. Only running on Linux/amd64 is actively exercised and supported. I've never tried it on Windows and their are known issues with process sheparding on OSX.

compiling the source : 'go get' will fail the first time; you must run 'make' after 'go get'.
--------------------

 * a) `go get -t -u github.com/glycerine/mangos/compat`

 * b) `go get -u -t github.com/glycerine/goq` # this will fail on the very first time, because gitcommit.go has not yet been generated. ignore the error about gitcommit.go, 'make' will fix it.

 * c) If not already, include $GOPATH/bin in your $PATH. The test suite needs to be able to find goq in your $PATH.

   For example, add a line like this to your ~/.bashrc (assumes GOPATH already set): 

~~~
# add to your ~/.bashrc
export PATH=$GOPATH/bin:$PATH  # might already done.
~~~

   Then save the ~/.bashrc changes, and source them with 

~~~
$ source ~/.bashrc # have changes take effect in the current shell
~~~

 * d) `cd $GOPATH/src/github.com/glycerine/goq; make; go test -v`


Goq was built using BDD, so the test suite has good coverage. If 'go test -v' reports *any* failures, please file an issue.

deploy
------

   a) server: On your master node, set the env variable GOQ_HOME to your home directory (must be a directory where Goq can store job output in a subdir). Then do:

~~~
$ cd $GOQ_HOME
$ goq init     # only needed once.
$ nohup goq serve &   # start the central server
~~~

   b) job submission: 'goq sub mycommand myarg1 myarg2 ...' will submit a job. You can be in any directory; Goq will try to write output to ./o back (by default; controlled by the GOQ_ODIR setting) in that directory. Failing that (if the filesystem on the server is laid out differently from that of the sub host), it will write to $GOQ_HOME. For example:

~~~
$ cd somewhere/where/the/job/wants/to/start
# start by doing 'goq sub' on the same machine 
# that 'goq serve' was launched on. Just to learn the system.
$ goq sub ./myjobscript  
~~~

   c) workers: Start workers on compute nodes by copying the .goq directory to them, setting GOQ_HOME in the env/your .bashrc. Then launch one worker per cpu with: 'nohup goq work forever &'.  For example (assuming linux where /proc exists):

~~~
$ ssh computenode
$ for i in $(seq 1 $(cat /proc/cpuinfo |grep processor|wc -l)); do 
  /usr/bin/nohup goq work forever & done
~~~

The 'runGoqWorker' script in the Goq repo shows how to automate the ssh and start-workers sequence. Even easier: start them automatically on boot (e.g. in /etc/rc.local) of 
your favorite cloud image, and workers will be ready and waiting for you when you bring up that image. Do not run Goq as root. Your regular user login suffices, and is safer.

   d) on your cloud nodes' firewalls, open tcp:1024-65535 so that goq nodes can communicate. All node's clocks should be synced to the same ntp time-source (or at least to within 10 seconds of each other). Goq will interpret old or repeated-and-identical messages as replay-attacks (or network wierdness) and will ignore them.


using the system: goq command reference
---------------------------------------

There are three fundamental commands to goq, corresponding to the three roles in the queuing system. For completeness, they are:

 * goq serve : starts a jobs server, by default on port 1776. Generally you only start one server; only one is needed for most purposes. Of course with a distinct GOQ_HOME and GOQ_JSERV_PORT, you can run as many separate servers as you wish.

 * goq sub *command* {*arguments*}*: submit a job to the job server for running. You can 'goq sub' from anywhere, assuming that GOQ_HOME is set and that the local $GOQ_HOME/.goq contains keys that match those on the server.

 * goq work {forever} : request a job from the job server and execute it, returning the result to the server. Wash, rinse, repeat. A worker will loop forever if started with 'goq work forever'. Otherwise, without the 'forever' argument, the worker does 'one-shot' behavior: it will wait for one job, do that job, and then stop. Generally you'll want to start a forever worker on each cpu of each compute node in your cluster. As for any node in your cluster, GOQ_HOME must be set and $GOQ_HOME/.goq must contain current keys.

Additional useful commands

 * goq stat : shows a snapshot of the server's internal state, including running jobs, waiting jobs (if not enough workers), and waiting workers (if not enough jobs).

 * goq kill *jobid* : kills a previously submitted jobid.

 * goq shutdown : shuts down the job server. Workers stay running, and will re-join the server when it comes back online.

 * goq wait *jobid* : waits until the specified job has finished. The jobid must been for an already started job. Returns immediately if the job is already done.

 * goq immolateworkers : this sounds bloody, as it is. Normally you should leave your workers running once started. They will reconnect to the server automatically if the server is bounced. If you really need to (e.g. if you must change your server port number), this will tell all listening workers to kill themselves. To avoid accidents, it is deliberately hard-to-type.


configuration reference
-----------------------

Configuration is controlled by these environment variables. Only the GOQ_HOME variable is mandatory. The rest have reasonable defaults. Once you have run 'goq init' the settings are kept in $GOQ_HOME/.goq/serverloc, and should be adjusted there.

 * GOQ_HOME = tells goq processes where to find their .goq directory of credentials. (required)

 * GOQ_JSERV_IP = the ipv4 address of the server. Default: the first external facing interface discovered.

 * GOQ_JSERV_PORT = the port number the server is listening on (defaults to 1776).

 * GOQ_ODIR = the output directory where the server will write job output. Default: ./o

 * GOQ_SENDTIMEOUT_MSEC = milliseconds of wait before timing-out various network communications (you shouldn't need to adjust this, unless traffic is super heavy and your workers aren't receiving jobs). The current default is 1000 msec.

 * GOQ_HEARTBEAT_SEC = time period between heartbeats in seconds. (default: 5 seconds). The server will check on jobs this often, and re-queue those from non-responsive (presumed dead) workers.

sample local-only session
-------------------------

Usually you would start a bunch of remote workers. But goq works just fine with local workers, and this is an excellent way to get familiar with the system before deploying to your cluster.

~~~
jaten@i7:~$ export GOQ_HOME=/home/jaten
jaten@i7:~$ goq init
[pid 3659] goq init: key created in '/home/jaten/.goq'.
jaten@i7:~$ goq serve &
[1] 3671
**** [jobserver pid 3671] listening for jobs on 'tcp://10.0.0.6:1776', output to 'o'. GOQ_HOME is '/home/jaten'.
jaten@i7:~$ goq stat
[pid 3686] stats for job server 'tcp://10.0.0.6:1776':
runQlen=0
waitingJobs=0
waitingWorkers=0
jservPid=3671
finishedJobsCount=0
droppedBadSigCount=0
nextJobId=1
jaten@i7:~$ goq sub go/goq/bin/sleep20.sh # sleep for 20 seconds, long enough that we can inspect the stats
**** [jobserver pid 3671] got job 1 submission. Will run 'go/goq/bin/sleep20.sh'.
[pid 3704] submitted job 1 to server at 'tcp://10.0.0.6:1776'.
jaten@i7:~$ goq stat
[pid 3715] stats for job server 'tcp://10.0.0.6:1776':
runQlen=0
waitingJobs=1
waitingWorkers=0
jservPid=3671
finishedJobsCount=0
droppedBadSigCount=0
nextJobId=2
wait 000000   WaitingJob[jid 1] = 'go/goq/bin/sleep20.sh []'   submitted by 'tcp://10.0.0.6:46011'.   
jaten@i7:~$ goq work &  # typically on a remote cpu, local here for simplicity of demonstration. Try 'runGoqWorker hostname' for starting remote nodes.
[2] 3726
jaten@i7:~$ **** [jobserver pid 3671] dispatching job 1 to worker 'tcp://10.0.0.6:37894'.
[pid 3671] dispatched job 1 to worker 'tcp://10.0.0.6:37894'
---- [worker pid 3726; tcp://10.0.0.6:37894] starting job 1: 'go/goq/bin/sleep20.sh' in dir '/home/jaten'

jaten@i7:~$ goq stat
[pid 3744] stats for job server 'tcp://10.0.0.6:1776':
runQlen=1
waitingJobs=0
waitingWorkers=0
jservPid=3671
finishedJobsCount=0
droppedBadSigCount=0
nextJobId=2
runq 000000   RunningJob[jid 1] = 'go/goq/bin/sleep20.sh []'   on worker 'tcp://10.0.0.6:37894'.   
jaten@i7:~$ # wait for awhile
jaten@i7:~$ ---- [worker pid 3726; tcp://10.0.0.6:37894] done with job 1: 'go/goq/bin/sleep20.sh'
**** [jobserver pid 3671] worker finished job 1, removing from the RunQ
[pid 3671] jobserver wrote output for job 1 to file 'o/out.00001'

jaten@i7:~$ ---- [worker pid 3726; tcp://10.0.0.6:37894] worker could not fetch job: recv timed out after 1000 msec: resource temporarily unavailable.


[2]+  Done                    goq work
jaten@i7:~$ goq shutdown
[pid 3767] sent shutdown request to jobserver at 'tcp://10.0.0.6:1776'.
[jobserver pid 3671] jobserver exits in response to shutdown request.
jaten@i7:~$ 

~~~

notes on the serialization library used - for developers
-------------------------

Goq uses a messaging system based 
on the nanocap transport, our term for a combination of the 
nanomsg(1) and Cap'n Proto(2) technologies. Update: we use the mangos(3)
 (compat layer) implementation of nanomsg now. Mangos is entirely written in golang, and therefore there is no C library dependency any more. Goq is now
portable to systems that don't have CGO available. Nice.

If you aren't doing development (where you re-compile the schema/zjob.capnp file),
then you should not need to install capnproto. You can just use the pre-compiled
schema/zjob.capnp.go file and the github.com/glycerine/go-capnproto module alone. In
this case, no c++11 compiler should be needed. If you want to hack on the schema
used for transport, get a c++11 compiler installed, and then install capnproto[2]. Presto!
Blazingly fast serialization.

[1] nanomsg: http://nanomsg.org/

[2] Cap'n Proto: http://kentonv.github.io/capnproto/

[3] mangos: https://github.com/glycerine/mangos



author: Jason E. Aten, Ph.D. <j.e.aten@gmail.com>.
