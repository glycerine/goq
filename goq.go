package main

// copyright(c) 2014, Jason E. Aten
//
// goq : a simple queueing system in go; qsub replacement.
//

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"time"

	schema "github.com/glycerine/goq/schema"
	capn "github.com/glycerine/go-capnproto"
	nn "github.com/op/go-nanomsg"
	//nn "bitbucket.org/gdamore/mangos/compat"
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

var Verbose bool

func Vprintf(format string, a ...interface{}) {
	if Verbose {
		fmt.Printf(format, a...)
	}
}

type control int

const (
	nothing control = iota
	die
)

func (cmd control) String() string {
	switch cmd {
	case die:
		return "die"
	}
	return fmt.Sprintf("%d", cmd)
}

// cache the sockets for reuse
type PushCache struct {
	Name string

	Addr     string     // even port number (mnemonic: stdout is 0/even)
	PushSock *nn.Socket // from => pull
}

func NewPushCache(name, addr string, cfg *Config) *PushCache {
	p := &PushCache{
		Name: name,
		Addr: addr,
	}

	t, err := MkPushNN(addr, cfg, false)
	if err != nil {
		panic(err)
	}

	p.PushSock = t

	return p
}

// Job represents a job to perform, and is our universal message type.
type Job struct {
	Id       int64
	Msg      schema.JobMsg
	Aboutjid int64 // in acksubmit, this holds the jobid of the job on the runq, so that Id can be unique and monotonic.

	Cmd     string
	Args    []string
	Out     []string
	Env     []string
	Host    string
	Stm     int64
	Etm     int64
	Elapsec int64
	Status  string
	Subtime int64
	Pid     int64
	Dir     string

	Submitaddr string
	Serveraddr string
	Workeraddr string

	Finishaddr []string // who, if anyone, you want notified upon job completion. JOBMSG_JOBFINISHEDNOTICE will be sent.

	Signature string
	IsLocal   bool

	// not serialized, just used
	// for routing
	DestinationSocket *nn.Socket
}

var NextJobId int64

func NewJob() *Job {
	j := &Job{
		Id:         0, // only server should assign job.Id, until then, should be 0.
		Args:       make([]string, 0),
		Out:        make([]string, 0),
		Env:        make([]string, 0),
		Finishaddr: make([]string, 0),
	}
	return j
}

