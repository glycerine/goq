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
	"net/http"
	"os"
	"os/exec"
	"runtime/pprof"
	"strings"
	"sync"
	"time"

	"github.com/glycerine/cryrand"
	schema "github.com/glycerine/goq/schema"
	rpc "github.com/glycerine/rpc25519"
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
var WebDebug bool

// for debugging signature issues
var ShowSig bool

var AesOff bool = true // TLS-v1.3 over QUIC or TCP suffices, so turn off extra encryption.

// number of finished job records to retain in a ring buffer. Oldest are discarded when full.
var DefaultFinishedRingMaxLen = 1000

func init() {
	rand.Seed(time.Now().UnixNano() + int64(GetExternalIPAsInt()) + CryptoRandInt64())
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

	Addr string   // even port number (mnemonic: stdout is 0/even)
	nc   net.Conn // from => pull
	cfg  *Config
}

func NewPushCache(name, addr string, nc net.Conn, cfg *Config) *PushCache {
	p := &PushCache{
		Name: name,
		Addr: addr,
		cfg:  cfg,
		nc:   nc,
	}
	SocketCountPushCache++
	return p

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
	    }		*/

}

// re-create socket on-demand. Used because we may close
// sockets to keep from using too many.
func (p *PushCache) DemandPushSock() net.Conn {
	return p.nc
}

func (p *PushCache) Close() {
	//vv("PushCache.Close() called")
	// SetLinger is essential or else cancel and
	// immo tests which need submit-replies will fail.
	// Another system sets linger to be 10 seconds anyway.
	if tc, ok := p.nc.(*net.TCPConn); ok {
		//tc.SetKeepAlive(true)
		//tc.SetKeepAlivePeriod(3 * time.Minute)
		tc.SetLinger(10)
	}

	if p != nil && p.nc != nil {
		//vv("PushCache close is closing net.Conn remote='%v'", p.nc.RemoteAddr())
		p.nc.Close()
		p.nc = nil
	}
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

	HomeOnSubmitter string // so the worker can figure out the same path relative to local home.

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

	Runinshell bool
	MaxShow    int64
	CmdOpts    uint64

	// not serialized, just used
	// for routing
	destinationSock net.Conn

	// not serialized
	nc      net.Conn // set by the receiver, so we can talk who sent this.
	replyCh chan *rpc.Message

	callid    string // for rpc25519 header CallID tracking
	callSeqno uint64 // attempt to reply with +1, not sure we always can.
}

