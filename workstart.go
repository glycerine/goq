package main

import (
	"fmt"
	"os"
	//"syscall"
	"time"
)

// set to true for excrutiating amounts of internal detail
var WorkerVerbose bool

func WPrintf(format string, a ...interface{}) {
	if WorkerVerbose {
		TSPrintf(format, a...)
	}
}

func (nr *NanoRecv) ReconnectToServer() error {

	AlwaysPrintf("[pid %d] worker [its been too long] teardown and reconnect to server '%s'\n", os.Getpid(), nr.Cfg.JservAddrNoProto())
	//jea debug: nr.Cli.Close()
	for {
		cli, err := NewClientRpc("workstart", &nr.Cfg, false)
		if err != nil {
			// connection refused if server is down
			AlwaysPrintf("could not create new client. error: '%v'", err)
			time.Sleep(time.Second)
			continue
		}
		nr.Cli = cli
		if len(nr.ClientReconnect) == 0 {
			nr.ClientReconnect <- true
		}
		return nil
	}
}

// does all the receiving (on nanomsg) for Worker
func (nr *NanoRecv) NanomsgListener(reconNeeded chan<- string, w *Worker) {

	if nr.Deaf {
		close(nr.Done)
		return
	}

	go func() {
		pid := os.Getpid()

		var j *Job
		var evercount int
		var err error

		for {
			WPrintf("at top of w.NR.NanomsgListener receive loop.\n\n")
			select { // hung here 2
			case cmd := <-nr.Ctrl:
				WPrintf("[pid %d; %s] worker NanomsgListener() got control cmd: %v. Dying.\n", pid, nr.Addr, cmd)
				close(nr.Done)
				return

			case jb := <-nr.Cli.ReadIncomingCh:
				//if jb == nil {
				//	continue
				//}
				j, err = nr.Cfg.bytesToJob(jb.JobSerz)
				//vv("got msg on ReadIncomingCh, j='%v';err='%v'", j, err)
				if err != nil {
					// probabably: our connection has closed. we have reconnect.
					vv("err after nr.Cli.ReadIncomingCh, doing reconnect...")
					nr.ReconnectToServer()
					continue
				}

			case <-time.After(10 * time.Second):
				if nr.Cli.Cli.IsDown() {
					//vv("client is down, try to reconnect.")
					select {
					case <-w.ShutdownSequenceStarted: // prevent deadlock by having this case
					case reconNeeded <- nr.Addr: // our main goal is to do this.
					}
				} else {
					//vv("client is up, fine. check again in 10 sec...")
				}

			case fwd := <-nr.BounceToNanomsgRecvCh:
				nr.NanomsgRecv <- fwd
			}

			if err == nil && j != nil {
				nr.NanomsgRecv <- j
				evercount = 0
				if nr.MonitorRecv != nil {
					WPrintf("MonitorRecv <- true after receiving j = %s\n", j)
					nr.MonitorRecv <- true
					nr.MonitorRecv = nil // oneshot only
				}
			} else {
				//vv("worker recvZJob returned err='%v'", err)
				// the sends on reconNeeded and nr.Nanoerr will be problematic during shutdown sequence,
				// so include the select over case <-w.ShutdownSequenceStarted to avoid deadlock.
				if err != nil {
					vv("NanomsgListener loop sees err = '%v'", err)
					if err.Error() == "resource temporarily unavailable" {
						evercount++
						if evercount == 60 {
							// hmm, its been 60 timeouts (60 seconds). Tear down the socket
							// and try reconnecting to the server.
							// This allows the server to go down, and we can still reconnect
							// when they come back up.
							WPrintf("[pid %d; %s] worker NanomsgListener sending reconNeeded <- nr.Addr(%s).\n", pid, nr.Addr, nr.Addr)

							select {
							case <-w.ShutdownSequenceStarted: // prevent deadlock by having this case
							case reconNeeded <- nr.Addr: // our main goal is to do this.
							}
							evercount = 0
							continue
						}
					}
					continue
				}
				select {
				case <-w.ShutdownSequenceStarted:
				case nr.Nanoerr <- fmt.Errorf("[pid %d; %s] worker NanomsgListener timed out after %d msec: %s.\n", pid, nr.Addr, nr.Cfg.SendTimeoutMsec, err):
					VPrintf("[pid %d; %s] worker NanomsgListener timed out after %d msec: %s.\n", pid, nr.Addr, nr.Cfg.SendTimeoutMsec, err)
				}
			}

		} // forever
	}()
}