func NewJobId() int64 {
	id := NextJobId
	NextJobId++
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
		if _, ok := js.Who[j.Workeraddr]; !ok {
			if c, found := js.Who[j.Workeraddr]; found {
				c.PushSock.Close()
				delete(js.Who, j.Workeraddr)
			}
		}
	}

	if j.Submitaddr != "" {
		if _, ok := js.Who[j.Submitaddr]; !ok {
			if c, found := js.Who[j.Submitaddr]; found {
				c.PushSock.Close()
				delete(js.Who, j.Submitaddr)
			}
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

func (js *JobServ) CloseRegistery() {
	for _, pp := range js.Who {
		if pp.PushSock != nil {
			//LogClose(pp.PushSock)
			pp.PushSock.Close()
		}
	}

	js.Shutdown()
}

func (js *JobServ) Shutdown() {
	if js.Nnsock != nil {
		//LogClose(js.Nnsock)
		js.Nnsock.Close()
	}
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
	SnapRequest     chan *Job  // worker requests state snapshot from JobServ.
	ObserveFinish   chan *Job  // submitter sends on, Jobserv recieves on; when a submitter wants to wait for another job to be done.
	NotifyFinishers chan *Job  // submitter receives on, jobserv dispatches a notification message for each finish observer
	Cancel          chan *Job  // submitter sends on, to request job cancellation.

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

	// directory of submitters and workers
	Who map[string]*PushCache

	// Finishers : who wants to be notified when a job is done.
	Finishers map[int64][]Address

	CountDeaf         int
	PrevDeaf          int
	BadSgtCount       int64
	FinishedJobsCount int64

	// set Cfg *once*, before any goroutines start, then
	// treat it as immutable and never changing.
	Cfg       Config
	DebugMode bool // show badsig messages if true
	IsLocal   bool
}

// don't make consumers of DeafChan busy wait;
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
	//argv := os.Argv()
	cmd := exec.Command(GoqExeName, "serve")

	cmd.Env = cfg.Setenv(os.Environ())

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Start()

	// reap so we don't zombify, which makes
	// it difficult for the test in fetch_test.go to detect that
	// the process is indeed gone. This one liner fixes all that.
	go func() { cmd.Wait() }()

	return cmd.Process.Pid, err
}

func NewJobServ(addr string, cfg *Config) (*JobServ, error) {

	NewJobId() // give out id starting at 1, so we can detect uninitialized jobs.

	if cfg == nil {
		cfg = DefaultCfg()
	}

	var pullsock *nn.Socket
	var err error
	var remote bool
	if addr != "" {
		remote = true
		pullsock, err = MkPullNN(addr, cfg, false)
		if err != nil {
			panic(err)
		}

		Vprintf("[pid %d] JobServer bound endpoints addr: '%s'\n", os.Getpid(), addr)
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

		WaitingJobs: make([]*Job, 0),
		Submit:      make(chan *Job),
		ReSubmit:    make(chan int64),
		WorkerReady: make(chan *Job),
		ToWorker:    make(chan *Job),
		RunDone:     make(chan *Job),
		SigMismatch: make(chan *Job),
		SnapRequest: make(chan *Job),
		Cancel:      make(chan *Job),

		ObserveFinish:   make(chan *Job), // when a submitter wants to wait for another job to be done.
		NotifyFinishers: make(chan *Job),

		DeafChan:       make(chan int),
		Ctrl:           make(chan control),
		Done:           make(chan bool),
		WaitingWorkers: make([]*Job, 0),
		Who:            make(map[string]*PushCache),
		Finishers:      make(map[int64][]Address),

		Pid:       os.Getpid(),
		Cfg:       *cfg,
		DebugMode: cfg.DebugMode,
		Odir:      cfg.Odir,
		IsLocal:   !remote,
	}

	js.Start()
	if remote {
		//Vprintf("remote, server starting ListenForJobs() goroutine.\n")
		fmt.Printf("**** [jobserver pid %d] listening for jobs, output to '%s'.\n", js.Pid, js.Odir)
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

func (js *JobServ) ConfirmOrMakeOutputDir() {
	if !DirExists(js.Odir) {
		err := os.Mkdir(js.Odir, 0775)
		if err != nil {
			panic(err)
		}
	}
}

func (js *JobServ) WriteJobOutputToDisk(donejob *Job) {
	Vprintf("WriteJobOutputToDisk() called for Job: %#v\n", donejob)

	js.ConfirmOrMakeOutputDir()

	fn := fmt.Sprintf("%s/out.%05d", js.Odir, donejob.Id)

	// append if already existing file: so we can have incremental updates.
	var err error
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
	fmt.Printf("[pid %d] jobserver wrote output for job %d to file '%s'\n", js.Pid, donejob.Id, fn)
}

func (js *JobServ) Start() {

	go func() {
		var loopcount int64 = 0
		for {
			loopcount++
			Vprintf(" - - - JobServ at top for Start() event loop, loopcount: (%d).\n", loopcount)

			select {
			case newjob := <-js.Submit:
				Vprintf("  === event loop case ===  (%d) JobServ got from Submit channel a newjob, msg: %s, job: %#v\n", loopcount, newjob.Msg, newjob)

				if newjob.Id != 0 {
					panic(fmt.Sprintf("new jobs should have zero (unassigned) Id!!! But, this one did not: %#v", newjob))
				}

				curId := NewJobId()
				newjob.Id = curId
				js.KnownJobHash[curId] = newjob

				// open and cache any sockets we will need.
				js.RegisterWho(newjob)

				if newjob.Msg == schema.JOBMSG_SHUTDOWNSERV {
					Vprintf("JobServ got JOBMSG_SHUTDOWNSERV from Submit channel.\n")
					go func() { js.Ctrl <- die }()
					continue
				}

				fmt.Printf("**** [jobserver pid %d] got job %d submission. Will run '%s'.\n", js.Pid, newjob.Id, newjob.Cmd)

				js.WaitingJobs = append(js.WaitingJobs, newjob)
				js.Dispatch()
				// we just dispatched, now reply to submitter with ack (in an async goroutine); they don't need to
				// wait for it, but often they will want confirmation/the jobid.
				js.AckBack(newjob, newjob.Submitaddr, schema.JOBMSG_ACKSUBMIT, []string{})

			case resubId := <-js.ReSubmit:
				Vprintf("  === event loop case === (%d) JobServ got resub for jobid %d\n", loopcount, resubId)
				js.CountDeaf++
				resubJob, ok := js.RunQ[resubId]
				if !ok {
					// maybe it was cancelled in the meantime. panic(fmt.Sprintf("go resub for job id(%d) that isn't on our RunQ", resubId)
					fmt.Printf("**** [jobserver pid %d] got re-submit of job %d that is now not on our RunQ, so dropping it without re-queuing.\n", js.Pid, resubId)
					continue
				}
				fmt.Printf("**** [jobserver pid %d] got re-submit of job %d that was dispatched to '%s'. Trying again.\n", js.Pid, resubId, resubJob.Workeraddr)
				resubJob.Workeraddr = ""

				delete(js.RunQ, resubId)
				// prepend, so the job doesn't loose its place in line. *try* to FIFO as much as possible.
				js.WaitingJobs = append([]*Job{resubJob}, js.WaitingJobs...)
				js.Dispatch()

			case reqjob := <-js.WorkerReady:
				Vprintf("  === event loop case === (%d) JobServ got request for work from WorkerReady channel: %#v\n", loopcount, reqjob)
				if !js.IsLocal && reqjob.Workeraddr == "" {
					// ignore bad packets
				}
				js.RegisterWho(reqjob)
				if _, dup := js.DedupWorkerHash[reqjob.Workeraddr]; !dup {
					js.WaitingWorkers = append(js.WaitingWorkers, reqjob)
					js.DedupWorkerHash[reqjob.Workeraddr] = true
				} else {
					fmt.Printf("**** [jobserver pid %d] ignored duplicate worker-ready message from '%s'\n", js.Pid, reqjob.Workeraddr)
				}
				js.Dispatch()

			case donejob := <-js.RunDone:
				Vprintf("  === event loop case === (%d)  JobServ got donejob from RunDone channel: %#v\n", loopcount, donejob)
				// we've got a new copy, with Out on it, but the old copy may have added listeners, so
				// we'll need to merge in those Finishaddr too.

				withFinishers, ok := js.RunQ[donejob.Id]
				if !ok {
					panic(fmt.Sprintf("got donejob %d for job(%#v) from js.RunDone channel, but it was not in our js.RunQ: %#v", donejob.Id, donejob, js.RunQ))
				}
				kjh, ok := js.KnownJobHash[donejob.Id]
				if !ok {
					panic(fmt.Sprintf("got donejob %d for job(%#v) from js.RunDone channel, but it was not in our js.KnownJobHash: %#v", donejob.Id, donejob, js.KnownJobHash))
				}
				if withFinishers != kjh {
					panic(fmt.Sprintf("withFinishers(%v) from RunQ did not agree with kjh(%v) from KnownJobHash", withFinishers, kjh))
				}

				donejob.Finishaddr = js.MergeAndDedupFinishers(donejob, withFinishers)

				delete(js.RunQ, donejob.Id)
				delete(js.KnownJobHash, donejob.Id)
				js.FinishedJobsCount++
				fmt.Printf("**** [jobserver pid %d] worker finished job %d, removing from the RunQ\n", js.Pid, donejob.Id)
				js.WriteJobOutputToDisk(donejob)
				js.TellFinishers(donejob, schema.JOBMSG_JOBFINISHEDNOTICE)

			case cmd := <-js.Ctrl:
				fmt.Printf("  === event loop case === (%d)  JobServ got control cmd: %v\n", loopcount, cmd)
				switch cmd {
				case die:
					fmt.Printf("[jobserver pid %d] jobserver exits in response to shutdown request.\n", js.Pid)
					// try not closing for a minute, to see if we avoid the nanomsg aborts. Didn't seem to help, might hurt?
					js.CloseRegistery()
					// but still have to close ourself:
					js.Shutdown()

					close(js.Done)
					return
				}

			case js.DeafChanIfUpdate() <- js.CountDeaf:
				Vprintf("  === event loop case === (%d)  JobServ supplied js.CountDeaf on channel js.DeafChan.\n", loopcount)
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

					fmt.Printf("**** [jobserver pid %d] DebugMode: actively rejecting badsig message from '%s'.\n", js.Pid, addr)
					if addr != "" {
						js.RegisterWho(badsigjob)
						js.AckBack(badsigjob, addr, schema.JOBMSG_REJECTBADSIG, []string{})
					}
				}

			case snapreq := <-js.SnapRequest:
				js.RegisterWho(snapreq)
				js.AckBack(snapreq, snapreq.Submitaddr, schema.JOBMSG_ACKTAKESNAPSHOT, js.AssembleSnapShot())
				js.UnRegisterWho(snapreq)

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
				}

				delete(js.RunQ, canid)
				delete(js.KnownJobHash, canid)
				js.RemoveFromWaitingJobs(j)

				js.TellFinishers(j, schema.JOBMSG_CANCELSUBMIT)

				js.AckBack(canreq, canreq.Submitaddr, schema.JOBMSG_ACKCANCELSUBMIT, []string{})
			unreg:
				js.UnRegisterWho(canreq)
				fmt.Printf("**** [jobserver pid %d] server cancelled job %d per request of '%s'.\n", js.Pid, canid, canreq.Submitaddr)

			case obsreq := <-js.ObserveFinish:
				if obsreq.Submitaddr == "" {
					// ignore bad requests
					if js.DebugMode {
						fmt.Printf("**** [jobserver pid %d] DebugMode: got Observe Request with bad Submitaddr: %#v.\n", js.Pid, obsreq)
					}
					continue
				}
				if j, ok := js.KnownJobHash[obsreq.Aboutjid]; ok {
					// still in progress, so we add this requester to the Finishaddr list
					fmt.Printf("**** [jobserver pid %d] noting request to get notice about the finish of job %d from '%s'.\n", js.Pid, obsreq.Aboutjid, obsreq.Submitaddr)
					j.Finishaddr = append(j.Finishaddr, obsreq.Submitaddr)
				} else {
					// probably already finished
					fmt.Printf("**** [jobserver pid %d] impossible request for finish-notify oh job %d (unknown job) from '%s'. Sending JOBMSG_JOBNOTKNOWN\n", js.Pid, obsreq.Aboutjid, obsreq.Submitaddr)
					fakedonejob := NewJob()
					fakedonejob.Id = obsreq.Aboutjid
					fakedonejob.Finishaddr = []string{obsreq.Submitaddr}
					js.TellFinishers(fakedonejob, schema.JOBMSG_JOBNOTKNOWN)
				}
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

func (js *JobServ) RemoveFromWaitingJobs(j *Job) {
	wl := len(js.WaitingJobs)
	if wl == 0 {
		return
	}
	slice := make([]*Job, 0, wl)
	k := 0
	found := false
	for _, v := range js.WaitingJobs {
		if v == j {
			found = true
		} else {
			slice[k] = v
			k++
		}
	}
	if !found {
		panic(fmt.Sprintf("jobid %d expected but not found on WaitingJobs", j.Id))
	} else {
		js.WaitingJobs = slice
	}
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
	//fmt.Printf("merge of %#v and %#v  ---->  %#v\n", a.Finishaddr, b.Finishaddr, slice)
	return slice
}

func (js *JobServ) AssembleSnapShot() []string {
	out := make([]string, 0)
	out = append(out, fmt.Sprintf("droppedBadSigCount=%d", js.BadSgtCount))
	out = append(out, fmt.Sprintf("runQlen=%d", len(js.RunQ)))
	out = append(out, fmt.Sprintf("waitingJobs=%d", len(js.WaitingJobs)))
	out = append(out, fmt.Sprintf("waitingWorkers=%d", len(js.WaitingWorkers)))
	out = append(out, fmt.Sprintf("jservPid=%d", js.Pid))
	out = append(out, fmt.Sprintf("finishedJobsCount=%d", js.FinishedJobsCount))
	out = append(out, fmt.Sprintf("nextJobId=%d", NextJobId))
	return out
}

// reqjob should be treated as immutable (read-only) here.
func (js *JobServ) SetAddrDestSocket(destAddr string, job *Job) {
	dest, ok := js.Who[destAddr]
	if ok {
		job.DestinationSocket = dest.PushSock
	}
}

func (js *JobServ) DispatchJobToWorker(reqjob, job *Job) {
	job.Msg = schema.JOBMSG_DELEGATETOWORKER

	if job.Id == 0 {
		panic("job.Id must be non-zero by now")
	}

	js.RunQ[job.Id] = job

	if js.IsLocal {
		fmt.Printf("**** [jobserver pid %d] dispatching job %d to local worker.\n", js.Pid, job.Id)

		js.ToWorker <- job
		return
	}

	job.Workeraddr = reqjob.Workeraddr

	js.SetAddrDestSocket(reqjob.Workeraddr, job)
	fmt.Printf("**** [jobserver pid %d] dispatching job %d to worker '%s'.\n", js.Pid, job.Id, reqjob.Workeraddr)

	// try to send, give worker 30 seconds to grab it.
	if job.DestinationSocket != nil {
		go func(job Job) { // by value, so we can read without any race
			// we can send, go for it. But be on the lookout for timeout, i.e. when worker dies
			// before receiving their job. Then we should just re-queue it.
			err := sendZjob(job.DestinationSocket, &job, &js.Cfg)
			if err != nil {
				// for now assume deaf worker
				fmt.Printf("[pid %d] Got error back trying to dispatch job %d to worker '%s'. Incrementing "+
					"deaf worker count and resubmitting. err: %s\n", os.Getpid(), job.Id, job.Workeraddr, err)
				// arg: can't touch the jobserv when not in Start either: incrementing js.CountDeaf is a race!!
				// js.CountDeaf++

				// can't touch the job when not in Start() goroutine!!! So no: job.Msg = schema.JOBMSG_RESUBMITNOACK
				// have to let Start() notice that it is a resub, b/c Id and Workeraddr are already set.
				js.ReSubmit <- job.Id
			} else {
				fmt.Printf("[pid %d] dispatched job %d to worker '%s'\n", os.Getpid(), job.Id, job.Workeraddr)
			}
			return
		}(*job)
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
			err := sendZjob(sock, job, &js.Cfg)
			if err != nil {
				// timed-out
				fmt.Printf("[pid %d] TellFinishers for job %d with msg %s to '%s' timed-out after %d msec.\n", os.Getpid(), job.Aboutjid, job.Msg, addr, js.Cfg.SendTimeoutMsec)
			} else {
				fmt.Printf("[pid %d] TellFinishers for job %d with msg %s to '%s' succeeded.\n", os.Getpid(), job.Aboutjid, job.Msg, addr)
			}
			sock.Close()
			return
		}(job, sock, donejob.Finishaddr[i])
	}

}

// AckBack is used when Jserv doesn't expect a reply after this one (and we aren't issuing work).
// It plus addr into a new Jobs Submitaddr and sends to addr.
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
	if job.DestinationSocket != nil {
		go func(job Job, addr string) {
			// doesn't matter if it times out, and it prob will.
			err := sendZjob(job.DestinationSocket, &job, &js.Cfg)
			if err != nil {
				// for now assume deaf worker
				fmt.Printf("[pid %d] AckBack with msg %s to '%s' timed-out.\n", os.Getpid(), job.Msg, addr)
			}
			return
		}(*job, toaddr)
	} else {
		fmt.Printf("[pid %d] hmmm... jobserv could not find desination for final reply to addr: '%s'. Job: %#v\n", os.Getpid(), toaddr, job)
	}
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

func (js *JobServ) ListenForJobs(cfg *Config) {
	go func() {
		for {
			// recvZjob blocks, which is why we are in our own goroutine.
			// do we guard against address already bound errors here?
			job, err := recvZjob(js.Nnsock)
			if err != nil {
				continue // ignore timeouts after N seconds
			}
			Vprintf("ListenForJobs got * %s * job: %#v\n", job.Msg, job)

			// check signature
			if !JobSignatureOkay(job, cfg) {
				if js.DebugMode {
					fmt.Printf("[pid %d] dropping job '%s' (Msg: %s) from '%s' whose signature did not verify.\n", os.Getpid(), job.Cmd, job.Msg, discrimAddr(job))
				}
				js.SigMismatch <- job
				continue
			}

			switch job.Msg {
			case schema.JOBMSG_INITIALSUBMIT:
				js.Submit <- job
			case schema.JOBMSG_REQUESTFORWORK:
				js.WorkerReady <- job
			case schema.JOBMSG_DELEGATETOWORKER:
				panic("server should never receive JOBMSG_DELEGATETOWORKER, only send it to worker. ")
			case schema.JOBMSG_FINISHEDWORK:
				js.RunDone <- job
			case schema.JOBMSG_SHUTDOWNSERV:
				js.Ctrl <- die
			case schema.JOBMSG_TAKESNAPSHOT:
				js.SnapRequest <- job
			case schema.JOBMSG_OBSERVEJOBFINISH:
				js.ObserveFinish <- job
			case schema.JOBMSG_CANCELSUBMIT:
				js.Cancel <- job
			default:
				panic(fmt.Sprintf("unrecognized JobMsg: %v   in job: %#v", job.Msg, job))
			}
		}
	}()
}

func recvMsgOnZBus(nnzbus *nn.Socket) {
	pid := os.Getpid()

	// receive, synchronously so flags == 0
	var flags int = 0
	//LogRecv(nnzbus)
	heardBuf, err := nnzbus.Recv(flags)
	if err != nil {
		panic(err)
	}

	Vprintf("[pid %d] gozbus server: I heard: '%s'.\n", pid, heardBuf)
}

func sendZjob(nnzbus *nn.Socket, j *Job, cfg *Config) error {

	// sanity check
	if j.Submitaddr == "" && j.Serveraddr == "" && j.Workeraddr == "" {
		panic("job cannot have all empty addresses")
	}

	// Create Zjob and Write to nnzbus.
	SignJob(j, cfg)
	buf, _ := JobToCapnp(j)
	//LogSend(nnzbus)
	_, err := nnzbus.Send(buf.Bytes(), 0)
	return err

}

func StringSliceToCapnp(a []string, seg *capn.Segment) *capn.TextList {
	if a == nil {
		panic("string slice can't be nil")
	}
	if len(a) > 0 {
		tl := seg.NewTextList(len(a))
		for i := range a {
			tl.Set(i, a[i])
		}
		return &tl
	}
	return nil
}

func JobToCapnp(j *Job) (bytes.Buffer, *capn.Segment) {
	seg := capn.NewBuffer(nil)
	z := schema.NewRootZ(seg)
	zjob := schema.NewZjob(seg)

	zjob.SetId(j.Id)
	zjob.SetAboutjid(j.Aboutjid)
	zjob.SetMsg(j.Msg)

	zjob.SetCmd(j.Cmd)

	if tl := StringSliceToCapnp(j.Out, seg); tl != nil {
		zjob.SetOut(*tl)
	}

	if tl := StringSliceToCapnp(j.Args, seg); tl != nil {
		zjob.SetArgs(*tl)
	}

	if tl := StringSliceToCapnp(j.Env, seg); tl != nil {
		zjob.SetEnv(*tl)
	}

	zjob.SetHost(j.Host)

	zjob.SetStm(int64(j.Stm))
	zjob.SetEtm(int64(j.Etm))
	zjob.SetElapsec(j.Elapsec)

	zjob.SetStatus(j.Status)
	zjob.SetSubtime(int64(j.Subtime))
	zjob.SetPid(j.Pid)

	zjob.SetDir(j.Dir)

	zjob.SetSubmitaddr(j.Submitaddr)
	zjob.SetServeraddr(j.Serveraddr)
	zjob.SetWorkeraddr(j.Workeraddr)

	if tl := StringSliceToCapnp(j.Finishaddr, seg); tl != nil {
		zjob.SetFinishaddr(*tl)
	}

	zjob.SetSignature(j.Signature)
	zjob.SetIslocal(j.IsLocal)

	z.SetJob(zjob)

	buf := bytes.Buffer{}
	seg.WriteTo(&buf)

	return buf, seg
}

func recvZjob(nnzbus *nn.Socket) (*Job, error) {

	// Read job submitted to the server
	//LogRecv(nnzbus)
	myMsg, err := nnzbus.Recv(0)
	if err != nil {
		return nil, err
	}

	buf := bytes.NewBuffer(myMsg)
	job := CapnpToJob(buf)
	return job, nil
}

func CapnpToJob(buf *bytes.Buffer) *Job {
	capMsg, err := capn.ReadFromStream(buf, nil)
	if err != nil {
		panic(err)
	}

	z := schema.ReadRootZ(capMsg)
	d3 := z.Which()
	if d3 != schema.Z_JOB {
		panic(fmt.Sprintf("expected schema.Z_JOB, got %d", d3))
	}

	zj := z.Job()

	job := &Job{
		Id:       zj.Id(),
		Msg:      zj.Msg(),
		Aboutjid: zj.Aboutjid(),

		Cmd:  zj.Cmd(),
		Args: zj.Args().ToArray(),
		Out:  zj.Out().ToArray(),
		Env:  zj.Env().ToArray(),

		Host: zj.Host(),
		Stm:  zj.Stm(),
		Etm:  zj.Etm(),

		Elapsec: zj.Elapsec(),
		Status:  zj.Status(),
		Subtime: zj.Subtime(),

		Pid: zj.Pid(),
		Dir: zj.Dir(),

		Submitaddr: zj.Submitaddr(),
		Serveraddr: zj.Serveraddr(),
		Workeraddr: zj.Workeraddr(),
		Finishaddr: zj.Finishaddr().ToArray(),

		Signature: zj.Signature(),
		IsLocal:   zj.Islocal(),
	}

	//Vprintf("[pid %d] recvZjob got Zjob message: %#v\n", os.Getpid(), job)
	return job
}

func MakeTestJob() *Job {
	job := NewJob()

	job.Id = NewJobId()
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

func MakeActualJob(args []string) *Job {
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
	if len(args) > 1 {
		job.Args = args[1:]
	}
	return job
}

func main() {

	//startLog()
	//defer closeLog()

	pid := os.Getpid()

	var isServer bool
	if len(os.Args) > 1 && (os.Args[1] == "serve" || os.Args[1] == "server") {
		isServer = true
	}

	var isSubmitter bool
	if len(os.Args) > 1 && os.Args[1] == "sub" {
		isSubmitter = true
	}

	var isWorker bool
	if len(os.Args) > 1 && os.Args[1] == "work" {
		isWorker = true
	}

	var isKill bool
	if len(os.Args) > 1 && os.Args[1] == "kill" {
		isKill = true
	}

	var isShutdown bool
	if len(os.Args) > 1 && os.Args[1] == "shutdown" {
		isShutdown = true
	}

	var isStat bool
	if len(os.Args) > 1 && os.Args[1] == "stat" {
		isStat = true
	}

	var isWait bool
	if len(os.Args) > 1 && os.Args[1] == "wait" {
		isWait = true
	}

	// deafWorker is for testing the behavior
	// of the jobserver when the worker dies or
	// doesn't answer after requesting a job.
	var isDeafWorker bool
	if len(os.Args) > 1 && os.Args[1] == "deafworker" {
		isDeafWorker = true
	}

	var isClusterid bool
	if len(os.Args) > 1 && os.Args[1] == "clusterid" {
		isClusterid = true
	}

	// report existing id from GOQ_CLUSTERID env var; (generates new random one if none found in the env)
	home := ErrorCheckedPwd()
	cfg, err := DiskThenEnvConfig(home)
	if err != nil {
		// can't display this, because would mess up clusterid command which expects just
		//  a clusterid on stdout.
		// fmt.Printf("[pid %d] ignoring error on trying to read .goqclusterid file in home '%s'\n", pid, home)
	}

	switch {
	case isClusterid:
		fmt.Printf("%s\n", cfg.ClusterId)
		os.Exit(0)

	case isServer:
		Vprintf("[pid %d] making new external job server, listening on %s\n", pid, cfg.JservAddr)

		// save our cfg so other clients can read and access this server.
		SaveLocalClusterId(cfg.ClusterId, home, cfg)

		// report to a log file too, so we aren't blind.
		/*
			file, err := os.Create("server.out-goq")
			if err == nil {
				fmt.Fprintf(file, "[pid %d] %v : making new external job server, listening on %s\n", pid, time.Now(), cfg.JservAddr)
				file.Close()
			} else {
				panic(err)
			}
		*/
		serv, err := NewJobServ(cfg.JservAddr, cfg)
		if err != nil {
			panic(err)
		}

		Vprintf("[pid %d] job server made, now handling requests.\n", pid)
		// wait till done, serving requests
		<-serv.Done

	case isSubmitter:
		args := os.Args[2:]
		if len(args) == 0 {
			fmt.Printf("[pid %d] cowardly refusing to submit empty job.\n", pid)
			os.Exit(1)
		}
		subaddr := GenAddress()
		sub, err := NewSubmitter(subaddr, cfg, false)
		if err != nil {
			panic(err)
		}
		todojob := MakeActualJob(args)
		Vprintf("[pid %d] submitter instantiated, make testjob to submit over nanomsg: %#v.\n", pid, todojob)

		reply, err := sub.SubmitJobGetReply(todojob)
		if err != nil {
			panic(err)
		}
		if reply.Aboutjid != 0 {
			fmt.Printf("[pid %d] submitted job %d to server at '%s'.\n", pid, reply.Aboutjid, cfg.JservAddr)
			os.Exit(0)
		}
		fmt.Printf("[pid %d] submitted job to server over nanomsg, got unexpected '%s' reply: %#v.\n", pid, reply.Msg, reply)
		os.Exit(1)

	case isWorker:
		// client code, connects to the bus.
		waddr := GenAddress()

		// set a small, 1 seecond, timeout
		cpcfg := CopyConfig(cfg)
		cpcfg.SendTimeoutMsec = 1000
		worker, err := NewWorker(waddr, cpcfg)
		if err != nil {
			panic(err)
		}
		worker.SetServer(cpcfg.JservAddr, cpcfg)

		Vprintf("[pid %d] worker instantiated, asking for work. Nnsock: %#v\n", os.Getpid(), worker.Nnsock)

		worker.StandaloneExeStart()
		//<-worker.Done

	case isDeafWorker:
		waddr := GenAddress()
		worker, err := NewWorker(waddr, cfg)
		if err != nil {
			panic(err)
		}
		worker.SetServer(cfg.JservAddr, cfg)

		Vprintf("[pid %d] worker instantiated, asking for work. Nnsock: %#v\n", os.Getpid(), worker.Nnsock)

		worker.StandaloneExeStart()

	case isKill:
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "error in kill invocation. Expected: %s kill {jobid}, but jobid is missing.\n", GoqExeName)
			os.Exit(1)
		}
		jid, err := strconv.ParseInt(os.Args[2], 10, 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error in kill invocation. Expected: %s kill {jobid}, but jobid is not numeric.\n", GoqExeName)
			os.Exit(1)
		}

		SendKill(cfg, jid)
		//fmt.Printf("[pid %d] sent kill %d request to jobserver at '%s'.\n", pid, jid, cfg.JservAddr)

	case isShutdown:
		SendShutdown(cfg)
		fmt.Printf("[pid %d] sent shutdown request to jobserver at '%s'.\n", pid, cfg.JservAddr)

	case isWait:
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "error in wait invocation. Expected: %s wait {jobid}, but jobid is missing.\n", GoqExeName)
			os.Exit(1)
		}
		jid, err := strconv.Atoi(os.Args[2])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error in wait invocation. Expected: %s wait {jobid}, but jobid is not numeric.\n", GoqExeName)
			os.Exit(1)
		}
		sub, err := NewSubmitter(GenAddress(), cfg, true) // true to wait forever for it (no timeout)
		if err != nil {
			panic(err)
		}
		fmt.Printf("[pid %d; %s] waiting for jobid %d to finish at server '%s'.\n", pid, sub.Addr, jid, cfg.JservAddr)

		waitchan, err := sub.WaitForJob(int64(jid))
		if err != nil {
			panic(err)
		}
		waitres := <-waitchan
		if waitres.Id == -1 {
			if len(waitres.Out) > 0 {
				fmt.Printf("[pid %d] wait on jobid %d result: error while waiting to finish: %#v.\n", pid, jid, waitres.Out[0])
			} else {
				fmt.Printf("[pid %d] wait on jobid %d result: error while waiting to finish.\n", pid, jid)
			}
			os.Exit(1)
		}
		if waitres.Msg == schema.JOBMSG_JOBNOTKNOWN {
			fmt.Printf("[pid %d] wait on jobid %d result: error: server says jobid-unknown.\n", pid, jid)
			os.Exit(1)
		}
		if waitres.Msg == schema.JOBMSG_JOBFINISHEDNOTICE {
			fmt.Printf("[pid %d] wait on jobid %d result: success, job was completed.\n", pid, jid)
			os.Exit(0)
		}
		fmt.Printf("[pid %d] wait on jobid %d result: done with unrecognized Msg code: %#v.\n", pid, jid, waitres)
		os.Exit(1)

	case isStat:
		sub, err := NewSubmitter(GenAddress(), cfg, false)
		if err != nil {
			panic(err)
		}

		o, err := sub.SubmitSnapJob()
		if err != nil {
			fmt.Printf("[pid %d] error while trying to get stats from server '%s': %s\n", pid, cfg.JservAddr, err)
			os.Exit(1)
		}

		fmt.Printf("[pid %d] stats for job server '%s':\n", pid, cfg.JservAddr)
		for i := range o {
			fmt.Printf("%s\n", o[i])
		}

	default:
		fmt.Printf("err: only recognized goq commands: serve, sub, work, kill, stat, clusterid, wait\n")
		os.Exit(1)
	}

	Vprintf("[pid %d] done.\n", pid)
}

