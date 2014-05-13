package main

// copyright(c) 2014, Jason E. Aten
//
// goq : a simple queueing system in go; qsub replacement.
//

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"

	capn "github.com/glycerine/go-capnproto"
	schema "github.com/glycerine/goq/schema"
	nn "github.com/op/go-nanomsg"
)

var JSERV_ADDR string = "tcp://127.0.0.1:1776"

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

func NewPushCache(name, addr string) *PushCache {
	p := &PushCache{
		Name: name,
		Addr: addr,
	}

	t, err := MkPushNN(addr, defaultSockOp)
	if err != nil {
		panic(err)
	}

	p.PushSock = t

	return p
}

type Job struct {
	Id         int64
	Cmd        string
	Out        []string
	Host       string
	Stm        int64
	Etm        int64
	Elapsec    int64
	Status     string
	Subtime    int64
	Pid        int64
	Dir        string
	Msg        schema.JobMsg
	Workeraddr string

	Fromname string
	Fromaddr string

	Toname string
	Toaddr string

	Signature string

	// not serialized, just used
	// for routing
	DestinationSocket *nn.Socket
}

var NextJobId int64

func NewJob() *Job {
	j := &Job{
		Id: 0, // only server should assign job.Id, until then, should be 0.
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
	if j.Fromaddr != "" {
		if _, ok := js.Who[j.Fromaddr]; !ok {
			js.Who[j.Fromaddr] = NewPushCache(j.Fromname, j.Fromaddr)
		}
	}

	if j.Toaddr != "" {
		if _, ok := js.Who[j.Toaddr]; !ok {
			js.Who[j.Toaddr] = NewPushCache(j.Toname, j.Toaddr)
		}
	}

}

func (js *JobServ) CloseRegistery() {
	for _, pp := range js.Who {
		if pp.PushSock != nil {
			pp.PushSock.Close()
		}
	}

	if js.Nnsock != nil {
		js.Nnsock.Close()
	}
}

type JobServ struct {
	Name string

	Nnsock *nn.Socket // receive on
	Addr   string

	Submit         chan *Job // submitter sends on, JobServ receives on.
	WorkerReady    chan *Job // worker sends on, JobServ receives on.
	ToWorker       chan *Job // worker receives on, JobServ sends on.
	RunDone        chan *Job // worker sends on, JobServ receives on.
	DeafChan       chan int  // supply CountDeaf, when asked.
	WaitQ          []*Job
	RunQ           map[int64]*Job
	Ctrl           chan control
	Done           chan bool
	WaitingWorkers []*Job
	Pid            int

	// directory of submitters and workers
	Who map[string]*PushCache

	CountDeaf int
	PrevDeaf  int
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
	j.Msg = schema.JOBMSG_INITIALSUBMIT
	js.Submit <- j
	return nil
}

func NewExternalJobServ(addr string) (pid int, err error) {
	//argv := os.Argv()
	cmd := exec.Command(GoqExeName, "serve")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Start()

	// reap so we don't zombify, which makes
	// it difficult for the test in fetch_test.go to detect that
	// the process is indeed gone. This one liner fixes all that.
	go func() { cmd.Wait() }()

	return cmd.Process.Pid, err
}

func NewJobServ(addr string) (*JobServ, error) {

	var pullsock *nn.Socket
	var err error
	var remote bool
	if addr != "" {
		remote = true
		pullsock, err = MkPullNN(addr, defaultSockOp)
		if err != nil {
			panic(err)
		}

		Vprintf("[pid %d] JobServer bound endpoints addr: '%s'\n", os.Getpid(), addr)
	}

	js := &JobServ{
		Name:           fmt.Sprintf("jobserver.pid.%d", os.Getpid()),
		Addr:           addr,
		Nnsock:         pullsock,
		RunQ:           make(map[int64]*Job),
		Submit:         make(chan *Job),
		WorkerReady:    make(chan *Job),
		ToWorker:       make(chan *Job),
		RunDone:        make(chan *Job),
		DeafChan:       make(chan int),
		Ctrl:           make(chan control),
		Done:           make(chan bool),
		WaitingWorkers: make([]*Job, 0),
		Who:            make(map[string]*PushCache),
		Pid:            os.Getpid(),
	}

	js.Start()
	if remote {
		//Vprintf("remote, server starting ListenForJobs() goroutine.\n")
		fmt.Printf("**** [jobserver pid %d] listening for jobs.\n", js.Pid)
		js.ListenForJobs()
	}

	return js, nil
}

func (js *JobServ) toWorkerChannelIfJobAvail() chan *Job {
	if len(js.WaitQ) == 0 {
		return nil
	}
	return js.ToWorker
}

func (js *JobServ) nextJob() *Job {
	if len(js.WaitQ) == 0 {
		return nil
	}
	js.WaitQ[0].Msg = schema.JOBMSG_DELEGATETOWORKER
	return js.WaitQ[0]
}

func (js *JobServ) JobCleanup(donejob *Job) {
	Vprintf("JobCleanup() called for Job: %#v\n", donejob)
}

func (js *JobServ) IsLocal(j *Job) bool {
	if j.Fromaddr == "" {
		return true
	}
	return false
}

var loopcount int64 = 0

func (js *JobServ) Start() {

	go func() {
		for {
			loopcount++
			Vprintf(" - - - JobServ at top for Start() event loop, loopcount: (%d).\n", loopcount)

			select {
			case newjob := <-js.Submit:
				Vprintf("  === event loop case ===  (%d) JobServ got from Submit channel a newjob, msg: %s, job: %#v\n", loopcount, newjob.Msg, newjob)

				// assign a monotically increasing serial identifier
				newId := NewJobId()

				//in case of re-submit, delete from the runQ
				if newjob.Id != 0 {
					fmt.Printf("**** [jobserver pid %d] got re-submit of job %d, new Id is %d.\n", js.Pid, newjob.Id, newId)
					delete(js.RunQ, newjob.Id)
				}
				newjob.Id = newId

				// open and cache any sockets we will need.
				js.RegisterWho(newjob)

				if newjob.Msg == schema.JOBMSG_SHUTDOWNSERV {
					Vprintf("JobServ got JOBMSG_SHUTDOWNSERV from Submit channel.\n")
					go func() { js.Ctrl <- die }()
					break
				}

				fmt.Printf("**** [jobserver pid %d] got job %d submission. Will run '%s'.\n", js.Pid, newjob.Id, newjob.Cmd)

				if len(js.WaitingWorkers) == 0 {
					js.WaitQ = append(js.WaitQ, newjob)
				} else {
					worker := js.WaitingWorkers[0]
					js.WaitingWorkers = js.WaitingWorkers[1:]
					js.DispatchJobToWorker(worker, newjob) // below
				}

			case reqjob := <-js.WorkerReady:
				Vprintf("  === event loop case === (%d) JobServ got request for work from WorkerReady channel: %#v\n", loopcount, reqjob)
				js.RegisterWho(reqjob)

				if len(js.WaitQ) == 0 {
					js.WaitingWorkers = append(js.WaitingWorkers, reqjob)
				} else {
					job := js.WaitQ[0]
					js.WaitQ = js.WaitQ[1:]
					js.DispatchJobToWorker(reqjob, job) // below
				}

			case donejob := <-js.RunDone:
				Vprintf("  === event loop case === (%d)  JobServ got donejob from RunDone channel: %#v\n", loopcount, donejob)

				_, ok := js.RunQ[donejob.Id]
				if !ok {
					panic(fmt.Sprintf("got donejob %d for job(%#v) from js.RunDone channel, but it was not in our js.RunQ: %#v", donejob.Id, donejob, js.RunQ))
				}
				delete(js.RunQ, donejob.Id)
				fmt.Printf("**** [jobserver pid %d] worker finished job %d, removing from the RunQ\n", js.Pid, donejob.Id)
				js.JobCleanup(donejob)

			case cmd := <-js.Ctrl:
				Vprintf("  === event loop case === (%d)  JobServ got control cmd: %v\n", loopcount, cmd)
				switch cmd {
				case die:
					fmt.Printf("[jobserver pid %d] jobserver exits in response to shutdown request.\n", js.Pid)
					js.CloseRegistery()
					close(js.Done)
					return
				}

			case js.DeafChanIfUpdate() <- js.CountDeaf:
				Vprintf("  === event loop case === (%d)  JobServ supplied js.CountDeaf on channel js.DeafChan.\n", loopcount)
				// only one consumer gets each change; we only send on js.DeafChan once
				// when CountDeaf changes; this prevents our (only) client from busy waiting.
				js.PrevDeaf = js.CountDeaf
			}
		}
	}()
}

func (js *JobServ) DispatchJobToWorker(reqjob, job *Job) {
	job.Msg = schema.JOBMSG_DELEGATETOWORKER

	js.RunQ[job.Id] = job

	if js.IsLocal(reqjob) {
		fmt.Printf("**** [jobserver pid %d] dispatching job %d to local worker.\n", js.Pid, job.Id)

		js.ToWorker <- job
		return
	}

	// to address
	if reqjob.Fromaddr != "" {
		job.Workeraddr = reqjob.Fromaddr

		dest, ok := js.Who[reqjob.Fromaddr]
		if ok {
			job.DestinationSocket = dest.PushSock
		}

		job.Toname = reqjob.Fromname
		job.Toaddr = dest.Addr

		if reqjob.Fromaddr != "" && dest.Addr != reqjob.Fromaddr {
			panic(fmt.Sprintf("mismatch in reqjob.Fromaddr(%s) and dest.Addr(%s) in our cache.", reqjob.Fromaddr, dest.Addr))
		}
	}

	fmt.Printf("**** [jobserver pid %d] dispatching job %d to worker '%s'.\n", js.Pid, job.Id, job.Toaddr)

	// return address
	job.Fromname = js.Name
	job.Fromaddr = js.Addr

	// try to send, give worker 30 seconds to grab it.
	if job.DestinationSocket != nil {
		go func(job *Job) {
			// we can send, go for it. But be on the lookout for timeout, i.e. when worker dies
			// before receiving their job. Then we should just re-queue it.
			err := sendZjob(job.DestinationSocket, job)
			if err != nil {
				// for now assume deaf worker
				fmt.Printf("[pid %d] Got error back trying to dispatch job %d to worker '%s'. Incrementing "+
					"deaf worker count and resubmitting. err: %s\n", os.Getpid(), job.Id, job.Toaddr, err)
				js.CountDeaf++

				job.Msg = schema.JOBMSG_RESUBMITNOACK
				job.Workeraddr = ""
				js.Submit <- job
			} else {
				fmt.Printf("[pid %d] dispatched job %d to worker '%s'\n", os.Getpid(), job.Id, job.Toaddr)
			}
			return
		}(job)
	}
}

// there are 3 actors: submitter, server, and worker.
// The JobServer handles 4 essential types of job messages (marked with ***),
//   and many other acks/side info requests.
//
// Expanded universe: pairs of request/reply. Outlined, but not all are implemented.
//
/*	JOBMSG_INITIALSUBMIT     JobMsg = 0 // *** submitter requests job be queued/started
	JOBMSG_ACKSUBMIT                = 1

	JOBMSG_REQUESTFORWORK           = 2 // *** worker requests a new job (msg and workeraddr only)
	JOBMSG_DELEGATETOWORKER         = 3 // *** worker is sent job with Cmd and Dir filled in.

	JOBMSG_SHUTDOWNWORKER           = 4
	JOBMSG_ACKSHUTDOWNWORKER        = 5

	JOBMSG_FINISHEDWORK             = 6 // *** worker replies with finished job.
	JOBMSG_ACKFINISHED              = 7

	JOBMSG_SHUTDOWNSERV             = 8
	JOBMSG_ACKSHUTDOWNSERV          = 9

	JOBMSG_CANCELWIP                = 10
	JOBMSG_ACKCANCELWIP             = 11

	JOBMSG_CANCELSUBMIT             = 12
	JOBMSG_ACKCANCELSUBMIT          = 13

	JOBMSG_SHOWQUEUES               = 14
	JOBMSG_ACKSHOWQUEUES            = 15

	JOBMSG_SHOWWORKERS              = 16
	JOBMSG_ACKSHOWWORKERS           = 17
*/

func (js *JobServ) ListenForJobs() {
	go func() {
		for {
			// recvZjob blocks, which is why we are in our own goroutine.
			job := recvZjob(js.Nnsock)
			Vprintf("ListenForJobs got * %s * job: %#v\n", job.Msg, job)

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
			default:
				panic(fmt.Sprintf("unrecognized JobMsg: %v", job.Msg))
			}
		}
	}()
}

