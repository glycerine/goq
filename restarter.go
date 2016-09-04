package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type Restarter struct {
	Ready                        chan bool
	RestartChild                 chan bool
	ReqStopRestarter             chan bool
	TermChildAndStopRestarter    chan bool
	Done                         chan bool
	CurrentPid                   chan int
	StopRestarterAfterChildExits chan bool
	Cmd                          *exec.Cmd

	curPid int

	startCount int64

	mut      sync.Mutex
	shutdown bool

	cmd              *exec.Cmd
	err              error
	needRestart      bool
	proc             *os.Process
	exitAfterReaping bool
}

// NewRestarter creates a Restarter structure but
// does not launch it or its child process until
// Start() is called on it.
// Use w := NewRestarter() and w.Start() in
// place of cmd.Start().
func NewRestarter(cmd *exec.Cmd) *Restarter {

	w := &Restarter{
		Cmd:                          cmd,
		Ready:                        make(chan bool),
		RestartChild:                 make(chan bool),
		ReqStopRestarter:             make(chan bool),
		TermChildAndStopRestarter:    make(chan bool),
		StopRestarterAfterChildExits: make(chan bool),
		Done:       make(chan bool),
		CurrentPid: make(chan int),
	}

	return w
}

// NewOneshotReaper() is just like NewRestarter,
// except that it also does two things:
//
// a) sets a flag so that the
// Restarter won't restart the child process
// after it finishes.
//
// and
//
// b) the Restarter goroutine will itself exit
// once the child exists--after cleaning up.
// Cleanup means we don't leave
// zombies and we call go's os.Process.Release() to
// release associated resources after the child
// exits.
//
func NewOneshotReaper(cmd *exec.Cmd) *Restarter {

	w := NewRestarter(cmd)
	w.exitAfterReaping = true
	return w
}

// Oneshot() combines NewOneshotReaper() and Start().
// In other words, we start the child once. We don't
// restart once it finishes. Instead we just reap and
// cleanup. The returned pointer's Done channel will
// be closed when the child process and Restarter
// goroutine have finished.
func Oneshot(cmd *exec.Cmd) (*Restarter, error) {

	restarter := NewOneshotReaper(cmd)
	restarter.Start()

	return restarter, nil
}

// StartAndWatch() is the convenience/main entry API.
// pathToProcess should give the path to the executable within
// the filesystem. If it dies it will be restarted by
// the Restarter.
func StartAndWatch(cmd *exec.Cmd) (*Restarter, error) {

	// start our child; restart it if it dies.
	restarter := NewRestarter(cmd)
	restarter.Start()

	return restarter, nil
}

func (w *Restarter) AlreadyDone() bool {
	select {
	case <-w.Done:
		return true
	default:
		return false
	}
}
func (w *Restarter) Stop() error {
	if w.AlreadyDone() {
		// once Done, w.err is immutable, so we don't need to lock.
		return w.err
	}
	w.mut.Lock()
	if w.shutdown {
		defer w.mut.Unlock()
		return w.err
	}
	w.mut.Unlock()

	close(w.ReqStopRestarter)
	<-w.Done
	// don't wait for Done while holding the lock,
	// as that is deadlock prone.

	w.mut.Lock()
	defer w.mut.Unlock()
	w.shutdown = true
	return w.err
}

func (w *Restarter) SetErr(err error) {
	w.mut.Lock()
	defer w.mut.Unlock()
	w.err = err
}

func (w *Restarter) GetErr() error {
	w.mut.Lock()
	defer w.mut.Unlock()
	return w.err
}

