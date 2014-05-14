package main

import (
	"fmt"
	"io/ioutil"
	"os"
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

	cv.Convey("In a new empty dir (to avoid any .goqclusterid file), Shell out to 'goq clusterid' should give us new clusterids", t, func() {

		// make new temp dir that will have no ".goqclusterid files in it
		origdir, err := os.Getwd()
		if err != nil {
			panic(err)
		}
		tmpdir, err := ioutil.TempDir(".", "randomclusteridtest")
		if err != nil {
			panic(err)
		}
		os.Chdir(tmpdir)

		call0 := ShellOutForClusterId()
		call1 := ShellOutForClusterId()
		fmt.Printf("\n ShellOutForClusterId() produced sequential id: %s, %s\n", call0, call1)
		cv.So(call0, cv.ShouldNotEqual, call1)

		valid0 := IsValidClusterId(call0)
		valid1 := IsValidClusterId(call1)

		cv.So(valid0, cv.ShouldEqual, true)
		cv.So(valid1, cv.ShouldEqual, true)

		if !valid0 {
			fmt.Printf("call0 found invalid cid: '%s'\n", call0)
		}
		if !valid1 {
			fmt.Printf("call1 found invalid cid: '%s'\n", call1)
		}

		// cleanup
		os.Chdir(origdir)
		err = os.Remove(tmpdir)
		if err != nil {
			panic(err)
		}

	})
}

// requires passwordless ssh be setup from and to the current host itself (loopback via ssh).
func TestLearnClusterIdViaSSH(t *testing.T) {
	/*
		cv.Convey("Shell out to 'ssh goq clusterid' should give us new clusterids", t, func() {
			call0 := ShellOutForClusterId()
			call1 := ShellOutForClusterId()
			fmt.Printf("\n ShellOutForClusterId() produced sequential id: %s, %s\n", call0, call1)
			cv.So(call0, cv.ShouldNotEqual, call1)
			cv.So(IsValidClusterId(call0), cv.ShouldEqual, true)
			cv.So(IsValidClusterId(call1), cv.ShouldEqual, true)
		})
	*/
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

		m := EnvToMap(env)
		fmt.Printf("EnvToMap returned: %#v\n", m)
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

func TestConfigToEnv(t *testing.T) {

	cv.Convey("InjectConfigIntoEnv() should populate the env", t, func() {
		cv.Convey("So the env should start empty, and after injection have our values", func() {
			e := EnvToMap(os.Environ())
			cv.So(e[`GOQ_SENDTIMEOUT_MSEC`], cv.ShouldEqual, "")
			cv.So(e[`GOQ_JSERV_IP`], cv.ShouldEqual, "")
			cv.So(e[`GOQ_JSERV_PORT`], cv.ShouldEqual, "")
			cv.So(e[`GOQ_CLUSTERID`], cv.ShouldEqual, "")
			cv.So(e[`GOQ_NOSSHCONFIG`], cv.ShouldEqual, "")

			cfg := GetEnvConfig(RandId)
			InjectConfigIntoEnv(cfg)

			e2 := EnvToMap(os.Environ())
			cv.So(e2[`GOQ_SENDTIMEOUT_MSEC`], cv.ShouldEqual, fmt.Sprintf("%d", cfg.SendTimeoutMsec))
			cv.So(e2[`GOQ_JSERV_IP`], cv.ShouldEqual, cfg.JservIP)
			cv.So(e2[`GOQ_JSERV_PORT`], cv.ShouldEqual, fmt.Sprintf("%d", cfg.JservPort))
			cv.So(IsValidClusterId(e2[`GOQ_CLUSTERID`]), cv.ShouldEqual, true)
			cv.So(e2[`GOQ_NOSSHCONFIG`], cv.ShouldEqual, BoolToString(cfg.NoSshConfig))
		})
	})
}