type Worker struct {

	// remote server
	Name   string
	Addr   string
	Nnsock *nn.Socket // recv

	// or local server
	ToServerRequestWork chan *Job
	ToServerWorkDone    chan *Job
	FromServer          chan *Job

	ToWorker   chan *Job
	FromWorker chan *Job

	Ctrl chan control
	Done chan bool

	ServerName     string
	ServerAddr     string
	ServerPushSock *nn.Socket

	IsDeaf bool
}

func (worker *Worker) SetServer(pushaddr string) {

	var err error
	var pushsock *nn.Socket
	if pushaddr != "" {
		pushsock, err = MkPushNN(pushaddr, defaultSockOp)
		if err != nil {
			panic(err)
		}
		worker.ServerAddr = pushaddr
		worker.ServerPushSock = pushsock
	}
}

func (w *Worker) LocalStart() {
	pid := os.Getpid()

	go func() {
		for {
			select {
			case req := <-w.ToWorker:
				Vprintf("[pid %d] Worker: got request for work on w.ToWorker: %#v, submitting to ToServerRequestWork\n", pid, req)

				req.Msg = schema.JOBMSG_REQUESTFORWORK
				req.Workeraddr = ""
				w.ToServerRequestWork <- req
			case j := <-w.FromServer:
				//Vprintf("Worker: got job on w.FromServer: %#v\n", j)
				Vprintf("[pid %d] worker received job: %#v\n", pid, j)
				w.FromWorker <- j

			case cmd := <-w.Ctrl:
				Vprintf("worker got control cmd: %v\n", cmd)
				switch cmd {
				case die:
					Vprintf("worker dies.\n")
					close(w.Done)
					return
				}

			}
		}
	}()
}

