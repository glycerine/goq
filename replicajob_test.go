package main

import (
	"testing"

	cv "github.com/glycerine/goconvey/convey"
)

func TestReplicaJob(t *testing.T) {

	cv.Convey("If we start a job with a replication factor of 2", t, func() {
		cv.Convey("then the first job to finish should notify and cancel the other workers on that job", func() {
		})
	})
}
