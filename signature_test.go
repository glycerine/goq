package main

import (
	"testing"

	cv "github.com/glycerine/goconvey/convey"
)

// signature test
//

func TestSignatureConsistent(t *testing.T) {

	cv.Convey("signing a job with the same clusterid as you check with should be consistent, even after reserialization.", t, func() {
		job := MakeTestJob()
		cfg := &Config{}
		cfg.ClusterId = RandomClusterId()

		SignJob(job, cfg)
		cv.So(JobSignatureOkay(job, cfg), cv.ShouldEqual, true)

		// then pass through serial/deserialize
		//buf, _ := JobToCapnp(job)
		by, err := job.MarshalMsg(nil)
		panicOn(err)

		//job2, err := CapnpToJob(&buf)
		job2 := &Job{}
		_, err = job2.UnmarshalMsg(by)
		panicOn(err)
		//vv("job  = '%v'", job)
		//vv("job2 = '%v'", job2)

		cv.So(JobSignatureOkay(job2, cfg), cv.ShouldEqual, true)
		cv.So(GetJobSignature(job2, cfg), cv.ShouldEqual, GetJobSignature(job, cfg))

	})
}