// send communication helpers for Start() to
// send on w.JobFinished
func (w *Worker) maybeJobFinishedCh() chan *Job {
	if len(w.DoneQ) == 0 {
		return nil
	} else {
		return w.JobFinished
	}
}

func (w *Worker) IfDoneQReady() *Job {
	if len(w.DoneQ) == 0 {
		return nil
	} else {
		return w.DoneQ[0]
	}
}

// the main communication loop for the worker
func (w *Worker) Start() {

	// start my sender and my receiver
	w.NR.NanomsgListener(w.ServerReconNeeded, w)

	go func() {
		pid := os.Getpid()
		var j *Job

		for {
			WPrintf(" --------------- 33333   Worker.Start() top of comm loop\n")
			select {
			case w.maybeJobFinishedCh() <- w.IfDoneQReady():
				WPrintf(" --------------- 44444   Worker.Start(): after sending on maybeJobFinished()\n")
				// we had a finished job, and we just sent it to the interested party.
				w.DoneQ = w.DoneQ[1:]

			case recvAddr := <-w.ServerReconNeeded: // from receiver, the addr is just for proper logging at the moment.
				_ = recvAddr
				WPrintf(" --------------- 44444   Worker.Start(): after receiving on w.ServerReconNeeded()\n")
				//for {
				//vv("worker Start() sees w.ServerReconNeeded, doing reconnect...")
				err := w.NR.ReconnectToServer()
				panicOn(err)
				w.Addr = w.NR.Cli.LocalAddr()

				// This is redundant with our outer loop in StandaloneExeStart(),
				// and will result in double submit of job request to server.
				/*
					if w.RunningJob == nil && w.Forever {
						// actively tell server we are still here. Otherwise server may
						// have bounced and forgotten about our request. Requests are idempotent, so
						// duplicate requests from the same Workeraddr are fine.
						err = w.SendRequestForJobToServer()
						if err != nil {
							AlwaysPrintf("error on sending job request: '%v'", err)
							continue
						}
					}
				*/
				//}
			case cmd := <-w.Ctrl:
				WPrintf(" --------------- 44444   Worker.Start(): after receiving <-w.Ctrl()\n")

				switch cmd {
				case die:
					WPrintf("[pid %d; %s] Start() worker got die on Ctrl, dying.\n", pid, w.Addr)
					w.DoShutdownSequence()
					return
				}

			//case addr := <-w.NR.UpdateAddr:
			//	w.Addr = addr

			case <-w.DoSingleJob:
				WPrintf(" --------------- 44444   Worker.Start(): after <-w.DoSingleJob\n")
				w.SendRequestForJobToServer()

				// for testing, e.g. immo_test, needs to know worker is alive
				// before we shut it down.
				if w.NR.MonitorSend != nil {
					//vv("MonitorSend <- true after receiving job '%#v'\n", j)
					w.NR.MonitorSend <- true
					w.NR.MonitorSend = nil // one shot only
				}

			case recverr := <-w.NR.Nanoerr:
				WPrintf(" --------------- 44444   Worker.Start(): after <-w.NR.Nanoerr: %s\n", recverr)
				//AlwaysPrintf("%s\n", recverr) // info: worker is alive, but quiet b/c fills up logs too much.

			case pid := <-w.ShepSaysJobStarted:
				WPrintf(" --------------- 44444   Worker.Start(): after <-w.ShepSaysJobStarted\n")
				WPrintf("worker got <-w.ShepSaysJobStarted\n")
				w.Pid = pid
				w.RunningJob.Stm = time.Now().UnixNano()
				w.RunningJob.Pid = int64(pid)
				if w.MonitorShepJobStart != nil {
					WPrintf("worker just before one-shot MonitorShepJobStart\n")
					w.MonitorShepJobStart <- true
					WPrintf("worker just after one-shot MonitorShepJobStart\n")
					w.MonitorShepJobStart = nil // one shot
				}

			case j = <-w.ShepSaysJobDone:
				WPrintf(" --------------- 44444   Worker.Start(): after <-w.ShepSaysJobDone\n")
				// the j we get back points to a modified copy of w.RunningJob, that
				// now contains the .Output, .Cancelled, and .Pid fields set.
				j.Stm = w.RunningJob.Stm
				j.Etm = time.Now().UnixNano()
				w.TellServerJobFinished(j) // time-outs: 3
				w.DoneQ = append(w.DoneQ, j)
				if w.MonitorShepJobDone != nil {
					WPrintf("worker just before one-shot MonitorShepJobDone\n")
					w.MonitorShepJobDone <- true
					WPrintf("worker just after one-shot MonitorShepJobDone\n")
					w.MonitorShepJobDone = nil // one shot
				}

			case j = <-w.NR.NanomsgRecv:
				WPrintf(" --------------- 44444   Worker.Start(): after <-w.NR.NanomsgRecv, j: '%#v'\n", j)

				switch j.Msg {
				case JOBMSG_REJECTBADSIG:
					AlwaysPrintf("---- [worker pid %d; %s] work request rejected for bad signature", pid, j.Workeraddr)

				case JOBMSG_DELEGATETOWORKER:
					AlwaysPrintf("---- [worker pid %d; %s] starting job %d: '%s' in dir '%s'\n", pid, j.Workeraddr, j.Id, j.Cmd, j.Dir)

					if w.NR.MonitorSend != nil {
						WPrintf("MonitorSend <- true after receiving job '%#v'\n", j)
						w.NR.MonitorSend <- true
						w.NR.MonitorSend = nil // one shot only
					}

					w.RunningJob = j
					w.RunningJid = j.Id

					// shepard
					// add in group and array id
					j.Env = append(j.Env, fmt.Sprintf("GOQ_ARRAY_ID=%d", j.ArrayId)) // 0 by default
					j.Env = append(j.Env, fmt.Sprintf("GOQ_GROUP_ID=%d", j.GroupId)) // 0 by default
					//AlwaysPrintf("j.Env = %#v\n", j.Env)
					//AlwaysPrintf("j.Dir = %#v\n", j.Dir)

					// shepard will take off in its own goroutine, communicating
					// back over ShepSaysJobStarted, ShepSaysJobDone (done or cancelled both come back on ShepSaysJobDone).
					w.Shepard(j)

				case JOBMSG_SHUTDOWNWORKER:
					// ack to server
					WPrintf("at case JOBMSG_SHUTDOWNWORKER: j = %#v\n", j)
					w.NR.Cli.AsyncSend(CopyJobWithMsg(j, JOBMSG_ACKSHUTDOWNWORKER))
					AlwaysPrintf("---- [worker pid %d; %s] got 'shutdownworker' request from '%s'. Vanishing in a puff of smoke.\n",
						pid, j.Workeraddr, j.Serveraddr)
					w.DoShutdownSequence() // return must follow immediately, since we've close(w.Done) already
					return                 // terminate Start()
					// don't do Exit(0), in case we are in a goroutine local to the test process

				case JOBMSG_CANCELWIP:
					w.KillRunningJob(true)

				case JOBMSG_PINGWORKER:
					// server is asking if we are still working on it/alive.
					if w.RunningJob != nil {
						j.Aboutjid = w.RunningJob.Id
						j.Stm = w.RunningJob.Stm
						j.Pid = w.RunningJob.Pid
					} else {
						j.Aboutjid = 0
					}
					WPrintf("---- [worker pid %d; %s] got 'pingworker' from server '%s'. Aboutjid: %d\n",
						pid, j.Workeraddr, j.Serveraddr, j.Aboutjid)
					j.Msg = JOBMSG_ACKPINGWORKER
					w.NR.Cli.AsyncSend(j)

				case JOBMSG_JOBFINISHEDNOTICE:
					// should we do anything about this? should we get this at all?
					//  ... maybe if we were already running the same job id and could stop early?
					// unrecognized message '&Job{Id:0, Msg:jobfinishednotice,
					// Aboutjid:5, Cmd:, Args:[]string{}, Out:[]string{},
					// Submitaddr:tcp://172.22.80.1:50110, Serveraddr:tcp://172.22.92.40:1776,
					// Workeraddr:tcp://172.22.80.1:50110, ... }
				default:
					AlwaysPrintf("---- [worker pid %d; %s] unrecognized message '%s'\n", pid, j.Workeraddr, j)

				}
			}
		}
	}()
}