func (w *Worker) StandaloneExeStart() {
	pid := os.Getpid()
	//go func() {
	for {
		select {
		case cmd := <-w.Ctrl:
			Vprintf("[pid %d] worker got control cmd: %v\n", pid, cmd)
			switch cmd {
			case die:
				Vprintf("[pid %d] worker dies.\n", pid)
				close(w.Done)
				return
			}
		default:
			// here is where the main action happens, only
			// after we've given control commands priority.
			w.DoOneJob()
		}
	}
	//}()
}

func (w *Worker) ReportJobDone(donejob *Job) {
	donejob.Msg = schema.JOBMSG_FINISHEDWORK

	if w.Addr == "" {
		w.ToServerWorkDone <- donejob
	} else {
		sendZjob(w.ServerPushSock, donejob)
	}
}

func (w *Worker) FetchJob() *Job {
	var j *Job
	request := NewJob()
	request.Msg = schema.JOBMSG_REQUESTFORWORK
	request.Workeraddr = ""

	request.Fromname = w.Name
	request.Fromaddr = w.Addr
	request.Toname = w.ServerName
	request.Toaddr = w.ServerAddr

	if w.Addr == "" {
		w.ToWorker <- request
		if w.IsDeaf {
			return nil
		}
		j = <-w.FromWorker

	} else {
		if w.IsDeaf {
			// have to close out our socket or
			// else the servers reply will succeed
			// and just stay buffered in the nanomsg queues
			// under the covers. We want to simulate
			// the worker failing and thus his nanomsg queue
			// vanishing.
			err := w.Nnsock.Close()
			if err != nil {
				panic(err)
			}
			fmt.Printf("[pid %d] deaf worker closed worker.Nnsock before sending request for job to server.\n", os.Getpid())
			sendZjob(w.ServerPushSock, request)

			return nil
		} else {
			// non-deaf worker:
			sendZjob(w.ServerPushSock, request)
			j = recvZjob(w.Nnsock)
		}
	}

	return j
}

