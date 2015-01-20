package main

import (
	"testing"

	cv "github.com/glycerine/goconvey/convey"
)

func TestWorkerSubmitPortAllocation(t *testing.T) {

	cv.Convey("qzub submit and workers should allocate their own port numbers to avoid conflicts", t, func() {
		cv.Convey(" so that we can run lots of workers and submits on the same machine if need be", func() {

		})
	})
}