// best effort at killing, no promises due to race conditions.
// i.e. the job might have already died or finished.
func (w *Worker) KillRunningJob(serverRequested bool) {
	pid := os.Getpid()
	if w.RunningJob == nil || w.Pid == 0 {
		return
	}
	j := w.RunningJob

	AlwaysPrintf("---- [worker pid %d; %s] KillRunningJob executing against job %d / pid %d\n", pid, j.Workeraddr, j.Id, j.Pid)

	// we will still *also* send back a 'finishedwork' message indicating whether the
	// job completed or not, so the server should wait for this
	// 'finishedwork' to decide to report the job as cancelled or finished.

	killProcessGroup(w.Pid)
	proc, err := os.FindProcess(w.Pid)
	err = proc.Kill()

	w.TellShepPidKilled <- w.Pid
	if err != nil {
		// ignore, possible race: job already finished?
		//vv("err from proc.Kill: '%v'", err)
	} else {
		WPrintf("---- [worker pid %d; %s] Kill successful for job %d / pid %d\n", pid, j.Workeraddr, j.Id, j.Pid)
		processState, err := proc.Wait()
		WPrintf("---- [worker pid %d; %s] Kill details: After proc.Wait() for job %d / pid %d\n", pid, j.Workeraddr, j.Id, j.Pid)
		if err != nil {
			WPrintf("---- [worker pid %d; %s] Kill details: ProcessState for killed pid %d is: %#v\n", pid, j.Workeraddr, j.Pid, processState)
			/* // ProcessTable() segfaulting on OSX
			pt := ProcessTable()
			if (*pt)[w.Pid] {
				WPrintf(" --------------- after kill, FOUND in ptable, process w.Pid = %d\n", w.Pid)
			} else {
				WPrintf(" --------------- after kill, NOTFOUND, could not find w.Pid = %d\n", w.Pid)
			}
			*/

		}
	}

	if serverRequested {
		j.Aboutjid = j.Id
		w.NR.Cli.AsyncSend(CopyJobWithMsg(j, JOBMSG_ACKCANCELWIP))
		AlwaysPrintf("---- [worker pid %d; %s] Acked cancel wip back to server for job %d / pid %d\n", pid, j.Workeraddr, j.Id, w.Pid)
	}
}

