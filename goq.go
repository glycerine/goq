package main

// copyright(c) 2014, Jason E. Aten
//
// goq : a simple queueing system in go; qsub replacement.
//

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	schema "github.com/glycerine/goq/schema"
	//nn "github.com/glycerine/go-nanomsg"
	nn "github.com/glycerine/mangos/compat"
)

// In this model of work dispatch, there are three roles: submitter(s), a server, and worker(s).
//
// The JobServer handles 4 essential types of job messages (marked with ***),
//   and many other acks/side info requests. But these four are the
//   most important/fundament.
//
/*	JOBMSG_INITIALSUBMIT     JobMsg = 0 // *** submitter requests job be queued/started
	JOBMSG_REQUESTFORWORK           = 2 // *** worker requests a new job (msg and workeraddr only)
	JOBMSG_DELEGATETOWORKER         = 3 // *** worker is sent job with Cmd and Dir filled in.
	JOBMSG_FINISHEDWORK             = 6 // *** worker replies with finished job.
*/

const GoqExeName = "goq"

// for tons of debug output (see also WorkerVerbose)
var Verbose bool

// for a debug/heap/profile webserver on port, set WebDebug = true
var WebDebug bool = true

// for debugging signature issues
var ShowSig bool

var AesOff bool

// number of finished job records to retain in a ring buffer. Oldest are discarded when full.
var DefaultFinishedRingMaxLen = 1000

func init() {
	rand.Seed(time.Now().UnixNano() + int64(GetExternalIPAsInt()) + CryptoRandInt64())
}

func VPrintf(format string, a ...interface{}) {
	if Verbose {
		TSPrintf(format, a...)
	}
}

type control int

const (
	nothing control = iota
	die
	stateToDisk
)

func (cmd control) String() string {
	switch cmd {
	case die:
		return "die"
	}
	return fmt.Sprintf("%d", cmd)
}

var SocketCountPushCache int

// cache the sockets for reuse
type PushCache struct {
	Name string

	Addr     string     // even port number (mnemonic: stdout is 0/even)
	pushSock *nn.Socket // from => pull
	cfg      *Config
}

func NewPushCache(name, addr string, cfg *Config) *PushCache {
	p := &PushCache{
		Name: name,
		Addr: addr,
		cfg:  cfg,
	}

	//var count int = SocketCountPushCache
	//fmt.Printf("\n SocketCountPushCache = %d\n", count)
	t, err := MkPushNN(addr, cfg, false)
	if err != nil {
		pid := os.Getpid()
		fmt.Printf("\n SocketCountPushCache = %d, err = '%s'. Freezing here for debug inspection. pid = %d. errno = %d\n", SocketCountPushCache, err, pid, GetErrno())
		out, _ := exec.Command("lsof", "-p", fmt.Sprintf("%d", pid)).Output()
		fmt.Printf("lsof: '%s'\n", string(out))
		outns, _ := exec.Command("netstat", "-an").Output()
		fmt.Printf("netstat: '%s'\n", string(outns))
		select {}
		panic(err) // panic: too many open files here.
		// researching the too many open files upon restoring from state file:
		//
		//  key advice:
		/* as root:
		echo "\n# increase system IP port limits" >> /etc/sysctl.conf
		echo "net.ipv4.ip_local_port_range = 10000 65535" >> /etc/sysctl.conf
		echo "net.ipv4.tcp_fin_timeout = 10" >> /etc/sysctl.conf
		echo "net.core.somaxconn = 1024" >> /etc/sysctl.conf
		*/
		/* or setting them before reboot:
		  	    sudo sysctl -w net.ipv4.ip_local_port_range="1024 65535"
				sudo sysctl -w net.ipv4.tcp_fin_timeout=10
				sudo sysctl -w net.core.somaxconn=1024
		*/
		//  # should yield 5100 sockets/sec okay. But we still can't start that quickly.
		//
		// from:
		//  http://stackoverflow.com/questions/410616/increasing-the-maximum-number-of-tcp-ip-connections-in-linux
		//
		//Maximum number of connections are impacted by certain limits on both
		//  client & server sides, albeit a little differently.
		//
		//On the client side: Increase the ephermal port range, and decrease the tcp_fin_timeout
		//
		// To find out the default values:
		//
		// sysctl net.ipv4.ip_local_port_range
		// sysctl net.ipv4.tcp_fin_timeout
		// The ephermal port range defines the maximum number of outbound sockets a
		// host can create from a particular I.P. address. The fin_timeout defines
		// the minimum time these sockets will stay in TIME_WAIT state (unusable
		// after being used once). Usual system defaults are:
		//
		// net.ipv4.ip_local_port_range = 32768 61000
		// net.ipv4.tcp_fin_timeout = 60
		//
		// This basically means your system cannot guarantee more than (61000 - 32768) / 60 =
		// 470 sockets [per second (or minute?)]. If you are not happy with that, you could
		// begin with increasing the port_range. Setting the range to 15000 61000 is pretty
		// common these days. You could further increase the availability by decreasing the
		// fin_timeout. Suppose you do both, you should see over 1500 outbound connections,
		// more readily.
		//
		// Added this in my edit:
		// *The above should not be interpreted as the factors impacting system capability
		// for making outbound connections / second. But rather these factors affect system's
		// ability to handle concurrent connections in a sustainable manner for large periods
		// of activity.*
		//
		// Default Sysctl values on a typical linux box for tcp_tw_recycle & tcp_tw_reuse would be
		//
		// net.ipv4.tcp_tw_recycle = 0
		// net.ipv4.tcp_tw_reuse = 0
		// These do not allow a connection in wait state after use, and force them to last the complete time_wait cycle. I recommend setting them to:
		//
		// net.ipv4.tcp_tw_recycle = 1
		// net.ipv4.tcp_tw_reuse = 1
		// This allows fast cycling of sockets in time_wait state and re-using them. But before you do this change make sure that this does not conflict with the protocols that you would use for the application that needs these sockets.
		//
		// On the Server Side: The net.core.somaxconn value has an important role. It limits
		// the maximum number of requests queued to a listen socket. If you are sure of your
		// server application's capability, bump it up from default 128 to something like
		// 128 to 1024. Now you can take advantage of this increase by modifying the listen
		// backlog variable in your application's listen call, to an equal or higher integer.
		//
		// txqueuelen parameter of your ethernet cards also have a role to play. Default values are 1000, so bump them up to 5000 or even more if your system can handle it.
		//
		// Similarly bump up the values for net.core.netdev_max_backlog and net.ipv4.tcp_max_syn_backlog.
		// Their default values are 1000 and 1024 respectively.
		//
		// Now remember to start both your client and server side applications by increasing the
		// FD ulimts, in the shell.
		//
		// Besides the above one more popular technique used by programmers is to reduce the
		// number of tcp write calls. My own preference is to use a buffer wherein I push the
		// data I wish to send to the client, and then at appropriate points I write out the
		// buffered data into the actual socket. This technique allows me to use large data
		// packets, reduce fragmentation, reduces my CPU utilization both in the userland at
		// kernel-level.

		// still running out of resources, 'too many open files' when trying to make a new socket.
		/*

			try raising max_map_count:
						         https://my.vertica.com/docs/CE/5.1.1/HTML/index.htm#12962.htm
						    	 http://stackoverflow.com/questions/11683850/how-much-memory-could-vm-use-in-linux

								 sysctl vm.max_map_count
								 vm.max_map_count = 65530

								 #may be too low
								 echo 65535 > /proc/sys/vm/max_map_count
								 echo "vm.max_map_count = 16777216" | tee -a /etc/sysctl.conf
								 sudo sysctl -p
								 #logout and back in
		*/
	}
	SocketCountPushCache++

	p.pushSock = t

	return p
}

// re-create socket on-demand. Used because we may close
// sockets to keep from using too many.
func (p *PushCache) DemandPushSock() *nn.Socket {
	if p.pushSock == nil {
		t, err := MkPushNN(p.Addr, p.cfg, false)
		if err != nil {
			panic(err) // panic: too many open files here.
		}
		p.pushSock = t
	}
	return p.pushSock
}

func (p *PushCache) Close() {
	// SetLinger is essential or else cancel and immo tests which need submit-replies will fail.
	p.pushSock.SetLinger(2 * time.Second)
	p.pushSock.Close()
	p.pushSock = nil
	SocketCountPushCache--
}

