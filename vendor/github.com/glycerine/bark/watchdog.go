package bark

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type Watchdog struct {
	Ready                       chan bool
	RestartChild                chan bool
	ReqStopWatchdog             chan bool
	TermChildAndStopWatchdog    chan bool
	Done                        chan bool
	CurrentPid                  chan int
	StopWatchdogAfterChildExits chan bool

	curPid int

	startCount int64

	mut      sync.Mutex
	shutdown bool

	PathToChildExecutable string
	Args                  []string
	Attr                  os.ProcAttr
	err                   error
	needRestart           bool
	proc                  *os.Process
	exitAfterReaping      bool
}

// NewWatchdog creates a Watchdog structure but
// does not launch it or its child process until
// Start() is called on it.
// The attr argument sets function attributes such
// as environment and open files; see os.ProcAttr for details.
// Also, attr can be nil here, in which case and
// empty os.ProcAttr will be supplied to the
// os.StartProcess() call.
func NewWatchdog(
	attr *os.ProcAttr,
	pathToChildExecutable string,
	args ...string) *Watchdog {

	cpOfArgs := make([]string, 0)
	for i := range args {
		cpOfArgs = append(cpOfArgs, args[i])
	}
	w := &Watchdog{
		PathToChildExecutable: pathToChildExecutable,
		Args:                        cpOfArgs,
		Ready:                       make(chan bool),
		RestartChild:                make(chan bool),
		ReqStopWatchdog:             make(chan bool),
		TermChildAndStopWatchdog:    make(chan bool),
		StopWatchdogAfterChildExits: make(chan bool),
		Done:       make(chan bool),
		CurrentPid: make(chan int),
	}

	if attr != nil {
		w.Attr = *attr
	}
	return w
}

// NewOneshotReaper() is just like NewWatchdog,
// except that it also does two things:
//
// a) sets a flag so that the
// watchdog won't restart the child process
// after it finishes.
//
// and
//
// b) the watchdog goroutine will itself exit
// once the child exists--after cleaning up.
// Cleanup means we don't leave
// zombies and we call go's os.Process.Release() to
// release associated resources after the child
// exits.
//
// You still need to call Start(), just like
// after NewWatchdog().
//
func NewOneshotReaper(
	attr *os.ProcAttr,
	pathToChildExecutable string,
	args ...string) *Watchdog {

	w := NewWatchdog(attr, pathToChildExecutable, args...)
	w.exitAfterReaping = true
	return w
}

// Oneshot() combines NewOneshotReaper() and Start().
// In other words, we start the child once. We don't
// restart once it finishes. Instead we just reap and
// cleanup. The returned pointer's Done channel will
// be closed when the child process and watchdog
// goroutine have finished.
func Oneshot(pathToProcess string, args ...string) (*Watchdog, error) {

	watcher := NewOneshotReaper(nil, pathToProcess, args...)
	watcher.Start()

	return watcher, nil
}

// StartAndWatch() is the convenience/main entry API.
// pathToProcess should give the path to the executable within
// the filesystem. If it dies it will be restarted by
// the Watchdog.
func StartAndWatch(pathToProcess string, args ...string) (*Watchdog, error) {

	// start our child; restart it if it dies.
	watcher := NewWatchdog(nil, pathToProcess, args...)
	watcher.Start()

	return watcher, nil
}

func (w *Watchdog) AlreadyDone() bool {
	select {
	case <-w.Done:
		return true
	default:
		return false
	}
}
func (w *Watchdog) Stop() error {
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

	close(w.ReqStopWatchdog)
	<-w.Done
	// don't wait for Done while holding the lock,
	// as that is deadlock prone.

	w.mut.Lock()
	defer w.mut.Unlock()
	w.shutdown = true
	return w.err
}

func (w *Watchdog) SetErr(err error) {
	w.mut.Lock()
	defer w.mut.Unlock()
	w.err = err
}

func (w *Watchdog) GetErr() error {
	w.mut.Lock()
	defer w.mut.Unlock()
	return w.err
}

// see w.err for any error after w.Done
func (w *Watchdog) Start() {

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
				Q(" debug: about to start '%s'", w.PathToChildExecutable)
				w.proc, err = os.StartProcess(w.PathToChildExecutable, w.Args, &w.Attr)
				if err != nil {
					w.err = err
					return
				}
				w.curPid = w.proc.Pid
				w.needRestart = false
				w.startCount++
				Q(" Start number %d: Watchdog started pid %d / new process '%s'", w.startCount, w.proc.Pid, w.PathToChildExecutable)
			}

			select {
			case <-w.StopWatchdogAfterChildExits:
				w.exitAfterReaping = true
			case w.CurrentPid <- w.curPid:
			case <-w.TermChildAndStopWatchdog:
				Q(" TermChildAndStopWatchdog noted, exiting watchdog.Start() loop")

				err := w.proc.Signal(syscall.SIGKILL)
				if err != nil {
					err = fmt.Errorf("warning: watchdog tried to SIGKILL pid %d but got error: '%s'", w.proc.Pid, err)
					w.SetErr(err)
					log.Printf("%s", err)
					return
				}
				w.exitAfterReaping = true
				continue reaploop
			case <-w.ReqStopWatchdog:
				Q(" ReqStopWatchdog noted, exiting watchdog.Start() loop")
				return
			case <-w.RestartChild:
				Q(" debug: got <-w.RestartChild")
				err := w.proc.Signal(syscall.SIGKILL)
				if err != nil {
					err = fmt.Errorf("warning: watchdog tried to SIGKILL pid %d but got error: '%s'", w.proc.Pid, err)
					w.SetErr(err)
					log.Printf("%s", err)
					return
				}
				w.curPid = 0
				continue reaploop
			case <-signalChild:
				Q(" debug: got <-signalChild")

				for i := 0; i < 1000; i++ {
					pid, err := syscall.Wait4(w.proc.Pid, &ws, syscall.WNOHANG, nil)
					// pid > 0 => pid is the ID of the child that died, but
					//  there could be other children that are signalling us
					//  and not the one we in particular are waiting for.
					// pid -1 && errno == ECHILD => no new status children
					// pid -1 && errno != ECHILD => syscall interupped by signal
					// pid == 0 => no more children to wait for.
					Q(" pid=%v  ws=%v and err == %v", pid, ws, err)
					switch {
					case err != nil:
						err = fmt.Errorf("wait4() got error back: '%s' and ws:%v", err, ws)
						log.Printf("warning in reaploop, wait4(WNOHANG) returned error: '%s'. ws=%v", err, ws)
						w.SetErr(err)
						continue reaploop
					case pid == w.proc.Pid:
						Q(" Watchdog saw OUR current w.proc.Pid %d/process '%s' finish with waitstatus: %v.", pid, w.PathToChildExecutable, ws)
						if w.exitAfterReaping {
							Q("watchdog sees exitAfterReaping. exiting now.")
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
						Q("pid == 0 on wait4, (perhaps SIGSTOP?): nobody left to wait for, keep looping. ws = %v", ws)
						continue reaploop
					default:
						Q(" warning in reaploop: wait4() negative or not our pid, sleep and try again")
						time.Sleep(time.Millisecond)
					}
				} // end for i
				w.SetErr(fmt.Errorf("could not reap child PID %d or obtain wait4(WNOHANG)==0 even after 1000 attempts", w.proc.Pid))
				log.Printf("%s", w.err)
				return
			} // end select
		} // end for reaploop
	}()
}