func (j *Job) String() string {
	if j == nil {
		return "&Job{nil}"
	} else {
		return fmt.Sprintf("&Job{Id:%d, Msg:%s, Aboutjid:%d, Cmd:%s, Args:%#v, Out:%#v, Submitaddr:%s, Serveraddr:%s, Workeraddr:%s, Sendtime:%v, Sendernonce:%x}", j.Id, j.Msg, j.Aboutjid, j.Cmd, j.Args, j.Out, j.Submitaddr, j.Serveraddr, j.Workeraddr, time.Unix(j.Sendtime/1e9, j.Sendtime%1e9).UTC().Format(time.RFC3339Nano), j.Sendernonce)
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

// called from NewJob, can't call in SignJob() because that
// is used for verification too.
func StampJob(j *Job) {
	j.Sendtime = int64(time.Now().UnixNano())
}

// Clone j and set Args to empty string slice, duplicating the other []string fields
// and stamping the job.
func (j *Job) CloneWithEmptyArgs() (r *Job) {
	cp := *j
	r = &cp
	cp.Args = []string{}

	cp.Out = make([]string, len(j.Out))
	copy(cp.Out, j.Out)
	cp.Env = make([]string, len(j.Env))
	copy(cp.Env, j.Env)
	cp.Finishaddr = make([]string, len(j.Finishaddr))
	copy(cp.Finishaddr, j.Finishaddr)
	StampJob(r)
	return
}

// only JobServ assigns Ids, submitters and workers just leave Id == 0.
func (js *JobServ) NewJobId() int64 {
	id := js.NextJobId
	js.NextJobId++
	return id
}

func (js *JobServ) RegisterWho(j *Job) {
	//vv("RegisterWho called on job = '%s'", j) // not seen in shutdown
	// add addresses and sockets if not created already
	if j.Workeraddr != "" {
		if _, ok := js.Who[j.Workeraddr]; !ok {
			js.Who[j.Workeraddr] = NewPushCache(j.Workeraddr, j.Workeraddr, j.nc, &js.Cfg)
		}
	}

	if j.Submitaddr != "" {
		if _, ok := js.Who[j.Submitaddr]; !ok {
			js.Who[j.Submitaddr] = NewPushCache(j.Submitaddr, j.Submitaddr, j.nc, &js.Cfg)
		}
	}

}

func (js *JobServ) UnRegisterWho(j *Job) {
	// add addresses and sockets if not created already
	if j.Workeraddr != "" {
		if c, found := js.Who[j.Workeraddr]; found {
			_ = c
			c.Close()
			delete(js.Who, j.Workeraddr)
		}
	}

	if j.Submitaddr != "" {
		if c, found := js.Who[j.Submitaddr]; found {
			_ = c
			c.Close()
			delete(js.Who, j.Submitaddr)
		}
	}

}

func (js *JobServ) UnRegisterSubmitter(j *Job) {
	if j.Submitaddr != "" {
		if c, found := js.Who[j.Submitaddr]; found {
			_ = c
			c.Close()
			delete(js.Who, j.Submitaddr)
		}
	}
}

// assume these won't be long running finishers, so don't cache them in Who
func (js *JobServ) FinishersToNewSocket(j *Job) []*Nexus {

	res := make([]*Nexus, 0)
	for i := range j.Finishaddr {
		addr := j.Finishaddr[i]
		if addr == "" {
			panic("addr in Finishers should never be empty")
		}
		nc, err := js.CBM.get(addr)
		if err != nil {
			res = append(res, nc)
		}
	}
	return res
}

func (js *JobServ) CloseRegistry() {
	//vv("CloseRegistry has %v Who", len(js.Who))
	for _, pp := range js.Who {
		if pp.nc != nil {
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
	if js.CBM != nil {
		js.CBM.Close()
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
			AlwaysPrintf("[pid %d] job server error: stateToDisk() could not find file '%s': %s\n", os.Getpid(), fn, err)
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

	//vv("[pid %d] stateToDisk() done: wrote state (js.NextJobId=%d) to '%s'\n", os.Getpid(), js.NextJobId, fn)
}

func (js *JobServ) diskToState() {
	fn := js.stateFilename()

	js.checkForOldStateFile(fn)

	file, err := os.Open(fn)
	if err != nil {
		errs := err.Error()
		if strings.HasSuffix(errs, "no such file or directory") ||
			strings.HasSuffix(errs, "cannot find the file specified.") {
			VPrintf("[pid %d] diskToState() done: no state file found in '%s'\n", os.Getpid(), fn)
			return
		} else {
			panic(err)
		}
	}
	defer file.Close()
	js.SetStateFromCapnp(file, fn)

	//vv("[pid %d] diskToState() done: read state (js.NextJobId=%d) from '%s'\n", os.Getpid(), js.NextJobId, fn)
}

type Address string

// JobServ represents the single central job server.
type JobServ struct {
	Name string

	Nnsock *rpc.Server // receive on
	Addr   string

	Submit      chan *Job  // submitter sends on, JobServ receives on.
	ReSubmit    chan int64 // dispatch go-routine sends on when worker is unreachable, JobServ receives on.
	WorkerReady chan *Job  // worker sends on, JobServ receives on.
	ToWorker    chan *Job  // worker receives on, JobServ sends on.
	RunDone     chan *Job  // worker sends on, JobServ receives on.
	//SigMismatch     chan *Job  // Listener tells Start about bad signatures.
	SnapRequest     chan *Job // worker requests state snapshot from JobServ.
	ObserveFinish   chan *Job // submitter sends on, Jobserv recieves on; when a submitter wants to wait for another job to be done.
	NotifyFinishers chan *Job // submitter receives on, jobserv dispatches a notification message for each finish observer
	Cancel          chan *Job // submitter sends on, to request job cancellation.
	ImmoReq         chan *Job // submitter sends on, to requst all workers die.
	WorkerDead      chan *Job // worker tells server just before terminating self.
	WorkerAckPing   chan *Job // worker replies to server that it is still alive. If working on job then Aboutjid is set.

	UnregSubmitWho chan *Job // JobServ internal use: unregister submitter only.
	FromRpcServer  chan *Job // JobServ internal use: from rpc to original logic.

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

	CountDeaf int
	PrevDeaf  int
	//BadSgtCount       int64
	FinishedJobsCount int64
	CancelledJobCount int64

	// set Cfg *once*, before any goroutines start, then
	// treat it as immutable and never changing.
	Cfg       Config
	DebugMode bool // show badsig messages if true
	IsLocal   bool

	FinishedRing       []*Job
	FinishedRingMaxLen int
	Web                *WebServer

	CBM *ServerCallbackMgr
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
	if cfg.UseQUIC {
		addr = "udp://" + cfg.JservAddrNoProto()
	}

	if cfg.Cypher == nil {
		var key *CypherKey
		key, err = OpenExistingOrCreateNewKey(cfg)
		if err != nil || key == nil {
			panic(fmt.Sprintf("could not open or create encryption key: %s", err))
		}
		cfg.Cypher = key
	}

	MoveToDirOrPanic(cfg.Home)

	var pullsock *rpc.Server
	var remote bool
	var cbm *ServerCallbackMgr
	if cfg.JservIP != "" {
		remote = true
		cbm, err = NewServerCallbackMgr(addr, cfg)
		panicOn(err)
		pullsock = cbm.Srv
		AlwaysPrintf("[pid %d] JobServer bound endpoints addr: '%s'\n", os.Getpid(), addr)
	} else {
		AlwaysPrintf("cfg.JservIP is empty, not starting NewServerCallbackMgr")
	}

	js := &JobServ{
		CBM:    cbm,
		Name:   fmt.Sprintf("jobserver.pid.%d", os.Getpid()),
		Addr:   addr,
		Nnsock: pullsock,
		RunQ:   make(map[int64]*Job),

		// KnownJobHash tracks actual (numbered) jobs, not worker-ready/request-to-object jake-job requests.
		KnownJobHash: make(map[int64]*Job), // for fast lookup, jobs are either on WaitingJobs slice, or in RunQ table.

		// avoid the same worker doublying up and filling the worker queue
		//  thus we avoid lots of spurious dispatch attempts.
		DedupWorkerHash: make(map[string]bool),

		WaitingJobs: make([]*Job, 0),
		Submit:      make(chan *Job),
		ReSubmit:    make(chan int64),
		WorkerReady: make(chan *Job),
		ToWorker:    make(chan *Job),
		RunDone:     make(chan *Job),
		//SigMismatch:   make(chan *Job),
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

		FinishedRingMaxLen: DefaultFinishedRingMaxLen,
		FinishedRing:       make([]*Job, 0, DefaultFinishedRingMaxLen),

		UnregSubmitWho: make(chan *Job),
		FromRpcServer:  make(chan *Job),
	}

	// don't crash on local tests where cbm is nil.
	if cfg.JservIP != "" {
		cbm.jserv = js
		cbm.start()
	}

	VPrintf("ListenerShutdown channel created in ctor.\n")
	js.diskToState()

	if WebDebug {
		js.Web = NewWebServer()

		//startProfilingCPU("cpu")
		//startProfilingMemory("memprof", time.Minute)
	}
	js.Start()
	if remote {
		//VPrintf("remote, server starting ListenForJobs() goroutine.\n")
		AlwaysPrintf("**** [jobserver pid %d] listening for jobs on '%s', output to '%s'. GOQ_HOME is '%s'.\n", js.Pid, js.Addr, js.Odir, js.Cfg.Home)
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
		err := os.Mkdir(dirname, 0700)
		if err != nil {
			return err
		}
	}
	return nil
}

func (js *JobServ) WriteJobOutputToDisk(donejob *Job) {
	//vv("WriteJobOutputToDisk() called for Job: %s\n", donejob)

	var err error
	local := false
	var fn string
	var odir string

	// the directories on the submit host (where we start) may not match those on
	// the server host where (where we finish), but it would be a common situation
	// to have them be on the same host, hence we try to write back to donejob.Dir
	// if at all possible.
	if !DirExists(donejob.Dir) {
		// try the windows "Z:\path" -> "/cygdrive/z/path" change.
		djd2 := replaceWindrive(donejob.Dir)
		if djd2 != donejob.Dir {
			// we changed, try again. might be on a mac that has
			// /cygdrive symlink for compatibility.
			if DirExists(djd2) {
				donejob.Dir = djd2
			}
		}
	}

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
			AlwaysPrintf("[pid %d] server job-done badness: could not make output directory '%s' for job %d output.\n", js.Pid, odir, donejob.Id)
			return
		}
		fn = fmt.Sprintf("%s/%s/out.%05d", js.Cfg.Home, js.Odir, donejob.Id)
		AlwaysPrintf("[pid %d] drat, could not get to the submit-directory for job %d. Output to '%s' instead.\n", js.Pid, donejob.Id, fn)
	}
	// invar: fn is set.

	// try to avoid all those empty o/out.2381 files with just a newline in them.
	n := len(donejob.Out)
	if n > 1 || (n == 1 && len(donejob.Out[0]) > 0) {

		// append if already existing file: so we can have incremental updates.
		var file *os.File
		if FileExists(fn) {
			file, err = os.OpenFile(fn, os.O_RDWR|os.O_APPEND, 0600)
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
		AlwaysPrintf("[pid %d] jobserver wrote output for job %d to file '%s'\n", js.Pid, donejob.Id, fn)
	}
}

// return a length 2 slice, the command and the remainder as the second string.
func twoSplitOnFirstWhitespace(s string) []string {
	n := len(s)
	if n == 0 {
		return []string{"", ""}
	}
	r := []rune(s)
	for i := range r {
		if r[i] == ' ' || r[i] == '\t' {
			return []string{string(r[:i]), string(r[i:])}
		}
	}
	// no whitespace found
	return []string{s, ""}
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

				if newjob.Msg == schema.JOBMSG_SHUTDOWNSERV {
					VPrintf("JobServ got JOBMSG_SHUTDOWNSERV from Submit channel.\n")
					go func() { js.Ctrl <- die }()
					continue
				}

				var newJobIDs []int64
				if strings.HasPrefix(newjob.Cmd, "lines@") {
					// for efficiency, this is actually a submission of
					// multiple jobs at once, each specified in the newjob.Args
					for _, jobline := range newjob.Args {
						jobargs := twoSplitOnFirstWhitespace(jobline)

						if jobargs[0] == "" {
							// skip empty lines
							continue
						}
						// clone and give each its own Id
						job1 := newjob.CloneWithEmptyArgs()
						job1.Cmd = jobargs[0]

						// everything else is left in the 2nd string, possibly empty
						job1.Args = []string{jobargs[1]}
						curId := js.NewJobId()
						job1.Id = curId
						newjob.Id = curId // the ackback tells submitter the largest job ID.
						js.KnownJobHash[curId] = job1
						newJobIDs = append(newJobIDs, curId)

						// open and cache any sockets we will need.
						js.RegisterWho(job1)
						js.WaitingJobs = append(js.WaitingJobs, job1)
					}
					AlwaysPrintf("**** [jobserver pid %d] got %v new jobs from '%v'; jobIDs [%v : %v] submitted.\n", js.Pid, len(newJobIDs), newjob.Cmd, newJobIDs[0], newJobIDs[len(newJobIDs)-1])
				} else {

					// just the one job, not @lines
					curId := js.NewJobId()
					newJobIDs = append(newJobIDs, curId)
					newjob.Id = curId
					js.KnownJobHash[curId] = newjob

					// open and cache any sockets we will need.
					js.RegisterWho(newjob)
					AlwaysPrintf("**** [jobserver pid %d] got job %d submission. Will run '%s'.\n", js.Pid, newjob.Id, newjob.Cmd)
					js.WaitingJobs = append(js.WaitingJobs, newjob)
				}

				js.Dispatch()
				// we just dispatched, now reply to submitter with ack (in an async goroutine); they don't need to
				// wait for it, but often they will want confirmation/the jobid.
				//vv("got job, calling js.AckBack() with schema.JOBMSG_ACKSUBMIT.")
				js.AckBack(newjob, newjob.Submitaddr, schema.JOBMSG_ACKSUBMIT, []string{fmt.Sprintf("submitted %v job(s) [%v:%v]", len(newJobIDs), newJobIDs[0], newJobIDs[len(newJobIDs)-1])})

			case resubId := <-js.ReSubmit:
				//vv("  === event loop case === (%d) JobServ got resub for jobid %d\n", loopcount, resubId)
				js.CountDeaf++
				resubJob, ok := js.RunQ[resubId]
				if !ok {
					// maybe it was cancelled in the meantime. don't panic.
					AlwaysPrintf("**** [jobserver pid %d] got re-submit of job %d that is now not on our RunQ, so dropping it without re-queuing.\n", js.Pid, resubId)
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
					AlwaysPrintf("**** [jobserver pid %d] Problem? got ping back from worker at '%s' running job %d that was not in our RunQ???\n", js.Pid, ackping.Workeraddr, ackping.Aboutjid)
				}

			case reqjob := <-js.WorkerReady:
				//vv("  === event loop case === (%d) JobServ got request for work from WorkerReady channel: %s\n", loopcount, reqjob)
				if reqjob.nc != nil {
					vv("WorkerReady from worker remote addr: '%s'", netConnRemoteAddrAsKey(reqjob.nc))
				}
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
					//fmt.Sprintf("ignoring donejob %d for job(%s) since js.RunQ does not show it active.\n", donejob.Id, donejob)
					continue
				}
				kjh, ok := js.KnownJobHash[donejob.Id]
				if !ok {
					// just ignore, probably a re-issued job that finally woke up and came back.
					if js.DebugMode {
						AlwaysPrintf("\n jobserv debugmode: got donejob %d for job(%s) from js.RunDone channel, but it was not in our js.KnownJobHash: %#v\n", donejob.Id, donejob, js.KnownJobHash)
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
				AlwaysPrintf("**** [jobserver pid %d] worker finished job %d, removing from the RunQ\n", js.Pid, donejob.Id)
				js.WriteJobOutputToDisk(donejob)
				// tell waiting worker we got it.
				//js.returnToWaitingCallerWith(donejob, schema.JOBMSG_JOBFINISHEDNOTICE)
				// try this:
				js.AckBack(donejob, donejob.Workeraddr, schema.JOBMSG_JOBFINISHEDNOTICE, nil)

				js.TellFinishers(donejob, schema.JOBMSG_JOBFINISHEDNOTICE)
				js.AddToFinishedRingbuffer(donejob)

			case cmd := <-js.Ctrl:
				VPrintf("  === event loop case zowza === (%d)  JobServ got control cmd: %v\n", loopcount, cmd)
				switch cmd {
				case die:
					AlwaysPrintf("**** [jobserver pid %d] jobserver got 'die' cmd on js.Ctrl. about to call js.Shutdown().\n", js.Pid)
					js.Shutdown()
					AlwaysPrintf("**** [jobserver pid %d] jobserver got 'die' cmd on js.Ctrl. js.Shutdown() done. Exiting.\n", js.Pid)
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

				/*			case badsigjob := <-js.SigMismatch:
							//nothing doing with this job, it had a bad signature
							js.BadSgtCount++
							// ignore badsig packets; to prevent a bad worker from infinite looping/DOS-ing us.
							if js.DebugMode {
								addr := badsigjob.Submitaddr
								if addr == "" {
									addr = badsigjob.Workeraddr
								}

								AlwaysPrintf("**** [jobserver pid %d] DebugMode: actively rejecting badsig message from '%s'.\n", js.Pid, addr)
								if addr != "" {
									js.RegisterWho(badsigjob)
									js.AckBack(badsigjob, addr, schema.JOBMSG_REJECTBADSIG, []string{})
								}
							}
				*/
			case snapreq := <-js.SnapRequest:
				VPrintf("\nStart: got snapreq: '%#v'\n", snapreq)
				js.RegisterWho(snapreq)

				shot := js.AssembleSnapShot(int(snapreq.MaxShow))
				js.AckBack(snapreq, snapreq.Submitaddr, schema.JOBMSG_ACKTAKESNAPSHOT, shot)
				//VPrintf("\nHandling snapreq: done with AckBack; shot was: '%#v'\n", shot)

			case canreq := <-js.Cancel:
				//vv("got canreq in loop: '%s'", canreq)
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
					//vv("**** [jobserver pid %d] server sent 'cancelwip' for job %d to '%s'.\n", js.Pid, canid, j.Workeraddr)
				}

				// if we don't  remove from RunQ and KJH immediately, it looks wierd to the user.
				delete(js.RunQ, canid)
				delete(js.KnownJobHash, canid)
				js.RemoveFromWaitingJobs(j)
				// reducdnat with AckBack below
				//js.returnToWaitingCallerWith(canreq, schema.JOBMSG_ACKCANCELSUBMIT)
				js.TellFinishers(j, schema.JOBMSG_CANCELSUBMIT)

				// don't tell finishers twice
				j.Finishaddr = j.Finishaddr[:0]

				js.AckBack(canreq, canreq.Submitaddr, schema.JOBMSG_ACKCANCELSUBMIT, []string{})
			unreg:
				//js.UnRegisterWho(canreq) // ackback should take care of this now, right?
				AlwaysPrintf("**** [jobserver pid %d] server cancelled job %d per request of '%s'.\n", js.Pid, canid, canreq.Submitaddr)

			case obsreq := <-js.ObserveFinish:
				if obsreq.Submitaddr == "" {
					// ignore bad requests
					if js.DebugMode {
						AlwaysPrintf("**** [jobserver pid %d] DebugMode: got Observe Request with bad Submitaddr: %s.\n", js.Pid, obsreq)
					}
					continue
				}
				if j, ok := js.KnownJobHash[obsreq.Aboutjid]; ok {
					// still in progress, so we add this requester to the Finishaddr list
					js.RegisterWho(obsreq)
					AlwaysPrintf("**** [jobserver pid %d] noting request to get notice about the finish of job %d from '%s'.\n", js.Pid, obsreq.Aboutjid, obsreq.Submitaddr)
					j.Finishaddr = append(j.Finishaddr, obsreq.Submitaddr)
					js.AckBack(obsreq, obsreq.Submitaddr, schema.JOBMSG_OBSERVEJOBACK, []string{})

				} else {
					// probably already finished
					AlwaysPrintf("**** [jobserver pid %d] impossible request for finish-notify on job %d (unknown job) from '%s'. Sending JOBMSG_JOBNOTKNOWN\n", js.Pid, obsreq.Aboutjid, obsreq.Submitaddr)
					fakedonejob := NewJob()
					fakedonejob.Id = obsreq.Aboutjid
					fakedonejob.Finishaddr = []string{obsreq.Submitaddr}
					js.TellFinishers(fakedonejob, schema.JOBMSG_JOBNOTKNOWN)
				}
			case <-heartbeat:
				//vv("heartbeat happening...")
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
	AlwaysPrintf("**** [jobserver pid %d] got re-submit of job %d that was dispatched to '%s'. Trying again.\n", js.Pid, resubJob.Id, resubJob.Workeraddr)

	resubJob.Workeraddr = ""
	resubJob.Lastpingtm = 0
	resubJob.Unansweredping = 0
	delete(js.RunQ, resubJob.Id)
	// prepend, so the job doesn't loose its place in line. *try* to FIFO as much as possible.
	js.WaitingJobs = append([]*Job{resubJob}, js.WaitingJobs...)
	js.Dispatch()
}

func (js *JobServ) PingJobRunningWorkers() {
	//vv("top of PingJobRunningWorkers")
	now := Ntm(time.Now().UnixNano())
	hb := js.Cfg.Heartbeat // seconds
	timeout := Tmsec2Ntm(hb)
	twotimeouts := 2 * timeout

	for _, j := range js.RunQ {
		//vv("going through the RunQ, here is j='%#v'", j)
		elap := now - MaxNtm(Ntm(j.Delegatetm), Ntm(j.Lastpingtm))
		if elap < timeout {
			continue
		}
		if j.Unansweredping == 0 {
			//vv("**** [jobserver pid %d] (elapsed = %.1f sec) heartbeat pinging worker '%s' with running job %d.\n", js.Pid, float64(elap)/1e9, j.Workeraddr, j.Id)
			j.Aboutjid = j.Id
			j.Unansweredping = 1
			js.AckBack(j, j.Workeraddr, schema.JOBMSG_PINGWORKER, []string{})
			continue
		}

		if elap > twotimeouts {
			// its been at least two timeouts since job was last heard from
			js.CountDeaf++
			js.DeadWorkerResubJob(j, float64(elap)/1e9)
			continue
		}

	}
}

func (js *JobServ) DeadWorkerResubJob(j *Job, elapSec float64) {
	AlwaysPrintf("**** [jobserver pid %d] sees dead worker for job %d (no ping reply after %.1f sec). Resubmitting.\n", js.Pid, j.Id, elapSec)
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
	//AlwaysPrintf("merge of %#v and %#v  ---->  %#v\n", a.Finishaddr, b.Finishaddr, slice)
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
	//	out = append(out, fmt.Sprintf("droppedBadSigCount=%d", js.BadSgtCount))
	out = append(out, fmt.Sprintf("cancelledJobCount=%d", js.CancelledJobCount))
	out = append(out, fmt.Sprintf("nextJobId=%d", js.NextJobId))
	out = append(out, fmt.Sprintf("jservIP=%s", js.Cfg.JservIP))
	out = append(out, fmt.Sprintf("jservPort=%d", js.Cfg.JservPort))
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
		out = append(out, fmt.Sprintf("runq %06d   %s RunningJob[jid %d] = '%s %s'   on worker '%s'/pid:%d. %s  dir:'%v'  %s", k, runningTimeString(v), v.Id, v.Cmd, strings.Join(v.Args, " "), v.Workeraddr, v.Pid, pingmsg, v.Dir, stringFinishers(v)))
		k++
		shown++
		if shown > maxShow {
			break
		}
	}

	shown = 0
	for i, v := range js.WaitingJobs {
		out = append(out, fmt.Sprintf("wait %06d   WaitingJob[jid %d] = '%s %s'   submitted by '%s'.  dir:'%v'   %s", i, v.Id, v.Cmd, strings.Join(v.Args, " "), v.Submitaddr, v.Dir, stringFinishers(v)))
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
		finishLogLine := fmt.Sprintf("finished: [jid %d] %s. done: %s. cmd: '%s %s' finished on worker '%s'/pid:%d.  %s. Err: '%s' dir:'%v'", v.Id, totalTimeString(v), NanoToTime(Ntm(v.Etm)), v.Cmd, v.Args, v.Workeraddr, v.Pid, stringFinishers(v), v.Err, v.Dir)
		out = append(out, finishLogLine)
	}

	//out = append(out, fmt.Sprintf("--- goq security status---"))
	//out = append(out, fmt.Sprintf("summary-bad-signature-msgs: %d", js.BadSgtCount))
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
func (js *JobServ) SetAddrDestSocket(destAddr string, job *Job) error {
	dest, ok := js.Who[destAddr]
	if !ok {
		AlwaysPrintf("destAddr '%s' not avail", destAddr)
		return ErrNA
	}
	job.destinationSock = dest.DemandPushSock()
	return nil
}

var ErrNA = fmt.Errorf("worker address not found")

func (js *JobServ) DispatchJobToWorker(reqjob, job *Job) {
	//vv("top of DispatchJobToWorker()")
	job.Msg = schema.JOBMSG_DELEGATETOWORKER

	if job.Id == 0 {
		panic("job.Id must be non-zero by now")
	}

	job.Delegatetm = time.Now().UnixNano()
	js.RunQ[job.Id] = job

	if js.IsLocal {
		AlwaysPrintf("**** [jobserver pid %d] dispatching job %d to local worker.\n", js.Pid, job.Id)

		js.ToWorker <- job
		return
	}

	job.Workeraddr = reqjob.Workeraddr

	err := js.SetAddrDestSocket(reqjob.Workeraddr, job)
	if err != nil {
		AlwaysPrintf("cannot find the chosen worker's address: reqjob.Workeraddr='%s'", reqjob.Workeraddr)
		return
	}
	AlwaysPrintf("**** [jobserver pid %d] dispatching job %d to worker '%s'.\n", js.Pid, job.Id, reqjob.Workeraddr)

	// try to send, give worker 30 seconds to grab it.
	if job.destinationSock != nil {
		go func(job Job) { // by value, so we can read without any race
			// we can send, go for it. But be on the lookout for timeout, i.e. when worker dies
			// before receiving their job. Then we should just re-queue it.
			//vv("pushing job to job.Workeraddr='%s'", job.Workeraddr)
			key, ok, err := js.CBM.pushJobToClient("", job.Workeraddr, &job)
			//			_, err := sendZjob(job.destinationSock, &job, &js.Cfg, nil)
			_ = key
			_ = ok
			//vv("pushJobToClient got back err='%v'", err)
			if err != nil {
				// for now assume deaf worker
				//vv("[pid %d] Got error back trying to dispatch job %d to worker '%s'. Incrementing "+
				//	"deaf worker count and resubmitting. err: %s\n", os.Getpid(), job.Id, job.Workeraddr, err)
				// arg: can't touch the jobserv when not in Start either: incrementing js.CountDeaf is a race!!
				// js.CountDeaf++

				// can't touch the job when not in Start() goroutine!!! So no: job.Msg = schema.JOBMSG_RESUBMITNOACK
				// have to let Start() notice that it is a resub, b/c Id and Workeraddr are already set.
				js.ReSubmit <- job.Id
			} else {
				//vv("[pid %d] dispatched job %d to worker '%s'\n", os.Getpid(), job.Id, job.Workeraddr)
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

func (js *JobServ) returnToWaitingCallerWith(donejob *Job, msg schema.JobMsg) {
	if donejob.replyCh == nil {
		return
	}
	ackjob := NewJob()
	ackjob.Msg = msg
	ackjob.Aboutjid = donejob.Id
	ackjob.Submitaddr = donejob.Submitaddr
	ackjob.Serveraddr = donejob.Serveraddr
	ackjob.Workeraddr = donejob.Workeraddr
	js.returnToCaller(donejob, ackjob)
}

func (js *JobServ) TellFinishers(donejob *Job, msg schema.JobMsg) {
	//vv("TellFinishers sees donejob.Finishaddr (len %v) = '%v'", len(donejob.Finishaddr), donejob.Finishaddr)
	if len(donejob.Finishaddr) == 0 {
		return
	}

	for i := range donejob.Finishaddr {
		job := NewJob()
		job.Msg = msg
		job.Aboutjid = donejob.Id
		job.Submitaddr = donejob.Finishaddr[i]

		go func(job *Job, addr string) {
			_, _, err := js.CBM.pushJobToClient(donejob.callid, job.Submitaddr, job)
			if err != nil {
				// timed-out
				AlwaysPrintf("[pid %d] TellFinishers for job %d with msg %s to '%s' timed-out after %d msec.\n", os.Getpid(), job.Aboutjid, job.Msg, addr, js.Cfg.SendTimeoutMsec)
			} else {
				AlwaysPrintf("[pid %d] TellFinishers for job %d with msg %s to '%s' succeeded.\n", os.Getpid(), job.Aboutjid, job.Msg, addr)
			}

			nc, err := js.CBM.get(addr)
			_ = nc
			if err != nil {
				//nc.Close()
			}
		}(job, donejob.Finishaddr[i])
	}

}

const yesSkipTheWrite = true

func (js *JobServ) returnToCaller(reqjob, ansjob *Job) {
	// sender did DoCallSync() and is waiting for a reply.
	// send it back on the replyCh.
	//vv("sendZjob is sending on replyCh %v bytes", len(cy))
	cy, err := js.Cfg.jobToBytes(ansjob)
	panicOn(err)
	reply := &rpc.Message{
		JobSerz: cy,
	}
	select {
	case reqjob.replyCh <- reply:
		//vv("sendZjob after send on replyCh %v bytes", len(cy))
	case <-js.ListenerShutdown:
	default:
		// might not be anyone there?
	}
}

// AckBack is used when Jserv doesn't expect a reply after this one (and we aren't issuing work).
// It puts toaddr into a new Job's Submitaddr and sends to toaddr.
func (js *JobServ) AckBack(reqjob *Job, toaddr string, msg schema.JobMsg, out []string) {
	if js.IsLocal {
		return
	}

	//vv("AckBack called with toaddr = '%v'", toaddr) // 'udp://[::]:51633' problem!

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

	err := js.SetAddrDestSocket(toaddr, job)
	panicOn(err)
	job.Submitaddr = toaddr
	job.Serveraddr = js.Addr
	job.Workeraddr = reqjob.Workeraddr

	// try to send, give badsig sender

	if reqjob.replyCh != nil {
		//vv("AckBack trying returnToCaller...")
		js.returnToCaller(reqjob, job)
	}

	if job.destinationSock != nil {
		go func(job Job, addr string) {
			// doesn't matter if it times out, and it prob will.
			//vv("AckBack is calling  pushJobToClient(addr='%s')", addr)
			_, _, err := js.CBM.pushJobToClient(reqjob.callid, addr, &job)
			if err != nil {
				// for now assume deaf worker
				AlwaysPrintf("[pid %d] AckBack with msg %s to '%s' timed-out.\n", os.Getpid(), job.Msg, addr)
			}
			// close socket, to try not to leak it. ofh_test (open file handles test)
			// needs this next line or we'll see leaks.

			// jea, with changeover to rpc, let us let the client close to avoid FINWAIT
			//js.UnregSubmitWho <- &job
			return
		}(*job, toaddr)
	} else {
		AlwaysPrintf("[pid %d] hmmm... jobserv could not find desination for final reply to addr: '%s'. Job: %#v\n", os.Getpid(), toaddr, job)
		// close socket, to try not to leak it.
		//panic("should never get here now; the implementation of js.SetAddrDestSocket(toaddr, job) should now always find (or create on demand) the socket!")
		js.UnRegisterSubmitter(job)
	}
}

func (js *JobServ) DispatchShutdownWorker(immojob, workerready *Job) {
	j := NewJob()
	j.Msg = schema.JOBMSG_SHUTDOWNWORKER
	j.Serveraddr = js.Addr
	j.Submitaddr = immojob.Submitaddr
	j.Workeraddr = workerready.Workeraddr

	err := js.SetAddrDestSocket(j.Workeraddr, j)
	if err != nil {
		//vv("can't find worker addr to shutdown")
		return
	}
	AlwaysPrintf("**** [jobserver pid %d] sending 'shutdownworker' to worker '%s'.\n", js.Pid, j.Workeraddr)

	// try to send, give worker 30 seconds to grab it.
	if j.destinationSock == nil {
		panic("trying to immo, but j.DesinationSocket was nil?!?")
	}
	go func(job Job, addr string) { // by value, so we can read without any race
		_, _, err := js.CBM.pushJobToClient(immojob.callid, addr, &job)
		if err != nil {
			// ignore
		} else {
			AlwaysPrintf("**** [jobserver pid %d] dispatched 'shutdownworker' to worker '%s'\n", os.Getpid(), job.Workeraddr)
		}
		return
	}(*j, j.Workeraddr)
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
			var job *Job
			select {
			case job = <-js.FromRpcServer:
				//vv("got job from js.FromRpcServer: '%v'", job.Msg.String())
				// below
			case <-time.After(time.Second):
				continue // ignore timeouts after N seconds
			case <-js.ListenerShutdown:
				return
			}
			//vv("goq ListenForJobs got * %s * job: %s. %s\n", job.Msg, job, js.ifDebugCid())

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
				AlwaysPrintf("**** [jobserver pid %d] got ack of cancelled for job %d from worker '%s'; job.Cancelled: %v.\n", os.Getpid(), job.Id, job.Workeraddr, job.Cancelled)
				select {
				case <-js.ListenerShutdown:

					return
				case js.RunDone <- job:
				}

			default:
				AlwaysPrintf("Listener: unrecognized JobMsg: '%v' in job: %s\n", job.Msg, job)
			}
			VPrintf("\nListener at bottom of for{} loop\n")
		}
	}()
}

func (cfg *Config) jobToBytesWithStamp(j *Job) (by []byte, err error) {
	StampJob(j)
	return cfg.jobToBytes(j)
}

// do 	StampJob(j) yourself
func (cfg *Config) jobToBytes(j *Job) (by []byte, err error) {
	if j.Submitaddr == "" && j.Serveraddr == "" && j.Workeraddr == "" {
		//if cfg.DebugMode {
		panic(fmt.Sprintf("job cannot have all empty addresses: %s", j))
		//} // else don't crash the process on bad job send
	}

	SignJob(j, cfg)
	buf, _ := JobToCapnp(j)

	cy := []byte{}
	if AesOff {
		cy = buf.Bytes()
	} else {
		cy = cfg.Cypher.Encrypt(buf.Bytes())
	}
	return cy, nil
}

func (cfg *Config) bytesToJob(by []byte) (j *Job, err error) {
	// harden against cross-cluster communication
	defer func() {
		r := recover()
		if r != nil {
			j = nil
			err = fmt.Errorf("unknown recovered error in bytesToJob(): '%v'. where='\n%s'", r, Stack()) // EOF
		}
	}()

	plain := []byte{}
	if AesOff {
		plain = by
	} else {
		plain = cfg.Cypher.Decrypt(by)
	}

	buf := bytes.NewBuffer(plain)
	return CapnpToJob(buf)
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
	//if AesOff {
	//job.Out = append(job.Out, "clusterid:"+cfg.ClusterId)
	//}
	if len(args) > 1 {
		job.Args = args[1:]
	}

	// efficient submission of lots of jobs: special case
	// the referencing of jobs in a file, so we only need
	// one submit process instance, not a million, and
	// one job submission transmission.
	if job.Cmd == "lines@" {
		if len(job.Args) != 1 {
			panic("lines@ requested but no single file path followed")
		}
		if !FileExists(job.Args[0]) {
			panic(fmt.Sprintf("lines@ could not find path '%v'", job.Args[0]))
		}
		job.Cmd += job.Args[0] // append path to lines@
		by, err := ioutil.ReadFile(job.Args[0])
		panicOn(err)
		job.Args = strings.Split(string(by), "\n")
	}

	return job
}

// barebones, just get it done.
func SendShutdown(cfg *Config) {
	sub, err := NewSubmitter(cfg, false)
	if err != nil {
		panic(err)
	}
	sub.SubmitShutdownJob()
}

func (cfg *Config) IsAlreadyBound(addr string) (bool, error) {

	stripped, err := StripNanomsgAddressPrefix(addr)
	if err != nil {
		panic(err)
	}

	//vv("stripped = '%v'", stripped)
	if cfg.UseQUIC {
		conn, err := net.ListenPacket("udp", stripped)
		if err != nil {
			return true, err
		}
		conn.Close()
		return false, nil // Port is not in use
	}

	ln, err := net.Listen("tcp", stripped)
	if err != nil {
		return true, err
	}
	ln.Close()
	return false, nil
}

func SubmitGetServerSnapshot(cfg *Config) ([]string, error) {
	sub, err := NewSubmitter(cfg, false)
	if err != nil {
		panic(err)
	}

	j := NewJob()
	j.Msg = schema.JOBMSG_TAKESNAPSHOT

	return sub.SubmitSnapJob(10)
}

func (cfg *Config) WaitUntilAddrAvailable(addr string) int {
	sleeps := 0
	for {
		var isbound bool
		isbound, _ = cfg.IsAlreadyBound(addr)
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

func startProfilingCPU(path string) {
	// add randomness so two tests run at once don't overwrite each other.
	rnd8 := cryrand.RandomStringWithUp(8)
	fn := path + ".cpuprof." + rnd8
	f, err := os.Create(fn)
	panicOn(err)
	AlwaysPrintf("will write cpu profile to '%v'", fn)
	go func() {
		pprof.StartCPUProfile(f)
		time.Sleep(time.Minute)
		pprof.StopCPUProfile()
		AlwaysPrintf("stopped and wrote cpu profile to '%v'", fn)
	}()
}

func startOnlineWebProfiling() (port int) {

	// To dump goroutine stack from a running program for debugging:
	// Start an HTTP listener if you do not have one already:
	// Then point a browser to http://127.0.0.1:9999/debug/pprof for a menu, or
	// curl http://127.0.0.1:9999/debug/pprof/goroutine?debug=2
	// for a full dump.
	port = GetAvailPort()
	go func() {
		err := http.ListenAndServe(fmt.Sprintf("127.0.0.1:%v", port), nil)
		if err != nil {
			panic(err)
		}
	}()
	fmt.Fprintf(os.Stderr, "\n for stack dump:\n\ncurl http://127.0.0.1:%v/debug/pprof/goroutine?debug=2\n\n for general debugging:\n\nhttp://127.0.0.1:%v/debug/pprof\n\n", port, port)
	return
}

func startProfilingMemory(path string, wait time.Duration) {
	// add randomness so two tests run at once don't overwrite each other.
	rnd8 := cryrand.RandomStringWithUp(8)
	fn := path + ".memprof." + rnd8
	if wait == 0 {
		wait = time.Minute // default
	}
	AlwaysPrintf("will write mem profile to '%v'; after wait of '%v'", fn, wait)
	go func() {
		time.Sleep(wait)
		WriteMemProfiles(fn)
	}()
}

func WriteMemProfiles(fn string) {
	if !strings.HasSuffix(fn, ".") {
		fn += "."
	}
	h, err := os.Create(fn + "heap")
	panicOn(err)
	defer h.Close()
	a, err := os.Create(fn + "allocs")
	panicOn(err)
	defer a.Close()
	g, err := os.Create(fn + "goroutine")
	panicOn(err)
	defer g.Close()

	hp := pprof.Lookup("heap")
	ap := pprof.Lookup("allocs")
	gp := pprof.Lookup("goroutine")

	panicOn(hp.WriteTo(h, 1))
	panicOn(ap.WriteTo(a, 1))
	panicOn(gp.WriteTo(g, 2))
}