// Job represents a job to perform, and is our universal message type.
type Job struct {
	Id       int64
	Msg      schema.JobMsg
	Aboutjid int64 // in acksubmit, this holds the jobid of the job on the runq, so that Id can be unique and monotonic.

	Cmd      string
	Args     []string
	Out      []string
	Env      []string
	Err      string
	HadError bool
	Host     string
	Stm      int64
	Etm      int64
	Elapsec  int64
	Status   string
	Subtime  int64
	Pid      int64
	Dir      string

	Submitaddr string
	Serveraddr string
	Workeraddr string

	Finishaddr []string // who, if anyone, you want notified upon job completion. JOBMSG_JOBFINISHEDNOTICE will be sent.

	Signature string
	IsLocal   bool
	Cancelled bool

	ArrayId int64
	GroupId int64

	Delegatetm     int64
	Lastpingtm     int64
	Unansweredping int64
	Sendtime       int64
	Sendernonce    int64

	// not serialized, just used
	// for routing
	destinationSock *nn.Socket

	Runinshell bool
	MaxShow    int64
	CmdOpts    uint64
}

func (j *Job) String() string {
	if j == nil {
		return "&Job{nil}"
	} else {
		return fmt.Sprintf("&Job{Id:%d Msg:%s Aboutjid:%d Cmd:%s Args:%#v Out:%#v Submitaddr:%s Serveraddr: %s Workeraddr: %s Sendtime: %s Sendernonce: %x}", j.Id, j.Msg, j.Aboutjid, j.Cmd, j.Args, j.Out, j.Submitaddr, j.Serveraddr, j.Workeraddr, time.Unix(j.Sendtime/1e9, j.Sendtime%1e9), j.Sendernonce)
	}
}

func NewJob() *Job {
	j := &Job{
		Id:         0, // only server should assign job.Id, until then, should be 0.
		Args:       make([]string, 0),
		Out:        make([]string, 0),
		Env:        make([]string, 0),
		Finishaddr: make([]string, 0),
	}
	StampJob(j) // also in sendZjob, but here to support local job sends.
	return j
}

// only JobServ assigns Ids, submitters and workers just leave Id == 0.
func (js *JobServ) NewJobId() int64 {
	id := js.NextJobId
	js.NextJobId++
	return id
}

func (js *JobServ) RegisterWho(j *Job) {

	// add addresses and sockets if not created already
	if j.Workeraddr != "" {
		if _, ok := js.Who[j.Workeraddr]; !ok {
			js.Who[j.Workeraddr] = NewPushCache(j.Workeraddr, j.Workeraddr, &js.Cfg)
		}
	}

	if j.Submitaddr != "" {
		if _, ok := js.Who[j.Submitaddr]; !ok {
			js.Who[j.Submitaddr] = NewPushCache(j.Submitaddr, j.Submitaddr, &js.Cfg)
		}
	}

}

func (js *JobServ) UnRegisterWho(j *Job) {
	// add addresses and sockets if not created already
	if j.Workeraddr != "" {
		if c, found := js.Who[j.Workeraddr]; found {
			c.Close()
			delete(js.Who, j.Workeraddr)
		}
	}

	if j.Submitaddr != "" {
		if c, found := js.Who[j.Submitaddr]; found {
			c.Close()
			delete(js.Who, j.Submitaddr)
		}
	}

}

func (js *JobServ) UnRegisterSubmitter(j *Job) {
	if j.Submitaddr != "" {
		if c, found := js.Who[j.Submitaddr]; found {
			c.Close()
			delete(js.Who, j.Submitaddr)
		}
	}
}

// assume these won't be long running finishers, so don't cache them in Who
func (js *JobServ) FinishersToNewSocket(j *Job) []*nn.Socket {

	res := make([]*nn.Socket, 0)
	for i := range j.Finishaddr {
		addr := j.Finishaddr[i]
		if addr == "" {
			panic("addr in Finishers should never be empty")
		}
		t, err := MkPushNN(addr, &js.Cfg, false)
		if err != nil {
			panic(err)
		}
		res = append(res, t)
	}
	return res
}

func (js *JobServ) CloseRegistry() {
	for _, pp := range js.Who {
		if pp.pushSock != nil {
			pp.Close()
		}
	}
}

func (js *JobServ) Shutdown() {
	VPrintf("at top of JobServ::Shutdown()\n")
	js.ShutdownListener()
	VPrintf("in JobServ::Shutdown(): after ShutdownListener()\n")

	js.CloseRegistry()
	VPrintf("in JobServ::Shutdown(): after CloseRegistry()\n")

	if js.Nnsock != nil {
		js.Nnsock.Close()
	}
	js.stateToDisk()
	VPrintf("in JobServ::Shutdown(): after stateToDisk()\n")

	if WebDebug {
		VPrintf("calling js.Web.Stop()\n")
		js.Web.Stop()
		VPrintf("returned from js.Web.Stop()\n")
	}
}

func (js *JobServ) ShutdownListener() {
	if !js.IsLocal {
		// closing the js.ListenerShtudown channel allows us to broadcast
		// all the places the listener might be trying to send to JobServ.
		VPrintf("in ShutdownListener, about to call CloseChannelIfOpen()\n")
		CloseChannelIfOpen(js.ListenerShutdown)
		VPrintf("in ShutdownListener, after CloseChannelIfOpen()\n")
		<-js.ListenerDone
		VPrintf("in ShutdownListener, after <-js.ListenerDone\n")
	}
}

func (js *JobServ) stateFilename() string {
	return fmt.Sprintf("%s/serverstate", js.dotGoqPath())
}

func (js *JobServ) dotGoqPath() string {
	return fmt.Sprintf("%s/.goq", js.Cfg.Home)
}

func (js *JobServ) stateToDisk() {
	fn := js.stateFilename()
	dir := js.dotGoqPath()

	file, err := ioutil.TempFile(dir, "new.serverstate")
	if err != nil {
		if strings.HasSuffix(err.Error(), "no such file or directory") {
			TSPrintf("[pid %d] job server error: stateToDisk() could not find file '%s': %s\n", os.Getpid(), fn, err)
			return
		} else {
			panic(err)
		}
	}

	buf, _ := js.ServerToCapnp()
	file.Write(buf.Bytes())

	file.Close()

	// delete old file
	err = os.Remove(fn)
	if err != nil {
		// it might not exist. that's okay, don't panic.
	}

	// rename into its place
	err = os.Rename(file.Name(), fn)
	if err != nil {
		panic(err)
	}

	VPrintf("[pid %d] stateToDisk() done: wrote state (js.NextJobId=%d) to '%s'\n", os.Getpid(), js.NextJobId, fn)
}

func (js *JobServ) diskToState() {
	fn := js.stateFilename()

	js.checkForOldStateFile(fn)

	file, err := os.Open(fn)
	if err != nil {
		if strings.HasSuffix(err.Error(), "no such file or directory") {
			VPrintf("[pid %d] diskToState() done: no state file found in '%s'\n", os.Getpid(), fn)
			return
		} else {
			panic(err)
		}
	}
	defer file.Close()
	js.SetStateFromCapnp(file, fn)

	VPrintf("[pid %d] diskToState() done: read state (js.NextJobId=%d) from '%s'\n", os.Getpid(), js.NextJobId, fn)
}

type Address string

// JobServ represents the single central job server.
type JobServ struct {
	Name string

	Nnsock *nn.Socket // receive on
	Addr   string

	Submit          chan *Job  // submitter sends on, JobServ receives on.
	ReSubmit        chan int64 // dispatch go-routine sends on when worker is unreachable, JobServ receives on.
	WorkerReady     chan *Job  // worker sends on, JobServ receives on.
	ToWorker        chan *Job  // worker receives on, JobServ sends on.
	RunDone         chan *Job  // worker sends on, JobServ receives on.
	SigMismatch     chan *Job  // Listener tells Start about bad signatures.
	BadNonce        chan *Job  // Listener tells Start about bad nonce (duplicate nonce or stale timestamp)
	SnapRequest     chan *Job  // worker requests state snapshot from JobServ.
	ObserveFinish   chan *Job  // submitter sends on, Jobserv recieves on; when a submitter wants to wait for another job to be done.
	NotifyFinishers chan *Job  // submitter receives on, jobserv dispatches a notification message for each finish observer
	Cancel          chan *Job  // submitter sends on, to request job cancellation.
	ImmoReq         chan *Job  // submitter sends on, to requst all workers die.
	WorkerDead      chan *Job  // worker tells server just before terminating self.
	WorkerAckPing   chan *Job  // worker replies to server that it is still alive. If working on job then Aboutjid is set.

	UnregSubmitWho chan *Job // JobServ internal use: unregister submitter only.

	DeafChan chan int // supply CountDeaf, when asked.

	WaitingJobs     []*Job
	RunQ            map[int64]*Job
	KnownJobHash    map[int64]*Job
	DedupWorkerHash map[string]bool
	Ctrl            chan control
	Done            chan bool
	WaitingWorkers  []*Job
	Pid             int
	Odir            string
	NextJobId       int64

	// listener shutdown
	ListenerShutdown chan bool // tell listener to stop by closing this channel.
	ListenerDone     chan bool // listener closes this channel when finished.

	// allow cancel test to not race
	FirstCancelDone chan bool // server closes this after hearing on RunDone a job with .Cancelled set.

	// directory of submitters and workers
	Who     map[string]*PushCache
	WhoLock sync.RWMutex

	// Finishers : who wants to be notified when a job is done.
	Finishers map[int64][]Address

	CountDeaf         int
	PrevDeaf          int
	BadSgtCount       int64
	FinishedJobsCount int64
	CancelledJobCount int64
	BadNonceCount     int64

	// set Cfg *once*, before any goroutines start, then
	// treat it as immutable and never changing.
	Cfg       Config
	DebugMode bool // show badsig messages if true
	IsLocal   bool

	NoReplay *NonceRegistry // only ListenForJobs() goroutine should queries/updates this; never Start().

	FinishedRing       []*Job
	FinishedRingMaxLen int
	Web                *WebServer
}

