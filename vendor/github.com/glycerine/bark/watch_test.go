package bark

import (
	"fmt"
	"testing"
	"time"

	cv "github.com/glycerine/goconvey/convey"
)

func Test100WatchdogShouldNoticeAndRestartChild(t *testing.T) {

	cv.Convey("our Watchdog goroutine should be able to restart the child process upon request", t, func() {
		watcher := NewWatchdog(nil, "./testcmd/sleep50")
		watcher.Start()

		var err error
		testOver := time.After(100 * time.Millisecond)

	testloop:
		for {
			select {
			case <-testOver:
				fmt.Printf("\n testOver timeout fired.\n")
				err = watcher.Stop()
				break testloop
			case <-time.After(3 * time.Millisecond):
				VPrintf("watch_test: after 3 milliseconds: requesting restart of child process.\n")
				watcher.RestartChild <- true
			case <-watcher.Done:
				fmt.Printf("\n water.Done fired.\n")
				err = watcher.GetErr()
				fmt.Printf("\n watcher.Done, with err = '%v'\n", err)
				break testloop
			}
		}
		panicOn(err)
		// getting 14-27 starts on OSX
		fmt.Printf("\n done after %d starts.\n", watcher.startCount)
		cv.So(watcher.startCount, cv.ShouldBeGreaterThan, 5)
	})
}

func Test200WatchdogTerminatesUponRequest(t *testing.T) {

	cv.Convey("our Watchdog goroutine should terminate its child process and stop upon request", t, func() {

		watcher := NewWatchdog(nil, "./testcmd/sleep50", "arg1", "arg2")
		watcher.Start()

		sleepDur := 10 * time.Millisecond
		time.Sleep(sleepDur)

		pid := <-watcher.CurrentPid
		if pid <= 0 {
			panic("error: pid was <= 0 implying process did not start")
		}

		watcher.TermChildAndStopWatchdog <- true
		err := WaitForShutdownWithTimeout(pid, time.Millisecond*100)
		panicOn(err)
		cv.So(err, cv.ShouldEqual, nil)

		<-watcher.Done
		P("shutdown complete")
	})
}
