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
	"strings"
	"time"
)

func (w *Worker) Shepard(j *Job) {

	// reset our input channel
	w.DrainTellShepPidKilled()

	go func() {
		jid := j.Id
		dir := j.Dir
		cmd := j.Cmd
		args := j.Args
		env := j.Env
		if j.Out == nil {
			j.Out = make([]string, 0)
		}
		var origdir string
		var err error
		origdir, err = os.Getwd()
		if err != nil {
			panic(err)
		}

		if dir != "" {
			err = os.Chdir(dir)
			if err != nil {
				j.Out = append(j.Out, fmt.Sprintf("Shepard got error trying to move to submit directory with os.Chdir('%s'): %s", dir, err))
				return
			}
			// go back to our starting dir at the end of shepding this job.
			defer os.Chdir(origdir)
		}

		//		if w.ShepStarted != nil {
		//			w.ShepStarted <- true
		//		}

		c := exec.Command(cmd, args...)
		c.Dir = dir
		c.Env = env

		var oe bytes.Buffer
		c.Stdout = &oe
		c.Stderr = &oe

		//	oe, err = c.CombinedOutput()
		err = c.Start()
		if err != nil {
			j.Out = append(j.Out, fmt.Sprintf("Shepard finds non-nil err on trying to Start() cmd '%s' in dir '%s': %s", cmd, dir, err))
			w.ShepSaysJobStarted <- j
			w.ShepSaysJobDone <- j
			return
		}

		myPid := c.Process.Pid
		j.Pid = int64(myPid)
		VPrintf("\n SHEP Shepard goroutine about to block on w.ShepSaysJobStarted <- j\n")
		w.ShepSaysJobStarted <- j
		VPrintf("\n SHEP Shepard goroutine about to block on c.Wait()\n")

		// this c.Wait() can be 15-20 seconds *slooooow*, so also wait on TellShepPidKilled
		//  to speed things up.
		err = nil
		// waitDone is buffered so this next short goro can exit immediately after c.Wait() finishes.
		// i.e. if TellShepPidKilled arrives first, then there will never be a receiver, so
		// without the buffering the channel and goroutine would be blocked, uncollectable, waiting-forever garbage/leak.
		waitDone := make(chan error, 1)
		go func() {
			err = c.Wait()
			waitDone <- err
		}()

		// back in shep goroutine:
		select {
		case err = <-waitDone:
		case killedPid := <-w.TellShepPidKilled:
			if killedPid != myPid {
				panic(fmt.Sprintf("SHEP error: mismatch in myPid(%d) vs killedPid(%d) received on w.TellShepPidKilled", myPid, killedPid))
			}
			j.Cancelled = true
			WPrintf("\n SHEP got notice from w.TellShepPidKilled, setting j.Cancelled = true\n")
		}

		// Now set j.Out based on which of the two cases we just saw:
		//  Either we saw w.TellShepPidKilled, in which case j.Cancelled == true and we want to exit quickly.
		//  Otherwise, we had a normal or fast c.Wait() exit, and we want to gather and send output on j.Out.
		//
		if j.Cancelled {
			j.Out = append(j.Out, fmt.Sprintf("cancelled/killed: job %d / pid %d ; cmd '%s' in dir '%s' on worker '%s' at '%s'", jid, myPid, cmd, dir, j.Workeraddr, time.Now()))

			// Don't wait around for output/etc.
			// Just skip down to ShepSaysJobDone and get out of here fast.

		} else {
			// Normal/Fast c.Wait() exit:
			WPrintf("\n SHEP DONE with WAIT, err: '%s'\n", err)
			if err != nil && err.Error() == "signal: killed" {
				WPrintf("\n SHEP found 'signal:killed', setting j.Cancelled = true\n")
				j.Cancelled = true
			}
			s := string(oe.Bytes())
			strings.Trim(s, "\n")
			slen := len(s)
			out := strings.Split(s, "\n")
			// if file ended in '\n' then we now have an extra empty line to eliminate.
			N := len(out)
			if slen > 0 && s[slen-1] == '\n' && N > 0 && out[N-1] == "" {
				out = out[:N-1]
			}
			j.Out = append(j.Out, out...)

			if err != nil {
				j.Out = append(j.Out, fmt.Sprintf("Shepard finds non-nil err on trying to Wait() on cmd '%s' in dir '%s': %s", cmd, dir, err))
			}
		}
		WPrintf("end of SHEP: just before w.ShepSaysJobDone <- j\n")
		w.ShepSaysJobDone <- j
		WPrintf("end of SHEP: just after w.ShepSaysJobDone <- j\n")
	}()
}