// DeafChanIfUpdate: don't make consumers of DeafChan busy wait;
// send only upon update
func (js *JobServ) DeafChanIfUpdate() chan int {
	if js.CountDeaf != js.PrevDeaf {
		return js.DeafChan
	} else {
		return nil
	}
}

func (js *JobServ) SubmitJob(j *Job) error {
	fmt.Printf("SubmitJob called.\n")
	j.Msg = schema.JOBMSG_INITIALSUBMIT
	js.Submit <- j
	return nil
}

func NewExternalJobServ(cfg *Config) (pid int, err error) {
	// make sure that this external 'goq' version matches
	// our own.
	detectVersionSkew()

	//argv := os.Argv()
	cmd := exec.Command(GoqExeName, "serve")

	cmd.Env = cfg.Setenv(os.Environ())

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Start()

	// reap so we don't zombie-fy, which makes
	// it difficult for the test in fetch_test.go to detect that
	// the process is indeed gone. This one liner fixes all that.
	go func() { cmd.Wait() }()

	if err != nil {
		// if file not found, there will be no child pid;
		// cmd.Process will be nil, and trying to fetch cmd.Process.Pid
		// will crash.
		return -1, err
	}

	return cmd.Process.Pid, err
}

// avoid version skew in tests when using an external binary 'goq'.
// go install or make avoids the issue, but sometimes that is forgotten,
// and we need a reminder to run make.
// Called by NewExternalJobServ(), perhaps others.
func detectVersionSkew() {
	ver, err := exec.Command(GoqExeName, "version").Output()
	if err != nil {
		panic(err)
	}
	ver = bytes.TrimRight(ver, "\n")
	my_ver := goq_version()
	vers := string(ver)
	if vers != my_ver {
		panic(fmt.Sprintf("version skew detected, please run 'make' in the goq/ source directory to install the most recent 'goq' into $GOPATH/bin, and be sure that $GOPATH/bin is at the front of your $PATH. Version of 'goq' installed in path: '%s'. Version of this build: '%s'\n", vers, my_ver))
	}
}

func NewJobServ(cfg *Config) (*JobServ, error) {

	var err error
	if cfg == nil {
		cfg = DefaultCfg()
	}

	addr := cfg.JservAddr()

	if cfg.Cypher == nil {
		var key *CypherKey
		key, err = OpenExistingOrCreateNewKey(cfg)
		if err != nil || key == nil {
			panic(fmt.Sprintf("could not open or create encryption key: %s", err))
		}
		cfg.Cypher = key
	}

	MoveToDirOrPanic(cfg.Home)

	var pullsock *nn.Socket
	var remote bool
	if cfg.JservIP != "" {
		remote = true
		pullsock, err = MkPullNN(addr, cfg, false)
		if err != nil {
			panic(err)
		}

		VPrintf("[pid %d] JobServer bound endpoints addr: '%s'\n", os.Getpid(), addr)
	}

	js := &JobServ{
		Name:   fmt.Sprintf("jobserver.pid.%d", os.Getpid()),
		Addr:   addr,
		Nnsock: pullsock,
		RunQ:   make(map[int64]*Job),

		// KnownJobHash tracks actual (numbered) jobs, not worker-ready/request-to-object jake-job requests.
		KnownJobHash: make(map[int64]*Job), // for fast lookup, jobs are either on WaitingJobs slice, or in RunQ table.

		// avoid the same worker doublying up and filling the worker queue
		//  thus we avoid lots of spurious dispatch attempts.
		DedupWorkerHash: make(map[string]bool),

		WaitingJobs:   make([]*Job, 0),
		Submit:        make(chan *Job),
		ReSubmit:      make(chan int64),
		WorkerReady:   make(chan *Job),
		ToWorker:      make(chan *Job),
		RunDone:       make(chan *Job),
		SigMismatch:   make(chan *Job),
		BadNonce:      make(chan *Job),
		SnapRequest:   make(chan *Job),
		Cancel:        make(chan *Job),
		ImmoReq:       make(chan *Job),
		WorkerDead:    make(chan *Job),
		WorkerAckPing: make(chan *Job),

		ObserveFinish:   make(chan *Job), // when a submitter wants to wait for another job to be done.
		NotifyFinishers: make(chan *Job),

		DeafChan:       make(chan int),
		Ctrl:           make(chan control),
		Done:           make(chan bool),
		WaitingWorkers: make([]*Job, 0),
		Who:            make(map[string]*PushCache),
		Finishers:      make(map[int64][]Address),

		ListenerShutdown: make(chan bool),
		ListenerDone:     make(chan bool),
		//ListenerAckShutdown: make(chan bool),

		FirstCancelDone: make(chan bool),
		Pid:             os.Getpid(),
		Cfg:             *cfg,
		DebugMode:       cfg.DebugMode,
		Odir:            cfg.Odir,
		IsLocal:         !remote,
		NextJobId:       1,
		NoReplay:        NewNonceRegistry(NewRealTimeSource()),

		FinishedRingMaxLen: DefaultFinishedRingMaxLen,
		FinishedRing:       make([]*Job, 0, DefaultFinishedRingMaxLen),

		UnregSubmitWho: make(chan *Job),
	}

	VPrintf("ListenerShutdown channel created in ctor.\n")
	js.diskToState()

	if WebDebug {
		js.Web = NewWebServer()
	}
	js.Start()
	if remote {
		//VPrintf("remote, server starting ListenForJobs() goroutine.\n")
		TSPrintf("**** [jobserver pid %d] listening for jobs on '%s', output to '%s'. GOQ_HOME is '%s'.\n", js.Pid, js.Addr, js.Odir, js.Cfg.Home)
		js.ListenForJobs(cfg)
	}

	return js, nil
}

func (js *JobServ) toWorkerChannelIfJobAvail() chan *Job {
	if len(js.WaitingJobs) == 0 {
		return nil
	}
	return js.ToWorker
}

func (js *JobServ) nextJob() *Job {
	if len(js.WaitingJobs) == 0 {
		return nil
	}
	js.WaitingJobs[0].Msg = schema.JOBMSG_DELEGATETOWORKER
	return js.WaitingJobs[0]
}

func (js *JobServ) ConfirmOrMakeOutputDir(dirname string) error {
	if !DirExists(dirname) {
		err := os.Mkdir(dirname, 0775)
		if err != nil {
			return err
		}
	}
	return nil
}

func (js *JobServ) WriteJobOutputToDisk(donejob *Job) {
	VPrintf("WriteJobOutputToDisk() called for Job: %s\n", donejob)

	var err error
	local := false
	var fn string
	var odir string

	// the directories on the submit host (where we start) may not match those on
	// the server host where (where we finish), but it would be a common situation
	// to have them be on the same host, hence we try to write back to donejob.Dir
	// if at all possible.
	if DirExists(donejob.Dir) {

		odir = fmt.Sprintf("%s/%s", donejob.Dir, js.Odir)
		err = js.ConfirmOrMakeOutputDir(odir)
		if err == nil {
			local = true
		}
		fn = fmt.Sprintf("%s/%s/out.%05d", donejob.Dir, js.Odir, donejob.Id)
	}

	// local is false, Drat, couldn't write to Dir on the server-host.
	// Instead write to $GOQ_HOME/$GOQ_ODIR
	if !local {
		odir = fmt.Sprintf("%s/%s", js.Cfg.Home, js.Odir)
		err = js.ConfirmOrMakeOutputDir(odir)
		if err != nil {
			TSPrintf("[pid %d] server job-done badness: could not make output directory '%s' for job %d output.\n", js.Pid, odir, donejob.Id)
			return
		}
		fn = fmt.Sprintf("%s/%s/out.%05d", js.Cfg.Home, js.Odir, donejob.Id)
		TSPrintf("[pid %d] drat, could not get to the submit-directory for job %d. Output to '%s' instead.\n", js.Pid, donejob.Id, fn)
	}
	// invar: fn is set.

	// append if already existing file: so we can have incremental updates.
	var file *os.File
	if FileExists(fn) {
		file, err = os.OpenFile(fn, os.O_RDWR|os.O_APPEND, 0666)
	} else {
		file, err = os.Create(fn)
	}
	if err != nil {
		panic(err)
	}
	defer file.Close()
	for i := range donejob.Out {
		fmt.Fprintf(file, "%s\n", donejob.Out[i])
	}
	TSPrintf("[pid %d] jobserver wrote output for job %d to file '%s'\n", js.Pid, donejob.Id, fn)
}

