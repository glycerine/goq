package main

import (
	"fmt"
	"sort"
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
		cfg := GetEnvConfig(RandId)

		cid := RandomClusterId()
		SaveLocalClusterId(cid, ".", cfg)
		reread := LoadLocalClusterId(".", cfg)

		if reread != cid {
			fmt.Printf("\narg, difference between reread(%s) and original clusterid(%s)\n", reread, cid)
		}
		cv.So(reread, cv.ShouldEqual, cid)

		// cleanup
		RemoveLocalClusterId(".", cfg)
	})
}

func TestEnvAsMapAndBack(t *testing.T) {

	cv.Convey("EnvAsMap splits out the environment into key-value pairs", t, func() {
		env := sort.StringSlice([]string{"ALLY=1", "_=2"})
		sort.Sort(env)

		m := EnvAsMap(env)
		fmt.Printf("EnvAsMap returned: %#v\n", m)
		cv.So(len(m), cv.ShouldEqual, 2)
		cv.So(m["ALLY"], cv.ShouldEqual, "1")
		cv.So(m["_"], cv.ShouldEqual, "2")

		e2 := MapToEnv(m)
		sort.Sort(sort.StringSlice(e2))

		for i := range e2 {
			cv.So(e2[i], cv.ShouldEqual, env[i])
		}
	})
}