func (w *Worker) TellServerJobFinished(j *Job) {
	w.Pid = 0
	w.RunningJid = 0
	w.RunningJob = nil

	AlwaysPrintf("---- [worker pid %d; %s] done with job %d: '%s'\n", os.Getpid(), j.Workeraddr, j.Id, j.Cmd)

	err := w.NR.Cli.AsyncSend(CopyJobWithMsg(j, JOBMSG_FINISHEDWORK))

	// this was timing out:
	// timeout after 10 seonds in test TestCancelJobInProgress; call by w.TellServerJobFinished(j) at workstart.go:222 in response to <-w.ShepSaysJobDone; in from DoShutdownSequence() at workstart.go:394
	//_, _, err := w.NR.Cli.DoSyncCallWithTimeout(10*time.Second, CopyJobWithMsg(j, JOBMSG_FINISHEDWORK))
	if err != nil {
		//vv("arg, err back from finished work report job: '%#v'", err)
		r := err.Error()
		if r == "context canceled" || r == "done channel closed" || r == "shutting down" {
			// err can be context Cancelled on timeout, don't panic.
			return
		}
	}
	panicOn(err)
}

func (w *Worker) DoShutdownSequence() {
	//vv("doing shutdown sequence")
	close(w.ShutdownSequenceStarted)

	WPrintf("\n\n --->>>>>>>>>>> starting DoShutdownSequence() <<<<<<<<<<<\n\n")
	// don't do Exit(0), in case we are in a goroutine local to the test process
	w.Forever = false

	// stop receiving nanomsgs.
	WPrintf("\n\n --->>>>>>>>>>> before recv w.NR.Ctrl <- die <<<<<<<<<<<\n\n")
	if !w.IsDeaf {
		w.NR.Ctrl <- die
		<-w.NR.Done
	}
	// avoid shutdown race with shepard by
	// killing any job in progress and letting shepard finish.
	var j *Job

	WPrintf("\n\n --->>>>>>>>>>> after recv w.NR.Ctrl <- die <<<<<<<<<<<   w.Pid = %d\n\n", w.Pid)
	if w.RunningJob != nil {
		if w.Pid == 0 {
			WPrintf("\n\n --->>>>>>>>>>> before pid := <-w.ShepSaysJobStarted  <<<<<<<<<<<\n\n")
			pid := <-w.ShepSaysJobStarted
			WPrintf("\n\n --->>>>>>>>>>> after pid := <-w.ShepSaysJobStarted  <<<<<<<<<<<\n\n")
			w.Pid = pid
		}
		w.KillRunningJob(false)
		WPrintf("\n\n --->>>>>>>>>>> before j = <-w.ShepSaysJobDone  <<<<<<<<<<<\n\n")
		j = <-w.ShepSaysJobDone
		WPrintf("\n\n --->>>>>>>>>>> after j = <-w.ShepSaysJobDone  <<<<<<<<<<<\n\n")

		w.TellServerJobFinished(j) // one timeout 10 seconds.
	}

	// and then stop sending too
	WPrintf("\n\n --->>>>>>>>>>> before send w.NS.Ctrl <- die <<<<<<<<<<<\n\n")
	// tests hang with this in? TestJobOutputIsWrittenToDisk for example.
	// w.NR.Cli.Close()

	AlwaysPrintf("[pid %d; %s] worker dies.\n", os.Getpid(), w.Addr)
	WPrintf("\n\n --->>>>>>>>>>> THE END <<<<<<<<<<<\n\n")
	close(w.Done)
}

