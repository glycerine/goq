package main

import (
	"fmt"
	"testing"

	cv "github.com/smartystreets/goconvey/convey"
)

// cfgenv.go related test
//

func TestRandomClusterId(t *testing.T) {

	cv.Convey("Two calls to RandomClusterId() should produce different ids", t, func() {
		call0 := RandomClusterId()
		call1 := RandomClusterId()
		fmt.Printf("\n RandomClusterId() produced sequential id: %s, %s\n", call0, call1)
		cv.So(call0, cv.ShouldNotEqual, call1)
		cv.So(IsValidClusterId(call0), cv.ShouldEqual, true)
		cv.So(IsValidClusterId(call1), cv.ShouldEqual, true)

	})
}

func TestCommandClusterId(t *testing.T) {

	cv.Convey("Shell out to 'goq clusterid' should give us new clusterids", t, func() {
		call0 := ShellOutForClusterId()
		call1 := ShellOutForClusterId()
		fmt.Printf("\n ShellOutForClusterId() produced sequential id: %s, %s\n", call0, call1)
		cv.So(call0, cv.ShouldNotEqual, call1)
		cv.So(IsValidClusterId(call0), cv.ShouldEqual, true)
		cv.So(IsValidClusterId(call1), cv.ShouldEqual, true)
	})
}

// requires passwordless ssh be setup from and to the current host itself (loopback via ssh).
func TestLearnClusterIdViaSSH(t *testing.T) {

	cv.Convey("Shell out to 'ssh goq clusterid' should give us new clusterids", t, func() {
		call0 := ShellOutForClusterId()
		call1 := ShellOutForClusterId()
		fmt.Printf("\n ShellOutForClusterId() produced sequential id: %s, %s\n", call0, call1)
		cv.So(call0, cv.ShouldNotEqual, call1)
		cv.So(IsValidClusterId(call0), cv.ShouldEqual, true)
		cv.So(IsValidClusterId(call1), cv.ShouldEqual, true)
	})
}

func TestSaveLoadClusterId(t *testing.T) {

	cv.Convey("SaveLocalClusterId() and LoadLocalClusterId() should save and load a matching clusterid from disk", t, func() {
		cid := RandomClusterId()
		SaveLocalClusterId(cid, ".")
		reread := LoadLocalClusterId(".")

		if reread != cid {
			fmt.Printf("\narg, difference between reread(%s) and original clusterid(%s)\n", reread, cid)
		}
		cv.So(reread, cv.ShouldEqual, cid)

		// cleanup
		RemoveLocalClusterId(".")
	})
}