func (js *JobServ) Start() {

	go func() {
		// Save state to disk on each heartbeat.
		// Currently state is just NextJobId. See stateToDisk()
		heartbeat := time.Tick(time.Duration(js.Cfg.Heartbeat) * time.Second)

		var loopcount int64 = 0
		for {
			loopcount++
			VPrintf(" - - - JobServ at top for Start() event loop, loopcount: (%d).\n", loopcount)

			select {
			case newjob := <-js.Submit:
				VPrintf("  === event loop case ===  (%d) JobServ got from Submit channel a newjob, msg: %s, job: %s\n", loopcount, newjob.Msg, newjob)

				if newjob.Id != 0 {
					panic(fmt.Sprintf("new jobs should have zero (unassigned) Id!!! But, this one did not: %s", newjob))
				}

				curId := js.NewJobId()
				newjob.Id = curId
				js.KnownJobHash[curId] = newjob

				// open and cache any sockets we will need.
				js.RegisterWho(newjob)

				if newjob.Msg == schema.JOBMSG_SHUTDOWNSERV {
					VPrintf("JobServ got JOBMSG_SHUTDOWNSERV from Submit channel.\n")
					go func() { js.Ctrl <- die }()
					continue
				}

				TSPrintf("**** [jobserver pid %d] got job %d submission. Will run '%s'.\n", js.Pid, newjob.Id, newjob.Cmd)

				js.WaitingJobs = append(js.WaitingJobs, newjob)
				js.Dispatch()
				// we just dispatched, now reply to submitter with ack (in an async goroutine); they don't need to
				// wait for it, but often they will want confirmation/the jobid.
				js.AckBack(newjob, newjob.Submitaddr, schema.JOBMSG_ACKSUBMIT, []string{})

			case resubId := <-js.ReSubmit:
				VPrintf("  === event loop case === (%d) JobServ got resub for jobid %d\n", loopcount, resubId)
				js.CountDeaf++
				resubJob, ok := js.RunQ[resubId]
				if !ok {
					// maybe it was cancelled in the meantime. don't panic.
					TSPrintf("**** [jobserver pid %d] got re-submit of job %d that is now not on our RunQ, so dropping it without re-queuing.\n", js.Pid, resubId)
					continue
				}
				js.Resub(resubJob)

			case ackping := <-js.WorkerAckPing:
				j, ok := js.RunQ[ackping.Aboutjid]
				if ok {
					j.Unansweredping = 0
					now := time.Now()
					j.Lastpingtm = now.UnixNano()
					if ackping.Workeraddr != j.Workeraddr {
						panic(fmt.Sprintf("ackping.Workeraddr(%s) must match j.Workeraddr(%s)", ackping.Workeraddr, j.Workeraddr))
					}
					if j.Id != ackping.Aboutjid {
						panic(fmt.Sprintf("messed up RunQ?? j.Id(%d) must match ackping.Aboutjid(%d). RunQ: %#v", j.Id, ackping.Aboutjid, js.RunQ))
					}
					VPrintf("**** [jobserver pid %d] got ackping worker at '%s' running job %d. Lastpingtm now: %s\n", js.Pid, j.Workeraddr, j.Id, now)
					// record info about running process:
					j.Pid = ackping.Pid
					j.Stm = ackping.Stm

				} else {
					TSPrintf("**** [jobserver pid %d] Problem? got ping back from worker at '%s' running job %d that was not in our RunQ???\n", js.Pid, ackping.Workeraddr, ackping.Aboutjid)
				}

			case reqjob := <-js.WorkerReady:
				VPrintf("  === event loop case === (%d) JobServ got request for work from WorkerReady channel: %s\n", loopcount, reqjob)
				if !js.IsLocal && reqjob.Workeraddr == "" {
					// ignore bad packets
				}
				js.RegisterWho(reqjob)
				if _, dup := js.DedupWorkerHash[reqjob.Workeraddr]; !dup {
					js.WaitingWorkers = append(js.WaitingWorkers, reqjob)
					js.DedupWorkerHash[reqjob.Workeraddr] = true
				} else {
					VPrintf("**** [jobserver pid %d] ignored duplicate worker-ready message from '%s'\n", js.Pid, reqjob.Workeraddr)
				}
				// TODO: if this worker had a job on the RunQ, take it off. Assume that the worker died while running it.
				// It looks wierd to have a worker show up on both WaitingWorkers and the RunQ.
				js.Dispatch()

			case donejob := <-js.RunDone:
				VPrintf("  === event loop case === (%d)  JobServ got donejob from RunDone channel: %s\n", loopcount, donejob)
				// we've got a new copy, with Out on it, but the old copy may have added listeners, so
				// we'll need to merge in those Finishaddr too.
				if donejob.Cancelled {
					VPrintf("jserv: got donejob on js.RunDone that has .Cancelled set. donejob: %s\n", donejob)
					js.CancelledJobCount++
					if js.CancelledJobCount == 1 {
						// allow cancel_test.go to not race
						close(js.FirstCancelDone)
					}
				} else {
					VPrintf("jserv: got donejob on js.RunDone without .Cancelled set. donejob: %s\n", donejob)
				}

				withFinishers, ok := js.RunQ[donejob.Id]
				if !ok {
					// just ignore, probably a re-issued job that finally woke up and came back.
					// panic(fmt.Sprintf("got donejob %d for job(%s) from js.RunDone channel, but it was not in our js.RunQ: %#v", donejob.Id, donejob, js.RunQ))
					fmt.Sprintf("ignoring donejob %d for job(%s) since js.RunQ does not show it active.\n", donejob.Id, donejob)
					continue
				}
				kjh, ok := js.KnownJobHash[donejob.Id]
				if !ok {
					// just ignore, probably a re-issued job that finally woke up and came back.
					if js.DebugMode {
						TSPrintf("\n jobserv debugmode: got donejob %d for job(%s) from js.RunDone channel, but it was not in our js.KnownJobHash: %#v\n", donejob.Id, donejob, js.KnownJobHash)
					}
					continue
				}
				if withFinishers != kjh {
					panic(fmt.Sprintf("withFinishers(%v) from RunQ did not agree with kjh(%v) from KnownJobHash", withFinishers, kjh))
				}

				donejob.Finishaddr = js.MergeAndDedupFinishers(donejob, withFinishers)

				delete(js.RunQ, donejob.Id)
				delete(js.KnownJobHash, donejob.Id)
				js.FinishedJobsCount++
				TSPrintf("**** [jobserver pid %d] worker finished job %d, removing from the RunQ\n", js.Pid, donejob.Id)
				js.WriteJobOutputToDisk(donejob)
				js.TellFinishers(donejob, schema.JOBMSG_JOBFINISHEDNOTICE)
				js.AddToFinishedRingbuffer(donejob)

			case cmd := <-js.Ctrl:
				VPrintf("  === event loop case zowza === (%d)  JobServ got control cmd: %v\n", loopcount, cmd)
				switch cmd {
				case die:
					TSPrintf("**** [jobserver pid %d] jobserver got 'die' cmd on js.Ctrl. about to call js.Shutdown().\n", js.Pid)
					js.Shutdown()
					TSPrintf("**** [jobserver pid %d] jobserver got 'die' cmd on js.Ctrl. js.Shutdown() done. Exiting.\n", js.Pid)
					close(js.Done)
					return
				case stateToDisk:
					js.stateToDisk()
				}

			case js.DeafChanIfUpdate() <- js.CountDeaf:
				VPrintf("  === event loop case === (%d)  JobServ supplied js.CountDeaf on channel js.DeafChan.\n", loopcount)
				// only one consumer gets each change; we only send on js.DeafChan once
				// when CountDeaf changes; this prevents our (only) client from busy waiting.
				js.PrevDeaf = js.CountDeaf

			case badsigjob := <-js.SigMismatch:
				//nothing doing with this job, it had a bad signature
				js.BadSgtCount++
				// ignore badsig packets; to prevent a bad worker from infinite looping/DOS-ing us.
				if js.DebugMode {
					addr := badsigjob.Submitaddr
					if addr == "" {
						addr = badsigjob.Workeraddr
					}

					TSPrintf("**** [jobserver pid %d] DebugMode: actively rejecting badsig message from '%s'.\n", js.Pid, addr)
					if addr != "" {
						js.RegisterWho(badsigjob)
						js.AckBack(badsigjob, addr, schema.JOBMSG_REJECTBADSIG, []string{})
					}
				}

			case badnoncejob := <-js.BadNonce:
				// job was too old or duplicate (replay attack) nonce detected.
				js.BadNonceCount++
				if js.DebugMode {
					addr := badnoncejob.Submitaddr
					if addr == "" {
						addr = badnoncejob.Workeraddr
					}

					TSPrintf("**** [jobserver pid %d] DebugMode: badnonce/too old message from '%s' (js.BadNonceCount now: %d): '%s'.\n", js.Pid, addr, js.BadNonceCount, badnoncejob)
				}

			case snapreq := <-js.SnapRequest:
				VPrintf("\nStart: got snapreq: '%#v'\n", snapreq)
				js.RegisterWho(snapreq)

				shot := js.AssembleSnapShot(int(snapreq.MaxShow))
				js.AckBack(snapreq, snapreq.Submitaddr, schema.JOBMSG_ACKTAKESNAPSHOT, shot)
				//VPrintf("\nHandling snapreq: done with AckBack; shot was: '%#v'\n", shot)

			case canreq := <-js.Cancel:
				var j *Job
				var ok bool
				js.RegisterWho(canreq)
				canid := canreq.Aboutjid
				if j, ok = js.KnownJobHash[canid]; !ok {
					js.AckBack(canreq, canreq.Submitaddr, schema.JOBMSG_JOBNOTKNOWN, []string{})
					goto unreg
				}

				if _, running := js.RunQ[canid]; running {
					// tell worker to stop
					js.AckBack(canreq, j.Workeraddr, schema.JOBMSG_CANCELWIP, []string{})
					VPrintf("**** [jobserver pid %d] server sent 'cancelwip' for job %d to '%s'.\n", js.Pid, canid, j.Workeraddr)
				}

				// if we don't  remove from RunQ and KJH immediately, it looks wierd to the user.
				delete(js.RunQ, canid)
				delete(js.KnownJobHash, canid)
				js.RemoveFromWaitingJobs(j)

				js.TellFinishers(j, schema.JOBMSG_CANCELSUBMIT)

				// don't tell finishers twice
				j.Finishaddr = j.Finishaddr[:0]

				js.AckBack(canreq, canreq.Submitaddr, schema.JOBMSG_ACKCANCELSUBMIT, []string{})
			unreg:
				//js.UnRegisterWho(canreq) // ackback should take care of this now, right?
				TSPrintf("**** [jobserver pid %d] server cancelled job %d per request of '%s'.\n", js.Pid, canid, canreq.Submitaddr)

			case obsreq := <-js.ObserveFinish:
				if obsreq.Submitaddr == "" {
					// ignore bad requests
					if js.DebugMode {
						TSPrintf("**** [jobserver pid %d] DebugMode: got Observe Request with bad Submitaddr: %s.\n", js.Pid, obsreq)
					}
					continue
				}
				if j, ok := js.KnownJobHash[obsreq.Aboutjid]; ok {
					// still in progress, so we add this requester to the Finishaddr list
					TSPrintf("**** [jobserver pid %d] noting request to get notice about the finish of job %d from '%s'.\n", js.Pid, obsreq.Aboutjid, obsreq.Submitaddr)
					j.Finishaddr = append(j.Finishaddr, obsreq.Submitaddr)
				} else {
					// probably already finished
					TSPrintf("**** [jobserver pid %d] impossible request for finish-notify oh job %d (unknown job) from '%s'. Sending JOBMSG_JOBNOTKNOWN\n", js.Pid, obsreq.Aboutjid, obsreq.Submitaddr)
					fakedonejob := NewJob()
					fakedonejob.Id = obsreq.Aboutjid
					fakedonejob.Finishaddr = []string{obsreq.Submitaddr}
					js.TellFinishers(fakedonejob, schema.JOBMSG_JOBNOTKNOWN)
				}
			case <-heartbeat:
				js.stateToDisk()
				js.PingJobRunningWorkers()
				// print status too on every heartbeat
				lines := js.AssembleSnapShot(10)
				fmt.Printf("\nsnap-shot-of-goq-serve\n")
				for i := range lines {
					fmt.Println(lines[i])
				}

			case immoreq := <-js.ImmoReq:
				js.RegisterWho(immoreq)
				js.ImmolateWorkers(immoreq)
				js.AckBack(immoreq, immoreq.Submitaddr, schema.JOBMSG_IMMOLATEACK, []string{})

			case wd := <-js.WorkerDead:
				delete(js.DedupWorkerHash, wd.Workeraddr)

			case job := <-js.UnregSubmitWho:
				js.UnRegisterSubmitter(job)

			}
		}
	}()
}

