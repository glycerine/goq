package main

import (
	"testing"

	cv "github.com/glycerine/goconvey/convey"
	//schema "github.com/glycerine/goq/schema"
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
		job2, err := CapnpToJob(&buf)
		panicOn(err)

		cv.So(JobSignatureOkay(job2, cfg), cv.ShouldEqual, true)
		cv.So(GetJobSignature(job2, cfg), cv.ShouldEqual, GetJobSignature(job, cfg))

	})
}