func (w *Worker) DoOneJob() (*Job, error) {
	// fetch
	j := w.FetchJob()
	if w.IsDeaf {
		return nil, nil
	}

	fmt.Printf("---- [worker pid %d; %s] starting job %d: '%s'\n", os.Getpid(), j.Toaddr, j.Id, j.Cmd)

	// shepard
	o, err := Shepard(j.Dir, j.Cmd)
	j.Out = o

	//fmt.Printf("---- [worker pid %d] done with job %d output: '%#v'\n", os.Getpid(), j.Id, o)
	fmt.Printf("---- [worker pid %d; %s] done with job %d: '%s'\n", os.Getpid(), j.Toaddr, j.Id, j.Cmd)

	// tell server we are done
	w.ReportJobDone(j)

	// return
	return j, err
}

func NewWorker(pulladdr string) (*Worker, error) {
	var err error
	var pullsock *nn.Socket
	if pulladdr != "" {
		pullsock, err = MkPullNN(pulladdr, defaultSockOp)
		if err != nil {
			panic(err)
		}

	}
	w := &Worker{
		Name:   fmt.Sprintf("worker.pid.%d", os.Getpid()),
		Addr:   pulladdr,
		Nnsock: pullsock,
		Done:   make(chan bool),
		Ctrl:   make(chan control),
	}
	return w, nil
}