func (js *JobServ) Dispatch() {
	for len(js.WaitingWorkers) != 0 && len(js.WaitingJobs) != 0 {
		job := js.WaitingJobs[0]
		js.WaitingJobs = js.WaitingJobs[1:]

		readyrequest := js.WaitingWorkers[0]
		js.WaitingWorkers = js.WaitingWorkers[1:]
		delete(js.DedupWorkerHash, readyrequest.Workeraddr)

		js.DispatchJobToWorker(readyrequest, job)
	}
}

func (js *JobServ) Resub(resubJob *Job) {
	TSPrintf("**** [jobserver pid %d] got re-submit of job %d that was dispatched to '%s'. Trying again.\n", js.Pid, resubJob.Id, resubJob.Workeraddr)

	resubJob.Workeraddr = ""
	resubJob.Lastpingtm = 0
	resubJob.Unansweredping = 0
	delete(js.RunQ, resubJob.Id)
	// prepend, so the job doesn't loose its place in line. *try* to FIFO as much as possible.
	js.WaitingJobs = append([]*Job{resubJob}, js.WaitingJobs...)
	js.Dispatch()
}

func (js *JobServ) PingJobRunningWorkers() {
	now := Ntm(time.Now().UnixNano())
	hb := js.Cfg.Heartbeat // seconds
	timeout := Tmsec2Ntm(hb)
	twotimeouts := 2 * timeout

	for _, j := range js.RunQ {
		elap := now - MaxNtm(Ntm(j.Delegatetm), Ntm(j.Lastpingtm))
		if elap < timeout {
			continue
		}
		if j.Unansweredping == 0 {
			VPrintf("**** [jobserver pid %d] (elapsed = %.1f sec) heartbeat pinging worker '%s' with running job %d.\n",
				js.Pid, float64(elap)/1e9, j.Workeraddr, j.Id)
			j.Aboutjid = j.Id
			j.Unansweredping = 1
			js.AckBack(j, j.Workeraddr, schema.JOBMSG_PINGWORKER, []string{})
			continue
		}

		if elap > twotimeouts {
			// its been at least two timeouts since job was last heard from
			js.DeadWorkerResubJob(j, float64(elap)/1e9)
			continue
		}

	}
}

func (js *JobServ) DeadWorkerResubJob(j *Job, elapSec float64) {
	TSPrintf("**** [jobserver pid %d] sees dead worker for job %d (no ping reply after %.1f sec). Resubmitting.\n", js.Pid, j.Id, elapSec)
	js.Resub(j)
}

func (js *JobServ) RemoveFromWaitingJobs(j *Job) {
	wl := len(js.WaitingJobs)
	if wl == 0 {
		return
	}
	slice := make([]*Job, wl)
	k := 0
	//found := false
	for _, v := range js.WaitingJobs {
		if v == j {
			//found = true
		} else {
			slice[k] = v
			k++
		}
	}
	//if !found {
	// this is okay, sometimes we are called just taken as a precaution, as when cancelling
	// a job that is waiting and not running. Don't have a cow. i.e. Don't panic.
	//}
	js.WaitingJobs = slice[:k]
}

func (js *JobServ) MergeAndDedupFinishers(a, b *Job) []string {
	h := make(map[string]bool)

	for i := range a.Finishaddr {
		h[a.Finishaddr[i]] = true
	}

	for i := range b.Finishaddr {
		h[b.Finishaddr[i]] = true
	}

	slice := make([]string, len(h))
	i := 0
	for k := range h {
		slice[i] = k
		i++
	}
	//TSPrintf("merge of %#v and %#v  ---->  %#v\n", a.Finishaddr, b.Finishaddr, slice)
	return slice
}

func (js *JobServ) ImmolateWorkers(immojob *Job) {
	if len(js.WaitingWorkers) != 0 {
		for _, worker := range js.WaitingWorkers {
			js.DispatchShutdownWorker(immojob, worker)
		}
		// clear the slice
		js.WaitingWorkers = js.WaitingWorkers[:0]
	}
}