func MkPullNN(addr string, cfg *Config, infWait bool) (*nn.Socket, error) {
	pull1, err := nn.NewSocket(nn.AF_SP, nn.PULL)
	//LogOpen(pull1)

	if err != nil {
		panic(err)
		return nil, err
	}

	if bound, err := IsAlreadyBound(addr); bound {
		panic(fmt.Errorf("problem in MkpullNN: address (%s) is already bound: %s", addr, err))
	}

	if !infWait {
		if cfg != nil {
			if cfg.SendTimeoutMsec > 0 {
				err = pull1.SetRecvTimeout(time.Duration(cfg.SendTimeoutMsec) * time.Millisecond)
				if err != nil {
					panic(err)
				}
			}
		}
	}

	err = CheckedBind(addr, pull1)

	if err != nil {
		fmt.Printf("could not bind addr '%s': %v", addr, err)
		panic(err)
		return nil, err
	}
	//Vprintf("[pid %d] gozbus: pull socket made at '%s'.\n", os.Getpid(), addr)

	return pull1, nil
}

func MkPushNN(addr string, cfg *Config, infWait bool) (*nn.Socket, error) {
	push1, err := nn.NewSocket(nn.AF_SP, nn.PUSH)
	//LogOpen(push1)
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
		Vprintf("could not bind addr '%s': %v", addr, err)
		return nil, err
	}
	//Vprintf("[pid %d] gozbus: push socket made at '%s'.\n", os.Getpid(), addr)

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

func LogOpen(sock *nn.Socket) {
	fmt.Fprintf(opencloseLog, "open %p\n", sock)
	opencloseLog.Sync()
}

func LogRecv(sock *nn.Socket) {
	fmt.Fprintf(opencloseLog, "recv %p\n", sock)
	opencloseLog.Sync()
}

func LogSend(sock *nn.Socket) {
	fmt.Fprintf(opencloseLog, "send %p\n", sock)
	opencloseLog.Sync()
}

func LogClose(sock *nn.Socket) {
	fmt.Fprintf(opencloseLog, "close %p\n", sock)
	opencloseLog.Sync()
}

var opencloseLog *os.File

func startLog() {
	var err error
	opencloseLog, err = os.Create(fmt.Sprintf("opencloseLog.pid%d", os.Getpid()))
	if err != nil {
		panic(err)
	}
}

func closeLog() {
	opencloseLog.Close()
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

	return sub.SubmitSnapJob()
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
	return sleeps
}