func NewLocalWorker(js *JobServ) (*Worker, error) {
	w := &Worker{
		ToServerRequestWork: js.WorkerReady,
		ToServerWorkDone:    js.RunDone,
		FromServer:          js.ToWorker, // worker receives on
		ToWorker:            make(chan *Job),
		FromWorker:          make(chan *Job),
	}
	w.LocalStart()
	return w, nil
}

func recvMsgOnZBus(nnzbus *nn.Socket) {
	pid := os.Getpid()

	// receive, synchronously so flags == 0
	var flags int = 0
	heardBuf, err := nnzbus.Recv(flags)
	if err != nil {
		panic(err)
	}

	Vprintf("[pid %d] gozbus server: I heard: '%s'.\n", pid, heardBuf)
}

func sendZjob(nnzbus *nn.Socket, j *Job) error {

	// Create Zjob and Write to nnzbus.
	buf, _ := JobToCapnp(j)
	_, err := nnzbus.Send(buf.Bytes(), 0)
	return err

}

func JobToCapnp(j *Job) (bytes.Buffer, *capn.Segment) {
	seg := capn.NewBuffer(nil)
	z := schema.NewRootZ(seg)
	zjob := schema.NewZjob(seg)

	zjob.SetCmd(j.Cmd)

	tl := seg.NewTextList(len(j.Out))
	for i := range j.Out {
		tl.Set(i, j.Out[i])
	}
	zjob.SetOut(tl)
	zjob.SetHost(j.Host)

	zjob.SetStm(int64(j.Stm))
	zjob.SetEtm(int64(j.Etm))
	zjob.SetElapsec(j.Elapsec)

	zjob.SetStatus(j.Status)
	zjob.SetSubtime(int64(j.Subtime))
	zjob.SetPid(j.Pid)

	zjob.SetDir(j.Dir)
	zjob.SetMsg(j.Msg)
	zjob.SetWorkeraddr(j.Workeraddr)

	zjob.SetId(j.Id)

	zjob.SetFromname(j.Fromname)
	zjob.SetFromaddr(j.Fromaddr)

	zjob.SetToname(j.Toname)
	zjob.SetToaddr(j.Toaddr)

	zjob.SetSignature(j.Signature)

	z.SetJob(zjob)

	buf := bytes.Buffer{}
	seg.WriteTo(&buf)

	return buf, seg
}

func recvZjob(nnzbus *nn.Socket) *Job {

	// Read job submitted to the server
	myMsg, err := nnzbus.Recv(0)
	if err != nil {
		panic(err)
	}

	buf := bytes.NewBuffer(myMsg)
	job := CapnpToJob(buf)
	return job
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
		Cmd:  zj.Cmd(),
		Out:  zj.Out().ToArray(),
		Host: zj.Host(),
		Stm:  zj.Stm(),

		Etm:     zj.Etm(),
		Elapsec: zj.Elapsec(),
		Status:  zj.Status(),

		Subtime: zj.Subtime(),
		Pid:     zj.Pid(),
		Dir:     zj.Dir(),

		Msg:        zj.Msg(),
		Workeraddr: zj.Workeraddr(),
		Id:         zj.Id(),

		Fromname: zj.Fromname(),
		Fromaddr: zj.Fromaddr(),

		Toname: zj.Toname(),
		Toaddr: zj.Toaddr(),

		Signature: zj.Signature(),
	}

	//Vprintf("[pid %d] recvZjob got Zjob message: %#v\n", os.Getpid(), job)
	return job
}