func (js *JobServ) AssembleSnapShot(maxShow int) []string {
	if maxShow < 0 {
		// don't allow negative entries to crash us.
		maxShow = 10
	}
	out := make([]string, 0)
	out = append(out, fmt.Sprintf("runQlen=%d", len(js.RunQ)))
	out = append(out, fmt.Sprintf("waitingJobs=%d", len(js.WaitingJobs)))
	out = append(out, fmt.Sprintf("waitingWorkers=%d", len(js.WaitingWorkers)))
	out = append(out, fmt.Sprintf("jservPid=%d", js.Pid))
	out = append(out, fmt.Sprintf("finishedJobsCount=%d", js.FinishedJobsCount))
	out = append(out, fmt.Sprintf("droppedBadSigCount=%d", js.BadSgtCount))
	out = append(out, fmt.Sprintf("cancelledJobCount=%d", js.CancelledJobCount))
	out = append(out, fmt.Sprintf("nextJobId=%d", js.NextJobId))
	out = append(out, fmt.Sprintf("jservIP=%s", js.Cfg.JservIP))
	out = append(out, fmt.Sprintf("jservPort=%d", js.Cfg.JservPort))
	out = append(out, fmt.Sprintf("badNonceCount=%d", js.BadNonceCount))
	//out = append(out, "\n")

	k := int64(0)
	out = append(out, fmt.Sprintf("maxShow=%d", maxShow))

	shown := 0
	for _, v := range js.RunQ {
		elapSec := float64(time.Now().UnixNano()-v.Lastpingtm) / 1e9
		pingmsg := "Lastping: none."
		if elapSec < 1300000000 {
			pingmsg = fmt.Sprintf("Lastping: %.1f sec ago.", elapSec)
		}
		out = append(out, fmt.Sprintf("runq %06d   %s RunningJob[jid %d] = '%s %s'   on worker '%s'/pid:%d. %s   %s", k, runningTimeString(v), v.Id, v.Cmd, strings.Join(v.Args, " "), v.Workeraddr, v.Pid, pingmsg, stringFinishers(v)))
		k++
		shown++
		if shown > maxShow {
			break
		}
	}

	shown = 0
	for i, v := range js.WaitingJobs {
		out = append(out, fmt.Sprintf("wait %06d   WaitingJob[jid %d] = '%s %s'   submitted by '%s'.   %s", i, v.Id, v.Cmd, strings.Join(v.Args, " "), v.Submitaddr, stringFinishers(v)))
		shown++
		if shown > maxShow {
			break
		}
	}

	if false { // off for now

		for i, v := range js.WaitingWorkers {
			out = append(out, fmt.Sprintf("work %06d   WaitingWorker = '%s'", i, v.Workeraddr))
		}

		if Verbose {
			for i, v := range js.KnownJobHash {
				out = append(out, fmt.Sprintf("KnownJobHash key=%v    value.Msg=%s", i, v.Msg))
			}
		}
	}

	start := 0
	if len(js.FinishedRing) > maxShow {
		start = len(js.FinishedRing) - maxShow
	}

	// show the last FinishedRingMaxLen finished jobs.
	for _, v := range js.FinishedRing[start:] {
		finishLogLine := fmt.Sprintf("finished: [jid %d] %s. done: %s. cmd: '%s %s' finished on worker '%s'/pid:%d.  %s. Err: '%s'", v.Id, totalTimeString(v), NanoToTime(Ntm(v.Etm)), v.Cmd, v.Args, v.Workeraddr, v.Pid, stringFinishers(v), v.Err)
		out = append(out, finishLogLine)
	}

	out = append(out, fmt.Sprintf("--- goq security status---"))
	out = append(out, fmt.Sprintf("summary-bad-signature-msgs: %d", js.BadSgtCount))
	out = append(out, fmt.Sprintf("summary-bad-nonce-msg: %d", js.BadNonceCount))
	out = append(out, fmt.Sprintf("--- goq progress status ---"))
	out = append(out, fmt.Sprintf("summary-jobs-running: %d", len(js.RunQ)))
	out = append(out, fmt.Sprintf("summary-jobs-waiting: %d", len(js.WaitingJobs)))
	out = append(out, fmt.Sprintf("summary-known-jobs: %d", len(js.KnownJobHash)))
	out = append(out, fmt.Sprintf("summary-workers-waiting: %d", len(js.WaitingWorkers)))
	out = append(out, fmt.Sprintf("summary-cancelled-jobs: %d", js.CancelledJobCount))
	out = append(out, fmt.Sprintf("summary-jobs-finished: %d", js.FinishedJobsCount))
	out = append(out, fmt.Sprintf("--- goq end status at time: %s ---", time.Now()))

	return out
}

func stringFinishers(j *Job) string {
	if len(j.Finishaddr) == 0 {
		return ""
	}
	return fmt.Sprintf("finishers:%v", j.Finishaddr)
}

func NanoToTime(ntm Ntm) time.Time {
	return time.Unix(int64(ntm/1e9), int64(ntm%1e9))
}

func totalTimeString(j *Job) string {
	if j.Stm > 0 {
		dur := time.Duration(Ntm(j.Etm - j.Stm))
		return fmt.Sprintf("total-time: %s", dur)
	} else {
		return "total-time: unknown"
	}
}

func runningTimeString(j *Job) string {
	if j.Stm > 0 {
		dur := time.Since(NanoToTime(Ntm(j.Stm)))
		return fmt.Sprintf("runtime: %s", dur)
	} else {
		return "runtime: < 1 heartbeat"
	}
}

// SetAddrDestSocket: pull from cache, or make a new socket if not cached.
// should only be run on main Start() jobserv goroutine, because
// js.Who is a private resource.
func (js *JobServ) SetAddrDestSocket(destAddr string, job *Job) {
	dest, ok := js.Who[destAddr]
	if !ok {
		dest = NewPushCache(destAddr, destAddr, &js.Cfg)
		js.Who[destAddr] = dest
	}
	job.destinationSock = dest.DemandPushSock()
}

func (js *JobServ) DispatchJobToWorker(reqjob, job *Job) {
	job.Msg = schema.JOBMSG_DELEGATETOWORKER

	if job.Id == 0 {
		panic("job.Id must be non-zero by now")
	}

	job.Delegatetm = time.Now().UnixNano()
	js.RunQ[job.Id] = job

	if js.IsLocal {
		TSPrintf("**** [jobserver pid %d] dispatching job %d to local worker.\n", js.Pid, job.Id)

		js.ToWorker <- job
		return
	}

	job.Workeraddr = reqjob.Workeraddr

	js.SetAddrDestSocket(reqjob.Workeraddr, job)
	TSPrintf("**** [jobserver pid %d] dispatching job %d to worker '%s'.\n", js.Pid, job.Id, reqjob.Workeraddr)

	// try to send, give worker 30 seconds to grab it.
	if job.destinationSock != nil {
		go func(job Job) { // by value, so we can read without any race
			// we can send, go for it. But be on the lookout for timeout, i.e. when worker dies
			// before receiving their job. Then we should just re-queue it.
			_, err := sendZjob(job.destinationSock, &job, &js.Cfg)
			if err != nil {
				// for now assume deaf worker
				VPrintf("[pid %d] Got error back trying to dispatch job %d to worker '%s'. Incrementing "+
					"deaf worker count and resubmitting. err: %s\n", os.Getpid(), job.Id, job.Workeraddr, err)
				// arg: can't touch the jobserv when not in Start either: incrementing js.CountDeaf is a race!!
				// js.CountDeaf++

				// can't touch the job when not in Start() goroutine!!! So no: job.Msg = schema.JOBMSG_RESUBMITNOACK
				// have to let Start() notice that it is a resub, b/c Id and Workeraddr are already set.
				js.ReSubmit <- job.Id
			} else {
				VPrintf("[pid %d] dispatched job %d to worker '%s'\n", os.Getpid(), job.Id, job.Workeraddr)
			}
			return
		}(*job)
	}
}

func (js *JobServ) AddToFinishedRingbuffer(donejob *Job) {
	js.FinishedRing = append(js.FinishedRing, donejob)
	if len(js.FinishedRing) > js.FinishedRingMaxLen {
		js.FinishedRing = js.FinishedRing[1 : js.FinishedRingMaxLen+1]
	}
}

func (js *JobServ) TellFinishers(donejob *Job, msg schema.JobMsg) {
	if len(donejob.Finishaddr) == 0 {
		return
	}

	nnsocks := js.FinishersToNewSocket(donejob)

	for i := range nnsocks {
		job := NewJob()
		job.Msg = msg
		job.Aboutjid = donejob.Id
		job.Submitaddr = donejob.Finishaddr[i]

		sock := nnsocks[i]
		go func(job *Job, sock *nn.Socket, addr string) {
			_, err := sendZjob(sock, job, &js.Cfg)
			if err != nil {
				// timed-out
				TSPrintf("[pid %d] TellFinishers for job %d with msg %s to '%s' timed-out after %d msec.\n", os.Getpid(), job.Aboutjid, job.Msg, addr, js.Cfg.SendTimeoutMsec)
			} else {
				TSPrintf("[pid %d] TellFinishers for job %d with msg %s to '%s' succeeded.\n", os.Getpid(), job.Aboutjid, job.Msg, addr)
			}
			sock.Close()
			return
		}(job, sock, donejob.Finishaddr[i])
	}

}

