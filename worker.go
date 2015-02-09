package main

import (
	"fmt"
	"os"

	schema "github.com/glycerine/goq/schema"
	//nn "github.com/glycerine/go-nanomsg"
	nn "github.com/glycerine/mangos/compat"
)

// encapsulate the state that only NanomsgListener go routine should be touching
type NanoRecv struct {
	Addr   string
	Nnsock *nn.Socket // recv

	NanomsgRecv chan *Job
	Nanoerr     chan error

	// set Cfg *once*, before any goroutines start, then
	// treat it as immutable and never changing.
	Cfg  Config
	Ctrl chan control
	Done chan bool
	Deaf bool // simulate worker failure during testing.

	MonitorRecv chan bool // instrumentation for testing, possible nil
}

func NewNanoRecv(pulladdr string, cfg *Config, deaf bool) *NanoRecv {

	var err error
	var pullsock *nn.Socket
	if !deaf {
		if pulladdr != "" {
			pullsock, err = MkPullNN(pulladdr, cfg, false)
			if err != nil {
				panic(err)
			}
		}
	}

	n := &NanoRecv{
		Addr:        pulladdr,
		Nnsock:      pullsock,
		NanomsgRecv: make(chan *Job),
		Nanoerr:     make(chan error),
		Cfg:         *CopyConfig(cfg),
		Ctrl:        make(chan control),
		Done:        make(chan bool),
		Deaf:        deaf,
	}

	return n
}

// encapuslate the sending to server socket
type NanoSender struct {
	ServerAddr     string
	ServerPushSock *nn.Socket

	Ctrl chan control
	Done chan bool

	AckToServer  chan *Job
	ReconnectSrv chan string // send the receiver address for proper logging, and we'll reconnect to server.

	// set Cfg *once*, before any goroutines start, then
	// treat it as immutable and never changing.
	Cfg               Config
	LastHeardRecvAddr string

	MonitorSend chan bool // instrumentation for testing, possible nil
}

func NewNanoSender(cfg *Config, recvaddr string) *NanoSender {
	n := &NanoSender{
		Ctrl:              make(chan control),
		Done:              make(chan bool),
		AckToServer:       make(chan *Job, 100),
		ReconnectSrv:      make(chan string),
		Cfg:               *CopyConfig(cfg),
		LastHeardRecvAddr: recvaddr,
	}

	n.setServerPrivate(cfg.JservAddr())
	return n
}

func (n *NanoSender) setServerPrivate(pushaddr string) {

	var err error
	var pushsock *nn.Socket
	if pushaddr != "" {
		pushsock, err = MkPushNN(pushaddr, &n.Cfg, false)
		if err != nil {
			panic(err)
		}
		n.ServerAddr = pushaddr
		if n.ServerPushSock != nil {
			n.ServerPushSock.Close()
		}
		n.ServerPushSock = pushsock
	}
}

// Worker represents a process that is willing to do work
// for the server. It asks for jobs with JOBMSG_REQUESTFORWORK.
type Worker struct {
	NR *NanoRecv
	NS *NanoSender

	Name       string
	Addr       string // copy of same held in NR, to avoid data race.
	ServerAddr string // copy of same held in NS, to avoid data race.

	// test/external client monitoring, can be nil
	MonitorShepJobStart chan bool
	MonitorShepJobDone  chan bool

	ShepSaysJobStarted chan int  // send pid on it, or 0 if error.
	ShepSaysJobDone    chan *Job // cancels come back here too

	DoSingleJob       chan bool
	ServerReconNeeded chan string
	JobFinished       chan *Job // local clients can wait for a finished job here.
	DoneQ             []*Job

	TellShepPidKilled       chan int // speed up Shepard detected process kill.
	ShutdownSequenceStarted chan bool

	Ctrl chan control
	Done chan bool
	WorkOpts

	// job-running state: three possible job states
	//  (or, how to shutdown cleanly without crashing on a race condition).
	//
	//   a) W.RunningJob == nil, or no shepard started.
	//   b) w.RunningJob set to non-nil but w.Pid == 0: gotta wait for both ShepSaysJobStarted, then ShepSaysJobDone.
	//   c) w.RunningJob set to non-nil and w.Pid set to non-zero: gotta wait for ShepSaysJobDone.
	//
	// all of these are cleared by receipt on w.ShepSaysJobDone
	RunningJid int64 // filled by JOBMSG_DELEGATETOWORKER handler
	RunningJob *Job  // filled by JOBMSG_DELEGATETOWORKER handler
	Pid        int   // filled by receipt of w.ShepSaysJobStarted

	// set Cfg *once*, before any goroutines start, then
	// treat it as immutable and never changing.
	Cfg           Config
	NoReplay      *NonceRegistry
	BadNonceCount int64
}

type WorkOpts struct {
	Forever   bool
	IsDeaf    bool
	Monitor   bool
	DontStart bool // for testing shepard in isolation, shep_test.go
}

func NewWorker(pulladdr string, cfg *Config, opts *WorkOpts) (*Worker, error) {

	if opts == nil {
		opts = &WorkOpts{}
	}

	w := &Worker{
		Addr:       pulladdr,
		ServerAddr: cfg.JservAddr(),
		WorkOpts:   *opts,
		NR:         NewNanoRecv(pulladdr, cfg, opts.IsDeaf),
		NS:         NewNanoSender(cfg, pulladdr),
		Name:       fmt.Sprintf("worker.pid.%d", os.Getpid()),
		Done:       make(chan bool),
		Ctrl:       make(chan control),
		Cfg:        *CopyConfig(cfg),

		ShepSaysJobStarted: make(chan int),
		ShepSaysJobDone:    make(chan *Job),
		TellShepPidKilled:  make(chan int, 1),

		DoSingleJob:       make(chan bool),
		ServerReconNeeded: make(chan string),
		JobFinished:       make(chan *Job),
		DoneQ:             make([]*Job, 0),

		ShutdownSequenceStarted: make(chan bool),
		NoReplay:                NewNonceRegistry(NewRealTimeSource()),
	}

	if opts.Monitor {
		w.NR.MonitorRecv = make(chan bool)
		w.NS.MonitorSend = make(chan bool)
		w.MonitorShepJobStart = make(chan bool)
		w.MonitorShepJobDone = make(chan bool)
	}

	if !opts.DontStart {
		w.Start()
	}

	return w, nil
}

func (w *Worker) StandaloneExeStart() {
	pid := os.Getpid()
	if len(os.Args) >= 3 {
		if os.Args[2] == "forever" {
			w.Forever = true

			TSPrintf("---- [worker pid %d; %s] looping forever, looking for work every %d msec from server '%s'\n", pid, w.Addr, w.Cfg.SendTimeoutMsec, w.ServerAddr)
		}
	} else {
		TSPrintf("---- [worker pid %d; %s] doing one job for server: '%s'\n", pid, w.Addr, w.ServerAddr)
	}

	for {
		w.DoOneJob()

		if !w.Forever {
			break
		}
	}

	// all done
	w.Destroy()
}

func CopyJobWithMsg(job *Job, msg schema.JobMsg) *Job {
	j := *job
	j.Msg = msg
	return &j
}