// see w.err for any error after w.Done
func (w *Restarter) Start() {

	signalChild := make(chan os.Signal, 1)

	signal.Notify(signalChild, syscall.SIGCHLD)

	w.needRestart = true
	var ws syscall.WaitStatus
	go func() {
		defer func() {
			if w.proc != nil {
				w.proc.Release()
			}
			close(w.Done)
			// can deadlock if we don't close(w.Done) before grabbing the mutex:
			w.mut.Lock()
			w.shutdown = true
			w.mut.Unlock()
			signal.Stop(signalChild) // reverse the effect of the above Notify()
		}()
		var err error

	reaploop:
		for {
			if w.needRestart {
				if w.proc != nil {
					w.proc.Release()
				}
				TSPrintf(" debug: about to start '%s'", w.Cmd.Path)
				//w.proc, err = os.StartProcess(w.PathToChildExecutable, w.Args, &w.Attr)
				err = w.Cmd.Start()
				w.proc = w.Cmd.Process
				if err != nil {
					w.err = err
					return
				}
				w.curPid = w.proc.Pid
				w.needRestart = false
				w.startCount++
				TSPrintf(" Start number %d: Restarter started pid %d / new process '%s'", w.startCount, w.proc.Pid, w.Cmd.Path)
			}

			select {
			case <-w.StopRestarterAfterChildExits:
				w.exitAfterReaping = true
			case w.CurrentPid <- w.curPid:
			case <-w.TermChildAndStopRestarter:
				TSPrintf(" TermChildAndStopRestarter noted, exiting Restarter.Start() loop")

				err := w.proc.Signal(syscall.SIGKILL)
				if err != nil {
					err = fmt.Errorf("warning: Restarter tried to SIGKILL pid %d but got error: '%s'", w.proc.Pid, err)
					w.SetErr(err)
					log.Printf("%s", err)
					return
				}
				w.exitAfterReaping = true
				continue reaploop
			case <-w.ReqStopRestarter:
				TSPrintf(" ReqStopRestarter noted, exiting Restarter.Start() loop")
				return
			case <-w.RestartChild:
				TSPrintf(" debug: got <-w.RestartChild")
				err := w.proc.Signal(syscall.SIGKILL)
				if err != nil {
					err = fmt.Errorf("warning: Restarter tried to SIGKILL pid %d but got error: '%s'", w.proc.Pid, err)
					w.SetErr(err)
					log.Printf("%s", err)
					return
				}
				w.curPid = 0
				continue reaploop
			case <-signalChild:
				TSPrintf(" debug: got <-signalChild")

				for i := 0; i < 1000; i++ {
					pid, err := syscall.Wait4(w.proc.Pid, &ws, syscall.WNOHANG, nil)
					// pid > 0 => pid is the ID of the child that died, but
					//  there could be other children that are signalling us
					//  and not the one we in particular are waiting for.
					// pid -1 && errno == ECHILD => no new status children
					// pid -1 && errno != ECHILD => syscall interupped by signal
					// pid == 0 => no more children to wait for.
					TSPrintf(" pid=%v  ws=%v and err == %v", pid, ws, err)
					switch {
					case err != nil:
						err = fmt.Errorf("wait4() got error back: '%s' and ws:%v", err, ws)
						log.Printf("warning in reaploop, wait4(WNOHANG) returned error: '%s'. ws=%v", err, ws)
						w.SetErr(err)
						continue reaploop
					case pid == w.proc.Pid:
						TSPrintf(" Restarter saw OUR current w.proc.Pid %d/process '%s' finish with waitstatus: %v.", pid, w.Cmd.Path, ws)
						if w.exitAfterReaping {
							TSPrintf("Restarter sees exitAfterReaping. exiting now.")
							return
						}
						w.needRestart = true
						w.curPid = 0
						continue reaploop
					case pid == 0:
						// this is what we get when SIGSTOP is sent on OSX. ws == 0 in this case.
						// Note that on OSX we never get a SIGCONT signal.
						// Under WNOHANG, pid == 0 means there is nobody left to wait for,
						// so just go back to waiting for another SIGCHLD.
						TSPrintf("pid == 0 on wait4, (perhaps SIGSTOP?): nobody left to wait for, keep looping. ws = %v", ws)
						continue reaploop
					default:
						TSPrintf(" warning in reaploop: wait4() negative or not our pid, sleep and try again")
						time.Sleep(time.Millisecond)
					}
				} // end for i
				err = fmt.Errorf("could not reap child PID %d or obtain wait4(WNOHANG)==0 even after 1000 attempts", w.proc.Pid)
				w.SetErr(err)
				log.Printf("%s", err)
				return
			} // end select
		} // end for reaploop
	}()
}