func (w *Worker) SendRequestForJobToServer() (err error) {
	//vv("Worker.SendRequestForJobToServer() started.")
	request := NewJob()
	request.Msg = JOBMSG_REQUESTFORWORK
	request.Workeraddr = w.NR.Cli.LocalAddr()
	request.Serveraddr = w.ServerAddr

	AlwaysPrintf("---- [worker pid %d; %s] sending request for job to server '%s'\n", os.Getpid(), w.Addr, w.ServerAddr)
	return w.NR.Cli.AsyncSend(request)
	//job, _, err = w.NR.Cli.DoSyncCallWithTimeout(10*time.Second, request)
	//return job, err
}

func (w *Worker) DoOneJob() (j *Job, err error) {
	w.DoSingleJob <- true
	select { // hung here
	case j = <-w.JobFinished:
	case <-w.Done:
		// exit if the worker shutsdown
	case <-w.NR.ClientReconnect:
		// if we lost and regained the server, let
		// them know we are here.
	}
	return
}

// if it fails, I don't care. Issue/process if you
// get one, but don't block. This is used for testing (e.g. BadSig).
func (w *Worker) AttemptOnlyOneJob() {
	w.DoSingleJob <- true
}

func (w *Worker) DoOneJobTimeout(to time.Duration) (j *Job, err error) {
	w.DoSingleJob <- true
	timeout := time.After(to)
	select {
	case <-timeout:
		err = fmt.Errorf("DoOneJobTimeout() timed-out after %v", to)
	case j = <-w.JobFinished:
	}
	return
}

// public destructor: call this to invoke orderly shutdown.
func (w *Worker) Destroy() {
	// actually this might have already happened, in
	// which case we don't want to block, so check <-w.Done
	select {
	case <-w.Done:
		// alreadyDone, w.Done is closed.
	default:
		w.Ctrl <- die
		<-w.Done
	}
}

// drain any leftovers from this signaling channel, to reset it
// for reuse in the new shepard.
func (w *Worker) DrainTellShepPidKilled() {
	for {
		select {
		case <-w.TellShepPidKilled:
		default:
			return
		}
	}
}