// AckBack is used when Jserv doesn't expect a reply after this one (and we aren't issuing work).
// It puts toaddr into a new Job's Submitaddr and sends to toaddr.
func (js *JobServ) AckBack(reqjob *Job, toaddr string, msg schema.JobMsg, out []string) {
	if js.IsLocal {
		return
	}

	if toaddr == "" {
		panic(fmt.Sprintf("AckBack cannot use an empty address. reqjob: %#v", reqjob))
	}

	job := NewJob()
	job.Msg = msg
	if len(out) > 0 {
		job.Out = out
	}
	// tell acksubmit what number they got here.
	job.Aboutjid = reqjob.Id

	js.SetAddrDestSocket(toaddr, job)
	job.Submitaddr = toaddr
	job.Serveraddr = js.Addr
	job.Workeraddr = reqjob.Workeraddr

	// try to send, give badsig sender
	if job.destinationSock != nil {
		go func(job Job, addr string) {
			// doesn't matter if it times out, and it prob will.
			_, err := sendZjob(job.destinationSock, &job, &js.Cfg)
			if err != nil {
				// for now assume deaf worker
				TSPrintf("[pid %d] AckBack with msg %s to '%s' timed-out.\n", os.Getpid(), job.Msg, addr)
			}
			// close socket, to try not to leak it. ofh_test (open file handles test)
			// needs this next line or we'll see leaks.
			js.UnregSubmitWho <- &job
			return
		}(*job, toaddr)
	} else {
		TSPrintf("[pid %d] hmmm... jobserv could not find desination for final reply to addr: '%s'. Job: %#v\n", os.Getpid(), toaddr, job)
		// close socket, to try not to leak it.
		panic("should never get here now; the implementation of js.SetAddrDestSocket(toaddr, job) should now always find (or create on demand) the socket!")
		js.UnRegisterSubmitter(job)
	}
}

func (js *JobServ) DispatchShutdownWorker(immojob, workerready *Job) {
	j := NewJob()
	j.Msg = schema.JOBMSG_SHUTDOWNWORKER
	j.Serveraddr = js.Addr
	j.Submitaddr = immojob.Submitaddr
	j.Workeraddr = workerready.Workeraddr

	js.SetAddrDestSocket(j.Workeraddr, j)
	TSPrintf("**** [jobserver pid %d] sending 'shutdownworker' to worker '%s'.\n", js.Pid, j.Workeraddr)

	// try to send, give worker 30 seconds to grab it.
	if j.destinationSock == nil {
		panic("trying to immo, but j.DesinationSocket was nil?!?")
	}
	go func(job Job) { // by value, so we can read without any race
		_, err := sendZjob(job.destinationSock, &job, &js.Cfg)
		if err != nil {
			// ignore
		} else {
			TSPrintf("**** [jobserver pid %d] dispatched 'shutdownworker' to worker '%s'\n", os.Getpid(), job.Workeraddr)
		}
		return
	}(*j)
}

// for better debug output, when we drop jobs, guess which address we should report it from
func discrimAddr(j *Job) string {
	// if we only have one choice, then make it.
	if j.Workeraddr == "" && j.Submitaddr == "" {
		return ""
	}
	if j.Workeraddr == "" && j.Submitaddr != "" {
		return j.Submitaddr
	}
	if j.Workeraddr != "" && j.Submitaddr == "" {
		return j.Workeraddr
	}

	// otherwise use the Msg
	if j.Msg == schema.JOBMSG_REQUESTFORWORK {
		return j.Workeraddr
	}
	return j.Submitaddr
}

func (js *JobServ) ifDebugCid() string {
	if js.DebugMode {
		return fmt.Sprintf("cid:%s", js.Cfg.ClusterId)
	}
	return ""
}

func CloseChannelIfOpen(ch chan bool) {
	select {
	case <-ch:
	default:
		VPrintf("CloseChanelIfOpen is closing ch: %#v.\n", ch)
		close(ch)
	}
}

func (js *JobServ) ListenForJobs(cfg *Config) {
	go func() {
		defer close(js.ListenerDone)

		listenForJobsLoopCount := 0
		//lastPrintTm := time.Time{}
		for {
			listenForJobsLoopCount++
			//if listenForJobsLoopCount%1000 == 0 || time.Since(lastPrintTm) > time.Second*30 {
			VPrintf("at top of ListenForJobs loop, count = %d\n", listenForJobsLoopCount)
			//lastPrintTm = time.Now()
			//}

			select {
			case <-js.ListenerShutdown:
				VPrintf("\n**** [jobserver pid %d] JobServ::ListenForJobs() shutdown, closing ListenerDone.\n", os.Getpid())
				return
			default:
				//VPrintf("Listener did not find shutdown, checking for nanomsg.\n")
				// don't block here, instead try to get a nanomsg message
			}
			// after select:

			// recvZjob blocks, which is why we are in our own goroutine.
			// do we guard against address already bound errors here?
			job, err := recvZjob(js.Nnsock, cfg)
			if err != nil {
				continue // ignore timeouts after N seconds
			}
			VPrintf("ListenForJobs got * %s * job: %s. %s\n", job.Msg, job, js.ifDebugCid())

			// check signature
			if !JobSignatureOkay(job, cfg) {
				TSPrintf("JobSignature was false!!!\n")
				if js.DebugMode {
					TSPrintf("[pid %d] dropping job '%s' (Msg: %s) from '%s' whose signature did not verify.\n", os.Getpid(), job.Cmd, job.Msg, discrimAddr(job))
					if AesOff {
						TSPrintf("[pid %d] server's clusterid:%s.\n", os.Getpid(), js.Cfg.ClusterId)
					}
				}
				// debug run!
				res99 := JobSignatureOkay(job, cfg)
				fmt.Printf("debug with 2nd time res: %v\n", res99)
				// end debug
				js.SigMismatch <- job
				continue
			} else {
				VPrintf("JobSignature was OKAY.\n")
			}

			if !js.NoReplay.AddedOkay(job) {
				TSPrintf("NoReplay was  false !!!!\n")
				if js.DebugMode {
					TSPrintf("[pid %d] server dropping job '%s' (Msg: %s) from '%s': failed replay detection logic.\n", os.Getpid(), job.Cmd, job.Msg, discrimAddr(job))
				}
				js.BadNonce <- job
				continue
			} else {
				VPrintf("NoReplay was OKAY.\n")
			}

			if toonew, nsec := js.NoReplay.TooNew(job); toonew {
				if js.DebugMode {
					TSPrintf("[pid %d] server dropping job '%s' (Msg: %s) from '%s' whose sendtime was %d nsec into the future. Clocks not synced???.\n", os.Getpid(), job.Cmd, job.Msg, discrimAddr(job), nsec)
				}
				continue
			} else {
				VPrintf("TooNew was OKAY.\n")
			}

			VPrintf("**** 8888 got past all the sign/nonce/old checks. job.Msg = %s\n", job.Msg)

			switch job.Msg {
			case schema.JOBMSG_INITIALSUBMIT:
				select {
				case <-js.ListenerShutdown:
					return
				case js.Submit <- job:
				}
			case schema.JOBMSG_REQUESTFORWORK:
				select {
				case <-js.ListenerShutdown:
					return
				case js.WorkerReady <- job:
				}
			case schema.JOBMSG_DELEGATETOWORKER:
				if js.DebugMode {
					panic(fmt.Sprintf("[pid %d] server should never receive JOBMSG_DELEGATETOWORKER, only send it to worker. ", os.Getpid()))
				}
			case schema.JOBMSG_FINISHEDWORK:
				select {
				case <-js.ListenerShutdown:
					return
				case js.RunDone <- job:
				}
			case schema.JOBMSG_SHUTDOWNSERV:
				VPrintf("\nListener received on nanomsg JOBMSG_SHUTDOWNSERV. Sending die on js.Ctrl\n")
				select {
				// <-js.ListenerShutdown protects against deadlock in case JobServ::Start()
				// initiates shutdown (i.e. somebody else sends js.Ctrl <- die)
				// before we can send js.Ctrl <- die.
				case <-js.ListenerShutdown:
					VPrintf("\nListener received on js.ListenerShutdown.\n")
					return

				case js.Ctrl <- die:
					VPrintf("\nListener sent die on js.Ctrl\n")
				}
				VPrintf("\nListener: at end of case JOBMSG_SHUTDOWNSERV\n")

			case schema.JOBMSG_TAKESNAPSHOT:
				VPrintf("\nListener: in case JOBMSG_TAKESNAPSHOT\n")
				select {
				case <-js.ListenerShutdown:
					return
				case js.SnapRequest <- job:
					VPrintf("\nListener: sent job on js.SnapRequest\n")
				}
			case schema.JOBMSG_OBSERVEJOBFINISH:
				select {
				case <-js.ListenerShutdown:
					return
				case js.ObserveFinish <- job:
				}
			case schema.JOBMSG_CANCELSUBMIT:
				select {
				case <-js.ListenerShutdown:
					return
				case js.Cancel <- job:
				}
			case schema.JOBMSG_IMMOLATEAWORKERS:
				select {
				case <-js.ListenerShutdown:
					return
				case js.ImmoReq <- job:
				}
			case schema.JOBMSG_ACKSHUTDOWNWORKER:
				select {
				case <-js.ListenerShutdown:
					return
				case js.WorkerDead <- job:
				}
			case schema.JOBMSG_ACKPINGWORKER:
				select {
				case <-js.ListenerShutdown:
					return
				case js.WorkerAckPing <- job:
				}

			case schema.JOBMSG_ACKCANCELWIP:
				TSPrintf("**** [jobserver pid %d] got ack of cancelled for job %d from worker '%s'; job.Cancelled: %v.\n", os.Getpid(), job.Id, job.Workeraddr, job.Cancelled)
				select {
				case <-js.ListenerShutdown:

					return
				case js.RunDone <- job:
				}

			default:
				TSPrintf("Listener: unrecognized JobMsg: '%v' in job: %s\n", job.Msg, job)
			}
			VPrintf("\nListener at bottom of for{} loop\n")
		}
	}()
}

