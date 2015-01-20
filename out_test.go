package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	cv "github.com/glycerine/goconvey/convey"
)

// job output test
//

func TestJobOutputIsWrittenToDisk(t *testing.T) {

	cv.Convey("When we submit a simple echo job, the output should be found on disk, in a file tagged with the job id", t, func() {

		var jobserv *JobServ
		var err error
		var jobservPid int
		remote := false

		// *** universal test cfg setup
		skipbye := false
		cfg := NewTestConfig()
		defer cfg.ByeTestConfig(&skipbye)
		// *** end universal test setup

		//cfg := DefaultCfg()
		cfg.DebugMode = true // reply to badsig packets
		//cfg.SendTimeoutMsec = 30000
		cfg.Odir = "testo"
		// delete old contents of testo so we can run the test
		// repeatedly, but don't auto-cleanup after the test. => human can inspect.
		CleanupOutdir(cfg)
		cv.So(DirExists(cfg.Odir), cv.ShouldEqual, false)

		WaitUntilAddrAvailable(cfg.JservAddr())

		if remote {

			jobservPid, err = NewExternalJobServ(cfg)
			if err != nil {
				panic(err)
			}
			fmt.Printf("\n")
			fmt.Printf("[pid %d] spawned a new external JobServ with pid %d\n", os.Getpid(), jobservPid)

		} else {

			jobserv, err = NewJobServ(cfg)
			if err != nil {
				panic(err)
			}
		}
		var skip bool
		defer CleanupServer(cfg, jobservPid, jobserv, remote, &skip)
		// don't do this, since we are testing for output: defer CleanupOutdir(cfg)

		pwd, err := os.Getwd()
		if err != nil {
			panic(err)
		}

		j := NewJob()
		//j.Cmd = "./bin/sleep.sh"
		j.Cmd = "/bin/echo"
		j.Args = []string{"hello world"}
		j.Dir = pwd + "/new_sub_dir"

		err = os.Mkdir(j.Dir, 0775)
		if err != nil {
			panic(err)
		}

		sub, err := NewSubmitter(GenAddress(), cfg, false)
		if err != nil {
			panic(err)
		}
		reply, err := sub.SubmitJobGetReply(j)
		if err != nil {
			panic(err)
		}
		fmt.Printf("\n [pid %d] submitter got reply %s with job.Aboutjid=%d      full reply: %#v\n", os.Getpid(), reply.Msg, reply.Aboutjid, reply)

		// WaitForJob returns only after the request to watch the job has been registered
		waitchan, err := sub.WaitForJob(reply.Aboutjid)
		if err != nil {
			panic(err)
		}

		worker, err := NewWorker(GenAddress(), cfg, nil)
		if err != nil {
			panic(err)
		}

		// to test the wait-finish, get to it sooner while job is in background
		go func() {
			worker.DoOneJob()
			worker.Destroy()
		}()

		// gotta wait for server to write
		reply2 := <-waitchan
		if reply2.Id == -1 {
			fmt.Printf("\n There was an error while we were waiting for the job to finish.\n")
			panic(reply2.Out)
		}
		fmt.Printf("\n [pid %d] after WaitForJob, submitter got reply2 %s with job.Aboutid=%d     full reply: %#v\n", os.Getpid(), reply2.Msg, reply2.Aboutjid, reply2)

		// now we should be safe to shutdown
		CleanupServer(cfg, jobservPid, jobserv, remote, nil)
		skip = true // tell the deferred CleanupServer they don't need to run now.

		fn := fmt.Sprintf("%s/%s/out.%05d", j.Dir, cfg.Odir, reply.Aboutjid)
		odir := fmt.Sprintf("%s/%s", j.Dir, cfg.Odir)
		fmt.Printf("\nout_test is checking for file: %s\n", fn)
		dire := DirExists(odir)
		cv.So(dire, cv.ShouldEqual, true)

		if dire {
			filee := FileExists(fn)
			cv.So(filee, cv.ShouldEqual, true)

			if filee {
				slurp, err := ioutil.ReadFile(fn)
				if err != nil {
					panic(err)
				}
				line := strings.Trim(string(slurp), " \n\t")
				cv.So(line, cv.ShouldEqual, "hello world")
			} else {
				skipbye = true
			}
		} else {
			skipbye = true
		}

	})
}
