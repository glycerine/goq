package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	schema "github.com/glycerine/goq/schema"
)

// encapsulate the state that only NanomsgListener go routine should be touching
type NanoRecv struct {
	Addr string
	Cli  *ClientRpc

	NanomsgRecv           chan *Job
	BounceToNanomsgRecvCh chan *Job
	Nanoerr               chan error

	// set Cfg *once*, before any goroutines start, then
	// treat it as immutable and never changing.
	Cfg  Config
	Ctrl chan control
	Done chan bool
	Deaf bool // simulate worker failure during testing.

	MonitorRecv chan bool // instrumentation for testing, possible nil
	MonitorSend chan bool // instrumentation for testing, possible nil

	ClientReconnect chan bool
}

func NewNanoRecv(cli *ClientRpc, cfg *Config, deaf bool) *NanoRecv {

	if deaf {
		//cli.Close()
	}
	n := &NanoRecv{
		Cli:                   cli,
		NanomsgRecv:           make(chan *Job),
		BounceToNanomsgRecvCh: make(chan *Job),
		Nanoerr:               make(chan error),
		Cfg:                   *CopyConfig(cfg),
		Ctrl:                  make(chan control),
		Done:                  make(chan bool),
		Deaf:                  deaf,
		ClientReconnect:       make(chan bool, 1),
	}

	return n
}

// Worker represents a process that is willing to do work
// for the server. It asks for jobs with JOBMSG_REQUESTFORWORK.
type Worker struct {
	NR *NanoRecv

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
	Cfg Config
}

type WorkOpts struct {
	Forever   bool
	IsDeaf    bool
	Monitor   bool
	DontStart bool // for testing shepard in isolation, shep_test.go
}

func NewWorker(cfg *Config, opts *WorkOpts) (*Worker, error) {

	if opts == nil {
		opts = &WorkOpts{}
	}

	w := &Worker{
		ServerAddr: cfg.JservAddr(),
		WorkOpts:   *opts,
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
	}

	cli, err := NewClientRpc("worker", cfg, false)
	if err != nil {
		return nil, err
	}
	w.NR = NewNanoRecv(cli, cfg, opts.IsDeaf)
	w.Addr = cli.LocalAddr()

	if opts.Monitor {
		w.NR.MonitorRecv = make(chan bool)
		w.NR.MonitorSend = make(chan bool)
		w.MonitorShepJobStart = make(chan bool)
		w.MonitorShepJobDone = make(chan bool)
	}

	if !opts.DontStart {
		w.Start()
	}

	return w, nil
}

func (w *Worker) noticeControlC(f func()) {
	sigChan := make(chan os.Signal)
	go func() {
		for _ = range sigChan {
			f()
		}
	}()
	signal.Notify(sigChan, syscall.SIGINT)
}

func (w *Worker) StandaloneExeStart() {
	pid := os.Getpid()

	// 2024 Oct 28: new default is keep working. This is much more common.
	w.Forever = true
	if len(os.Args) >= 3 && os.Args[2] == "oneshot" {
		w.Forever = false
		AlwaysPrintf("---- [worker pid %d; %s] doing one job for server: '%s'\n", pid, w.Addr, w.ServerAddr)
	} else {
		AlwaysPrintf("---- [worker pid %d; %s] looping forever, looking for work every %d msec from server '%s'\n", pid, w.Addr, w.Cfg.SendTimeoutMsec, w.ServerAddr)
	}

	w.noticeControlC(func() {
		// try to get a quic-go active close sent.
		w.NR.Cli.Close()
		w.Destroy()
	})
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