func sendZjob(nnzbus *nn.Socket, j *Job, cfg *Config) ([]byte, error) {
	StampJob(j) // also in NewJob
	return sendZjobWithoutStamping(nnzbus, j, cfg)
}

func sendZjobWithoutStamping(nnzbus *nn.Socket, j *Job, cfg *Config) ([]byte, error) {

	// sanity check
	if j.Submitaddr == "" && j.Serveraddr == "" && j.Workeraddr == "" {
		//if cfg.DebugMode {
		panic(fmt.Sprintf("job cannot have all empty addresses: %s", j))
		//} // else don't crash the process on bad job send
	}

	// Create Zjob and Write to nnzbus.
	SignJob(j, cfg)
	buf, _ := JobToCapnp(j)

	cy := []byte{}
	if AesOff {
		cy = buf.Bytes()
	} else {
		cy = cfg.Cypher.Encrypt(buf.Bytes())
	}
	_, err := nnzbus.Send(cy, 0)
	return cy, err

}

func recvZjob(nnzbus *nn.Socket, cfg *Config) (job *Job, err error) {

	// harden against cross-cluster communication
	defer func() {
		if recover() != nil {
			job = nil
			err = fmt.Errorf("unknown recovered error on receive")
		}
	}()

	// Read job submitted to the server
	var myMsg []byte
	myMsg, err = nnzbus.Recv(0)
	if err != nil {
		return nil, err
	}

	plain := []byte{}
	if AesOff {
		plain = myMsg
	} else {
		plain = cfg.Cypher.Decrypt(myMsg)
	}

	buf := bytes.NewBuffer(plain)
	job = CapnpToJob(buf)
	return job, nil
}

func MakeTestJob() *Job {
	job := NewJob()

	job.Cmd = "bin/good.sh"
	job.Args = []string{}
	job.Out = []string{}
	job.Host = "testhost"
	job.Stm = 23
	job.Etm = 27
	job.Elapsec = 4
	job.Status = "okay"
	job.Subtime = 99
	job.Pid = 1011
	job.Dir = ""

	return job
}

func MakeActualJob(args []string, cfg *Config) *Job {
	if len(args) == 0 {
		panic("args len must be > 0")
	}
	pwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	job := NewJob()
	job.Dir = pwd
	job.Cmd = args[0]
	if AesOff {
		job.Out = append(job.Out, "clusterid:"+cfg.ClusterId)
	}
	if len(args) > 1 {
		job.Args = args[1:]
	}
	return job
}

func MkPullNN(addr string, cfg *Config, infWait bool) (*nn.Socket, error) {
	pull1, err := nn.NewSocket(nn.AF_SP, nn.PULL)

	if err != nil {
		panic(err)
		//return nil, err
	}

	if bound, err := IsAlreadyBound(addr); bound {
		panic(fmt.Errorf("problem in MkpullNN: address (%s) is already bound: %s", addr, err))
	}

	if !infWait {
		if cfg != nil {
			if cfg.SendTimeoutMsec > 0 {
				err = pull1.SetRecvTimeout(time.Duration(cfg.RecvTimeoutMsec) * time.Millisecond)
				if err != nil {
					panic(err)
				}
			}
		}
	}

	err = CheckedBind(addr, pull1)

	if err != nil {
		TSPrintf("could not bind addr '%s': %v", addr, err)
		panic(err)
		//return nil, err
	}
	//VPrintf("[pid %d] gozbus: pull socket made at '%s'.\n", os.Getpid(), addr)

	return pull1, nil
}

func MkPushNN(addr string, cfg *Config, infWait bool) (*nn.Socket, error) {
	push1, err := nn.NewSocket(nn.AF_SP, nn.PUSH)
	if err != nil {
		return nil, err
	}

	if !infWait {
		if cfg != nil {
			if cfg.SendTimeoutMsec > 0 {
				err = push1.SetSendTimeout(time.Duration(cfg.SendTimeoutMsec) * time.Millisecond)
				if err != nil {
					panic(err)
				}
			}
		}
	}
	_, err = push1.Connect(addr)
	if err != nil {
		VPrintf("could not bind addr '%s': %v", addr, err)
		return nil, err
	}
	//VPrintf("[pid %d] gozbus: push socket made at '%s'.\n", os.Getpid(), addr)

	return push1, nil
}

// barebones, just get it done.
func SendShutdown(cfg *Config) {
	sub, err := NewSubmitter(GenAddress(), cfg, false)
	if err != nil {
		panic(err)
	}
	sub.SubmitShutdownJob()
}

func CheckedBind(addr string, pull1 *nn.Socket) error {

	var err error
	var isbound bool
	isbound, err = IsAlreadyBound(addr)
	if isbound {
		return err
	} else {
		_, err = pull1.Bind(addr)
		return err
	}
}

func IsAlreadyBound(addr string) (bool, error) {

	stripped, err := StripNanomsgAddressPrefix(addr)
	if err != nil {
		panic(err)
	}

	ln, err := net.Listen("tcp", stripped)
	if err != nil {
		return true, err
	}
	ln.Close()
	return false, nil
}

func SubmitGetServerSnapshot(cfg *Config) ([]string, error) {
	sub, err := NewSubmitter(GenAddress(), cfg, false)
	if err != nil {
		panic(err)
	}

	j := NewJob()
	j.Msg = schema.JOBMSG_TAKESNAPSHOT

	return sub.SubmitSnapJob(10)
}

func WaitUntilAddrAvailable(addr string) int {
	sleeps := 0
	for {
		var isbound bool
		isbound, _ = IsAlreadyBound(addr)
		if isbound {
			time.Sleep(10 * time.Millisecond)
			sleeps++
		} else {
			fmt.Printf("\n took %d 10msec sleeps for address '%s' to become available.\n", sleeps, addr)
			return sleeps
		}
	}
	//return sleeps
}

// do (isolated here for testing) the startup of the server
func ServerInit(cfg *Config) {
	GenNewCreds(cfg)
}

func MoveToDirOrPanic(newdir string) {
	pwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	if pwd != newdir {
		err = os.Chdir(newdir)
		if err != nil {
			panic(err)
		}
	}
}

// get timestamp for logging purposes
func ts() string {
	return time.Now().Format("2006-01-02 15:04:05.999 -0700 MST")
}

// time-stamped printf
func TSPrintf(format string, a ...interface{}) {
	fmt.Printf("%s ", ts())
	fmt.Printf(format, a...)
}

// if we have an old state, file, load it and blow it away
// so we don't have to deal with the old format in the future.
func (js *JobServ) checkForOldStateFile(fn string) {
	file, err := os.Open(fn)
	if err != nil {
		// ignore errors here, just a backwards-compatiblity routine.
		return
	}

	_, err = fmt.Fscanf(file, "NextJobId=%d\n", &js.NextJobId)
	if err == nil {
		file.Close()
		os.Remove(fn) // only remove if we actually read the *old* format
		return
	} else {
		file.Close()
	}
}
