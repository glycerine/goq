package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"testing"

	cv "github.com/glycerine/goconvey/convey"
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

func TestSaveLoadClusterId(t *testing.T) {

	cv.Convey("SaveLocalClusterId() and LoadLocalClusterId() should save and load a matching clusterid from disk", t, func() {

		// *** universal test cfg setup
		skipbye := false
		cfg := NewTestConfig()
		defer cfg.ByeTestConfig(&skipbye)
		// *** end universal test setup

		cid := RandomClusterId()
		SaveLocalClusterId(cid, cfg)
		reread, err := LoadLocalClusterId(cfg)
		if err != nil {
			panic(err)
		}

		if reread != cid {
			fmt.Printf("\narg, difference between reread(%s) and original clusterid(%s)\n", reread, cid)
		}
		cv.So(reread, cv.ShouldEqual, cid)

		// cleanup
		RemoveLocalClusterId(cfg)
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

			cfg := GetEnvConfig()
			InjectConfigIntoEnv(cfg)

			e2 := EnvToMap(os.Environ())
			cv.So(e2[`GOQ_SENDTIMEOUT_MSEC`], cv.ShouldEqual, fmt.Sprintf("%d", cfg.SendTimeoutMsec))
			cv.So(e2[`GOQ_JSERV_IP`], cv.ShouldEqual, cfg.JservIP)
			cv.So(e2[`GOQ_JSERV_PORT`], cv.ShouldEqual, fmt.Sprintf("%d", cfg.JservPort))
			cv.So(e2[`GOQ_NOSSHCONFIG`], cv.ShouldEqual, BoolToString(cfg.NoSshConfig))
		})
	})
}

func TestEnvCannotContainKey(t *testing.T) {
	cv.Convey("To avoid transmitting the clusterid, the Env sent to the shepard/worker cannot contain COG_ variables or the clusterid", t, func() {
		cv.Convey("The 8 GOQ env var should all be subtracted by GetNonGOQEnv(), as well as any variable that has the specified cid in it", func() {

			// *** universal test cfg setup
			skipbye := false
			cfg := NewTestConfig()
			defer cfg.ByeTestConfig(&skipbye)
			// *** end universal test setup

			e := make(map[string]string)
			cfg.InjectConfigIntoMap(&e)

			e["UNTOUCHED"] = "sane"
			randomCid := RandomClusterId()
			e["SHALLNOTPASS"] = randomCid

			env2 := MapToEnv(e)

			cv.So(len(env2), cv.ShouldEqual, 11) // the 8 from cfg + UNTOUCHED and SHALLNOTPASS
			res := GetNonGOQEnv(env2, randomCid)

			cv.So(len(res), cv.ShouldEqual, 1)
			cv.So(res[0], cv.ShouldEqual, "UNTOUCHED=sane")
		})
	})
}

func TestStartupMakesDotHomeDir(t *testing.T) {

	cv.Convey("Upon successful startup (call to ServerInit(cfg)), goq serve creates the GOQ_HOME/.goq directory", t, func() {
		cv.Convey("And that our clusterid gets written to disk there.", func() {
			// originate and cd into new temp dir

			// *** universal test cfg setup
			skipbye := false
			cfg := NewTestConfig()
			defer cfg.ByeTestConfig(&skipbye)
			// *** end universal test setup

			// ServerInit already called by NewTestConfig()
			//ServerInit(cfg)

			cidfn := ClusterIdFileName(cfg)

			dire := DirExists(".goq")
			cv.So(dire, cv.ShouldEqual, true)
			if dire {
				fmt.Printf("\n confirmed that %s/.goq was made.\n", cfg.Home)
			} else {
				fmt.Printf("\n problem: no %s/.goq was made.\n", cfg.Home)
			}
			idokay := FileExists(cfg.Home + "/.goq/" + cidfn)
			cv.So(idokay, cv.ShouldEqual, true)
			if idokay {
				fmt.Printf("\n confirmed that %s/.goq/%s was made.\n", cidfn, cfg.Home)
			} else {
				fmt.Printf("\n problem: %s/.goq/%s missing???\n", cidfn, cfg.Home)
			}

			readcid, err := ioutil.ReadFile(cfg.Home + "/.goq/" + cidfn)
			readcidstr := string(readcid)
			if err != nil {
				panic(err)
			}
			fmt.Printf("\n    And: the .goq/%s file should contain contents matching our .ClusterId\n", cidfn)
			cv.So(readcidstr, cv.ShouldEqual, cfg.ClusterId)

			fmt.Printf("\n    And: Out ClusterId should not be empty: '%s'.\n", readcidstr)
			cv.So(cfg.ClusterId, cv.ShouldNotEqual, "")

		})
	})
}
