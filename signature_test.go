package main

import (
	"fmt"
	"os"
	"testing"

	cv "github.com/smartystreets/goconvey/convey"
)

// signature test
//

func TestSignatureConsistent(t *testing.T) {

	cv.Convey("signing a job with the same clusterid as you check with should be consistent, even after reserialization.", t, func() {
		job := MakeTestJob()
		cfg := &Config{
			ClusterId: RandomClusterId(),
		}
		SignJob(job, cfg)
		cv.So(JobSignatureOkay(job, cfg), cv.ShouldEqual, true)

		// then pass through capn serial/deserialize
		buf, _ := JobToCapnp(job)
		job2 := CapnpToJob(&buf)

		cv.So(JobSignatureOkay(job2, cfg), cv.ShouldEqual, true)
		cv.So(GetJobSignature(job2, cfg), cv.ShouldEqual, GetJobSignature(job, cfg))

	})
}

func TestSubmitBadSignatureDetected(t *testing.T) {

	cv.Convey("remotely, over nanomsg, goq should be able to submit job to the server", t, func() {
		cv.Convey("and then server should get back the expected output from the job run", func() {
			cfg := GetEnvConfig(RandId)
			childpid, err := NewExternalJobServ(cfg)
			if err != nil {
				panic(err)
			}
			fmt.Printf("[pid %d] spawned a new external JobServ with pid %d\n", os.Getpid(), childpid)

			j := NewJob()
			j.Cmd = "bin/good.sh"

			sub, err := NewSubmitter(GenAddress(), cfg)
			if err != nil {
				panic(err)
			}
			sub.SetServer(cfg.JservAddr)
			sub.SubmitJob(j)

			worker, err := NewWorker(GenAddress(), cfg)
			if err != nil {
				panic(err)
			}
			worker.SetServer(cfg.JservAddr)
			jobout, err := worker.DoOneJob()
			if err != nil {
				panic(err)
			}

			// *important* cleanup, and wait for cleanup to finish, so the next test can run.
			SendShutdown(cfg)
			WaitForShutdownWithTimeout(childpid)

			cv.So(len(jobout.Out), cv.ShouldEqual, 2)
			cv.So(jobout.Out[0], cv.ShouldEqual, "I'm starting some work")
			cv.So(jobout.Out[1], cv.ShouldEqual, "I'm done with my work")

		})
	})
}
