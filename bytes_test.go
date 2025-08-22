package main

import (
	"fmt"
	"reflect"
	"testing"

	cv "github.com/glycerine/goconvey/convey"
)

func TestJobToBytesAndBack(t *testing.T) {

	cv.Convey("job to bytes and back", t, func() {

		// encryption will mess up the comparison.
		origAes := AesOff
		defer func() {
			AesOff = origAes
		}()
		AesOff = true

		// *** universal test cfg setup
		skipbye := false
		cfg := NewTestConfig(t)
		defer cfg.ByeTestConfig(&skipbye)
		// *** end universal test setup

		j := MakeTestJob()
		j.Submitaddr = "localhost"

		by, err := cfg.jobToBytes(j)
		panicOn(err)
		j2, err := cfg.bytesToJob(by)
		panicOn(err)

		// the Args,Out,Env string slices being
		// either nil vs empty slice are causing
		// a problem here.
		if !reflect.DeepEqual(j, j2) {
			vv("j='%#v'", j)
			vv("j2='%#v'", j2)
			panic("jobToBytes problem")
		}
		cv.So(reflect.DeepEqual(j2, j), cv.ShouldBeTrue)

		// and inverse
		by2, err := cfg.jobToBytes(j2)
		panicOn(err)
		j3, err := cfg.bytesToJob(by2)
		_ = j3
		cv.So(reflect.DeepEqual(j3, j), cv.ShouldBeTrue)

		//cv.So(string(by2), cv.ShouldResemble, string(by))

		cv.So(len(by2), cv.ShouldResemble, len(by))
		for i := range by {
			if by2[i] != by[i] {
				panic(fmt.Sprintf("discrepency at byte %v: '%v' vs '%v'", i, string(by2), string(by)))
			}
		}
	})
}
