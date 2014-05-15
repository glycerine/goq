package main

import (
	"fmt"
	"os"

	schema "causalnetworks.com/goztix"
	nn "github.com/op/go-nanomsg"
)

// Worker represents a process that is willing to do work
// for the server. It asks for jobs with JOBMSG_REQUESTFORWORK.
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

	IsDeaf  bool
	Forever bool
	IsLocal bool

	// set Cfg *once*, before any goroutines start, then
	// treat it as immutable and never changing.
	Cfg Config
}

func (worker *Worker) SetServer(pushaddr string, cfg *Config) {

	var err error
	var pushsock *nn.Socket
	if pushaddr != "" {
		pushsock, err = MkPushNN(pushaddr, cfg, false)
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
				Vprintf("[pid %d; local] Worker: got request for work on chan w.ToWorker: %#v, submitting to ToServerRequestWork\n", pid, req)

				req.Msg = schema.JOBMSG_REQUESTFORWORK
				req.Workeraddr = ""
				w.ToServerRequestWork <- req
			case j := <-w.FromServer:
				//Vprintf("Worker: got job on w.FromServer: %#v\n", j)
				Vprintf("[pid %d; local] worker received job: %#v\n", pid, j)
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
	if len(os.Args) >= 3 {
		if os.Args[2] == "forever" {
			w.Forever = true

			fmt.Printf("---- [worker pid %d; %s] looping forever, looking for work every %d msec from server '%s'\n", os.Getpid(), w.Addr, w.Cfg.SendTimeoutMsec, w.ServerAddr)
		}
	}

	//go func() {
	for {
		select {
		case cmd := <-w.Ctrl:
			Vprintf("[pid %d; %s] worker got control cmd: %v\n", pid, w.Addr, cmd)
			switch cmd {
			case die:
				Vprintf("[pid %d; %s] worker dies.\n", pid, w.Addr)
				close(w.Done)
				return
			}
		default:
			// here is where the main action happens, only
			// after we've given control commands priority.
			_, err := w.DoOneJob()
			if err != nil {
				fmt.Printf("%s\n", err)
				if !w.Forever {
					os.Exit(0)
				}
			}
		}
	}
	//}()
}

func (w *Worker) ReportJobDone(donejob *Job) {
	donejob.Msg = schema.JOBMSG_FINISHEDWORK

	if w.IsLocal {
		w.ToServerWorkDone <- donejob
	} else {
		sendZjob(w.ServerPushSock, donejob, &w.Cfg)
	}
}

func (w *Worker) FetchJob() (*Job, error) {
	var j *Job
	request := NewJob()
	request.Msg = schema.JOBMSG_REQUESTFORWORK
	request.Workeraddr = w.Addr
	request.Serveraddr = w.ServerAddr

	if w.IsLocal {
		w.ToWorker <- request
		if w.IsDeaf {
			return nil, nil
		}
		j = <-w.FromWorker

	} else {
		if w.IsDeaf {
			// have to close out our socket before sending or
			// else the servers reply may get back so fast it will succeed
			// and just stay buffered in the nanomsg queues
			// under the covers. We want to simulate
			// the worker failing and thus his nanomsg queue
			// vanishing.
			err := w.Nnsock.Close()
			if err != nil {
				panic(err)
			}
			fmt.Printf("[pid %d] deaf worker closed worker.Nnsock before sending request for job to server.\n", os.Getpid())
			sendZjob(w.ServerPushSock, request, &w.Cfg)

			return nil, nil
		} else {
			// non-deaf worker:
			err := sendZjob(w.ServerPushSock, request, &w.Cfg)
			if err != nil {
				return nil, fmt.Errorf("send timed out after %d msec: %s.\n", w.Cfg.SendTimeoutMsec, err)
			}
			// implement w.Forever here:
			for {
				j, err = recvZjob(w.Nnsock)
				if err != nil {
					if w.Forever && err.Error() == "resource temporarily unavailable" {
						continue
					}
					return nil, fmt.Errorf("recv timed out after %d msec: %s.\n", w.Cfg.SendTimeoutMsec, err)
				} else {
					return j, nil
				}
			}
		}
	}

	return j, nil
}

func (w *Worker) DoOneJob() (*Job, error) {
	// fetch
	j, err := w.FetchJob()
	if j == nil {
		if err == nil {
			err = fmt.Errorf("") // allow the retuned error to not look crappy.
		}
		//		if !w.Forever {
		return nil, fmt.Errorf("---- [worker pid %d; %s] worker could not fetch job: %s", os.Getpid(), w.Addr, err)
		//		} else {
		// just keep looping, trying once/second (or whatever the timeout on the socket is set to.)
		//			return nil, nil
		//		}
	}
	if w.IsDeaf {
		return nil, nil
	}

	if j.Msg == schema.JOBMSG_REJECTBADSIG {
		errmsg := fmt.Errorf("---- [worker pid %d; %s] work request rejected for bad signature", os.Getpid(), j.Workeraddr)
		return nil, errmsg
	}

	if j.Msg == schema.JOBMSG_DELEGATETOWORKER {
		fmt.Printf("---- [worker pid %d; %s] starting job %d: '%s'\n", os.Getpid(), j.Workeraddr, j.Id, j.Cmd)

		// shepard
		o, err := Shepard(j.Dir, j.Cmd, j.Args, j.Env)
		j.Out = o

		//fmt.Printf("---- [worker pid %d] done with job %d output: '%#v'\n", os.Getpid(), j.Id, o)
		fmt.Printf("---- [worker pid %d; %s] done with job %d: '%s'\n", os.Getpid(), j.Workeraddr, j.Id, j.Cmd)

		// tell server we are done
		w.ReportJobDone(j)

		// return
		return j, err
	}

	return nil, nil
}

func NewWorker(pulladdr string, cfg *Config) (*Worker, error) {
	var err error
	var pullsock *nn.Socket
	if pulladdr != "" {
		pullsock, err = MkPullNN(pulladdr, cfg, false)
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
		Cfg:    *CopyConfig(cfg),
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
		IsLocal:             true,
	}
	w.LocalStart()
	return w, nil
}