func MakeTestJob() *Job {
	job := &Job{
		Id:  NewJobId(),
		Cmd: "bin/good.sh",
		//		Out:     []string{"test output", "with", "3lines"},
		Out:     []string{},
		Host:    "testhost",
		Stm:     23,
		Etm:     27,
		Elapsec: 4,
		Status:  "okay",
		Subtime: 99,
		Pid:     1011,
		//Dir:     "testcurwd",
		Dir: "",
	}
	return job
}

var Cfg *Config

func init() {
	// do this outside of main so that tests get the
	// proper config as well.
	Cfg = GetEnvConfig()
	JSERV_ADDR = Cfg.JservAddr
}

func main() {
	pid := os.Getpid()

	var isServer bool
	if len(os.Args) > 1 && os.Args[1] == "serve" {
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

	var isStat bool
	if len(os.Args) > 1 && os.Args[1] == "stat" {
		isStat = true
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

	switch {
	case isClusterid:
		fmt.Printf("%s\n", Cfg.ClusterId)
		os.Exit(0)

	case isServer:
		// server code, binds the bus to start it.
		Vprintf("[pid %d] making new external job server, listening on %s\n", pid, JSERV_ADDR)

		// report to a log file too, so we aren't blind.
		/*
			file, err := os.Create("server.out-goq")
			if err == nil {
				fmt.Fprintf(file, "[pid %d] %v : making new external job server, listening on %s\n", pid, time.Now(), JSERV_ADDR)
				file.Close()
			} else {
				panic(err)
			}
		*/
		serv, err := NewJobServ(JSERV_ADDR)
		if err != nil {
			panic(err)
		}

		Vprintf("[pid %d] job server made, now handling requests.\n", pid)
		// wait till done, serving requests
		<-serv.Done

	case isSubmitter:
		subaddr := GenAddress()
		sub, err := NewSubmitter(subaddr)
		if err != nil {
			panic(err)
		}
		sub.SetServer(JSERV_ADDR)
		testjob := MakeTestJob()
		Vprintf("[pid %d] submitter instantiated, make testjob to submit over nanomsg: %#v.\n", pid, testjob)

		sub.SubmitJob(testjob)
		Vprintf("[pid %d] submitted test job to server over nanomsg: %#v.\n", pid, testjob)

	case isWorker:
		// client code, connects to the bus.
		waddr := GenAddress()
		worker, err := NewWorker(waddr)
		if err != nil {
			panic(err)
		}
		worker.SetServer(JSERV_ADDR)

		Vprintf("[pid %d] worker instantiated, asking for work. Nnsock: %#v\n", os.Getpid(), worker.Nnsock)

		worker.StandaloneExeStart()
		//<-worker.Done

	case isDeafWorker:
		waddr := GenAddress()
		worker, err := NewWorker(waddr)
		if err != nil {
			panic(err)
		}
		worker.SetServer(JSERV_ADDR)

		Vprintf("[pid %d] worker instantiated, asking for work. Nnsock: %#v\n", os.Getpid(), worker.Nnsock)

		worker.StandaloneExeStart()

	case isKill:
		SendShutdown(JSERV_ADDR)
		fmt.Printf("[pid %d] sent shutdown request to jobserver at '%s'.\n", pid, JSERV_ADDR)

	case isStat:
		sub, err := NewSubmitter(GenAddress())
		if err != nil {
			panic(err)
		}
		sub.SetServer(JSERV_ADDR)

		o := sub.SubmitStatJob()

		fmt.Printf("[pid %d] stats for job server '%s':\n", pid, JSERV_ADDR)
		for i := range o {
			fmt.Printf("%s\n", o[i])
		}

	default:
		fmt.Printf("err: only recognized goq commands: serve, sub, work, kill, stat, clusterid\n")
		os.Exit(1)
	}

	Vprintf("[pid %d] done.\n", pid)
}

type Submitter struct {
	Name   string
	Addr   string
	Nnsock *nn.Socket

	ServerName     string
	ServerAddr     string
	ServerPushSock *nn.Socket

	ToServerSubmit chan *Job
}

func (sub *Submitter) SubmitJob(j *Job) {
	j.Msg = schema.JOBMSG_INITIALSUBMIT
	//j.FromAddr = sub.Addr
	//j.FromName = "Submitter"
	if sub.Addr != "" {
		sendZjob(sub.ServerPushSock, j)
	} else {
		sub.ToServerSubmit <- j
	}
}

func (sub *Submitter) SubmitShutdownJob() {
	j := NewJob()
	j.Msg = schema.JOBMSG_SHUTDOWNSERV
	j.Fromname = sub.Name
	j.Fromaddr = sub.Addr
	j.Toname = sub.ServerName
	j.Toaddr = sub.ServerAddr

	if sub.Addr != "" {
		sendZjob(sub.ServerPushSock, j)
	} else {
		sub.ToServerSubmit <- j
	}
}

func (sub *Submitter) SubmitStatJob() []string {
	j := NewJob()
	j.Msg = schema.JOBMSG_SHOWWORKERS
	j.Fromname = sub.Name
	j.Fromaddr = sub.Addr
	j.Toname = sub.ServerName
	j.Toaddr = sub.ServerAddr

	if sub.Addr != "" {
		sendZjob(sub.ServerPushSock, j)
		jstat := recvZjob(sub.Nnsock)
		return jstat.Out
	} else {
		fmt.Printf("local server stat not implemented.\n")
		//sub.ToServerSubmit <- j
	}
	return []string{}
}

func (sub *Submitter) SetServer(pushaddr string) {

	var err error
	var pushsock *nn.Socket
	if pushaddr != "" {
		pushsock, err = MkPushNN(pushaddr, defaultSockOp)
		if err != nil {
			panic(err)
		}
		sub.ServerAddr = pushaddr
		sub.ServerPushSock = pushsock
		sub.ServerName = "JSERV"
	}
}

func NewLocalSubmitter(js *JobServ) (*Submitter, error) {
	sub := &Submitter{
		ToServerSubmit: js.Submit,
	}
	return sub, nil
}

func NewSubmitter(pulladdr string) (*Submitter, error) {

	var err error
	var pullsock *nn.Socket
	if pulladdr != "" {
		pullsock, err = MkPullNN(pulladdr, defaultSockOp)
		if err != nil {
			panic(err)
		}
	}
	sub := &Submitter{
		Name:   fmt.Sprintf("submitter.pid.%d", os.Getpid()),
		Addr:   pulladdr,
		Nnsock: pullsock,
	}

	return sub, nil
}

func MkPullNN(addr string, op *SockOption) (*nn.Socket, error) {
	pull1, err := nn.NewSocket(nn.AF_SP, nn.PULL)
	if err != nil {
		panic(err)
		return nil, err
	}

	_, err = pull1.Bind(addr)
	if err != nil {
		fmt.Printf("could not bind addr '%s': %v", addr, err)
		panic(err)
		return nil, err
	}
	//Vprintf("[pid %d] gozbus: pull socket made at '%s'.\n", os.Getpid(), addr)

	return pull1, nil
}

type SockOption struct {
	SendTimeoutMsec time.Duration
}

var defaultSockOp = &SockOption{
	// by default, sends have a 30 second timeout
	// on them, after which they return EAGAIN
	SendTimeoutMsec: 30 * time.Second,
}

func init() {
	setSendTimeoutDefaultFromEnv()
}

func setSendTimeoutDefaultFromEnv() {
	// set send timeout default from env
	to := os.Getenv("GOQ_SENDTIMEOUT_MSEC")
	if to != "" {
		toi, err := strconv.Atoi(to)
		if err == nil {
			defaultSockOp.SendTimeoutMsec = time.Duration(toi) * time.Millisecond
		} else {
			panic(fmt.Sprintf("bad GOQ_SENDTIMEOUT_MSEC value found in environment (err: %s), aborting.", to, err))
		}
	}
}

func MkPushNN(addr string, op *SockOption) (*nn.Socket, error) {
	push1, err := nn.NewSocket(nn.AF_SP, nn.PUSH)
	if err != nil {
		return nil, err
	}

	if op != nil {
		if op.SendTimeoutMsec >= 0 {
			err = push1.SetSendTimeout(op.SendTimeoutMsec)
			if err != nil {
				panic(err)
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
func SendShutdown(addr string) {
	sock, err := MkPushNN(addr, defaultSockOp)
	if err != nil {
		panic(err)
	}
	j := NewJob()
	j.Msg = schema.JOBMSG_SHUTDOWNSERV
	j.Fromname = "SendShutdown"
	j.Toname = "JSERV"
	j.Toaddr = addr

	sendZjob(sock, j)
}
