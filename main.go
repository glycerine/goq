package main

// copyright(c) 2014, Jason E. Aten
//
// goq : a simple queueing system in go; qsub replacement.
//

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	schema "github.com/glycerine/goq/schema"
)

var timeoutRx = regexp.MustCompile("resource temporarily unavailable")

var LASTGITCOMMITHASH string

func main() {

	pid := os.Getpid()
	home, err := FindGoqHome()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fatal error: please set env var GOQ_HOME to point to your Goq installation: %s\n", err)
		os.Exit(1)
	}

	//ShowRlimit()
	//fmt.Printf("errno = %p\n", unsafe.Pointer(GetAddrErrno()))

	/*
		// debug, SIGQUIT -> stacktrace
		sigChan := make(chan os.Signal)
		go func() {
			stacktrace := make([]byte, 8192)
			for _ = range sigChan {
				length := runtime.Stack(stacktrace, true)
				fmt.Println(string(stacktrace[:length]))
			}
		}()
		signal.Notify(sigChan, syscall.SIGQUIT)
	*/

	if len(os.Args) > 1 && (os.Args[1] == "version" || os.Args[1] == "--version") {
		fmt.Printf("%s\n", goq_version())
		os.Exit(0)
	}

	var isServer bool
	if len(os.Args) > 1 && (os.Args[1] == "serve" || os.Args[1] == "server") {
		isServer = true
	}

	var isInit bool
	if len(os.Args) > 1 && os.Args[1] == "init" {
		isInit = true
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
	var maxShow int = 10
	if len(os.Args) > 1 && os.Args[1] == "stat" {
		isStat = true
		if len(os.Args) > 2 {
			m, err := strconv.Atoi(os.Args[2])
			if err == nil {
				maxShow = m
				//fmt.Printf("main sub is setting maxShow = %d\n", maxShow)
			} else {
				fmt.Fprintf(os.Stderr, "%s sub could not parse stat maxShow argument '%s', err = %v\n", GoqExeName, os.Args[2], err)
				os.Exit(1)
			}
		}
	}

	var isWait bool
	if len(os.Args) > 1 && os.Args[1] == "wait" {
		isWait = true
	}

	// Brutally tell all workers to kill themselves off. In firey smoke.
	var isImmo bool
	if len(os.Args) > 1 && os.Args[1] == "immolateworkers" {
		isImmo = true
	}

	// deafWorker is for testing the behavior
	// of the jobserver when the worker dies or
	// doesn't answer after requesting a job.
	var isDeafWorker bool
	if len(os.Args) > 1 && os.Args[1] == "deafworker" {
		isDeafWorker = true
	}

	cfg, err := DiskThenEnvConfig(home)
	if !isInit {
		if err != nil {
			fmt.Fprintf(os.Stderr, "[pid %d] error on trying to read GOQ_HOME dir %s/.goq: '%s'. Did you forget to do 'goq init' ?\n", pid, home, err)
			os.Exit(1)
		}
	}

	switch {
	case isInit:
		if KeyExists(cfg) {
			fmt.Printf("[pid %d] goq init: key already exists in '%s'; delete .goq manually if you want to re-init. Warning: you will have to redistribute the .goq auth creds to your cluster.\n", pid, cfg.Home+"/.goq")
			os.Exit(1)
		}
		ServerInit(cfg)
		fmt.Printf("[pid %d] goq init: key created in '%s'.\n", pid, cfg.Home+"/.goq")
		os.Exit(0)

	case isServer:
		VPrintf("[pid %d] making new external job server, listening on %s:%d\n", pid, cfg.JservIP, cfg.JservPort)

		serv, err := NewJobServ(cfg)
		if err != nil {
			panic(err)
		}

		VPrintf("[pid %d] job server made, now handling requests.\n", pid)
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
		// try really hard to cleanup, so no leftover sockets to fill up our file handle table.
		// It is okay to call sub.Bye() more than once.
		defer sub.Bye()
		todojob := MakeActualJob(args, cfg)
		VPrintf("[pid %d] submitter instantiated, make testjob to submit over nanomsg: %s.\n", pid, todojob)

		reply, err := sub.SubmitJobGetReply(todojob)
		if err != nil {
			match := timeoutRx.FindStringSubmatch(err.Error())
			if match != nil {
				fmt.Printf("[pid %d] sub timed-out after %d msec trying to contact server at '%s'.\n", pid, cfg.SendTimeoutMsec, cfg.JservAddr())
				sub.Bye()
				os.Exit(1)
			}
			fmt.Printf("[pid %d] goq sub: unknown error trying to contact server at '%s': '%s'.\n", pid, cfg.JservAddr(), err)
			sub.Bye()
			os.Exit(1)
		}
		if reply.Aboutjid != 0 {
			fmt.Printf("[pid %d] submitted job %d to server at '%s'.\n", pid, reply.Aboutjid, cfg.JservAddr())
			sub.Bye()
			os.Exit(0)
		}
		fmt.Printf("[pid %d] submitted job to server over nanomsg, got unexpected '%s' reply: %s.\n", pid, reply.Msg, reply)
		sub.Bye()
		os.Exit(1)

	case isImmo:
		sub, err := NewSubmitter(GenAddress(), cfg, false)
		if err != nil {
			panic(err)
		}

		err = sub.SubmitImmoJob()
		if err != nil {
			fmt.Printf("[pid %d] error while submitting ImmolateWorkers command to server '%s': %s\n", pid, cfg.JservAddr(), err)
			os.Exit(1)
		}

		fmt.Printf("[pid %d] immolate workers command submitted to server '%s':\n", pid, cfg.JservAddr())
		os.Exit(0)

	case isWorker:
		// client code, connects to the bus.
		waddr := GenAddress()

		// set a small, 1 seecond, timeout
		cpcfg := CopyConfig(cfg)
		cpcfg.SendTimeoutMsec = 1000
		worker, err := NewWorker(waddr, cpcfg, nil)
		if err != nil {
			panic(err)
		}

		VPrintf("[pid %d] worker instantiated, asking for work. Nnsock: %#v\n", os.Getpid(), worker.NR.Nnsock)

		worker.StandaloneExeStart()
		//<-worker.Done

	case isDeafWorker:
		waddr := GenAddress()
		worker, err := NewWorker(waddr, cfg, &WorkOpts{IsDeaf: true})
		if err != nil {
			panic(err)
		}

		VPrintf("[pid %d] worker instantiated, asking for work. Nnsock: %#v\n", os.Getpid(), worker.NR.Nnsock)

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
		fmt.Printf("[pid %d] sent kill %d request to jobserver at '%s'.\n", pid, jid, cfg.JservAddr())
		//  (no ack required on kill)

	case isShutdown:
		SendShutdown(cfg)
		fmt.Printf("[pid %d] sent shutdown request to jobserver at '%s'.\n", pid, cfg.JservAddr())

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
		fmt.Printf("[pid %d; %s] waiting for jobid %d to finish at server '%s'.\n", pid, sub.Addr, jid, cfg.JservAddr())

		waitchan, err := sub.WaitForJob(int64(jid))
		if err != nil {
			if strings.HasSuffix(err.Error(), "resource temporarily unavailable\n") {
				fmt.Printf("[pid %d] wait timed-out after %d msec trying to contact server at '%s'.\n", pid, cfg.SendTimeoutMsec, cfg.JservAddr())
				os.Exit(1)
			}
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
		switch waitres.Msg {
		case schema.JOBMSG_JOBNOTKNOWN:
			fmt.Printf("[pid %d] wait on jobid %d result: error: server says jobid-unknown.\n", pid, jid)
			os.Exit(1)

		case schema.JOBMSG_JOBFINISHEDNOTICE:
			fmt.Printf("[pid %d] wait on jobid %d result: success, job was completed.\n", pid, jid)
			os.Exit(0)

		case schema.JOBMSG_CANCELSUBMIT:
			fmt.Printf("[pid %d] wait on jobid %d result: error: job cancelled.\n", pid, jid)
			os.Exit(1)

		default:
			fmt.Printf("[pid %d] wait on jobid %d result: done with unrecognized Msg '%s': %#v.\n", pid, jid, waitres.Msg, waitres)
			os.Exit(1)
		}

	case isStat:
		sub, err := NewSubmitter(GenAddress(), cfg, false)
		if err != nil {
			panic(err)
		}

		o, err := sub.SubmitSnapJob(maxShow)
		if err != nil {
			fmt.Printf("[pid %d] error while trying to get stats from server '%s': %s\n", pid, cfg.JservAddr(), err)
			os.Exit(1)
		}

		fmt.Printf("[pid %d] stats for job server '%s':\n", pid, cfg.JservAddr())
		for i := range o {
			fmt.Printf("%s\n", o[i])
		}

	default:
		fmt.Printf("err: only recognized goq commands: init, sub, work, kill (jobid), stat, wait (jobid), immolateworkers, serve, shutdown\n")
		os.Exit(1)
	}

	VPrintf("[pid %d] done.\n", pid)
}
