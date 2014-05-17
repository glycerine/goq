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

func init() {
	// don't use init, you don't know yet what temp directory your test will be running in.
	//InitTestEnv()
}

// for testing, set the required GOQ_HOME variable to local dir,
// so we have a place to store our certs, etc.
func InitTestEnv() {
	pwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	testhome := pwd
	envhome := os.Getenv("GOQ_HOME")
	if envhome == testhome {
		panic(fmt.Sprintf("error: refusing to do testing in current working directory('%s'), as it appears to be a production directory according to GOQ_HOME('%s')", testhome, envhome))
	}

	// cleanup any old .goq test dir
	testcfg := DefaultCfg()
	testcfg.Home = testhome

	DeleteDotGoqDir(testcfg)

	fmt.Printf("for testing, ignoring GOQ_HOME='%s'; using '%s' instead.\n", envhome, testhome)

	err = os.Setenv("GOQ_HOME", testhome)
	if err != nil {
		panic(err)
	}
	err = MakeDotGoqDir(testcfg)
	if err != nil {
		panic(err)
	}

	if testcfg.Cypher == nil {
		// gen key
		key, err := NewKey(testcfg)
		if err != nil {
			panic(err)
		}
		testcfg.Cypher = key
	}

}

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
		SaveLocalClusterId(cid, cfg)
		reread := LoadLocalClusterId(cfg)

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

func TestEnvCannotContainKey(t *testing.T) {
	cv.Convey("To avoid transmitting the clusterid, the Env sent to the shepard/worker cannot contain COG_ variables or the clusterid", t, func() {
		cv.Convey("The 8 GOQ env var should all be subtracted by GetNonGOQEnv(), as well as any variable that has the specified cid in it", func() {

			cfg := DefaultCfg()
			e := make(map[string]string)
			cfg.InjectConfigIntoMap(&e)

			e["UNTOUCHED"] = "sane"
			randomCid := RandomClusterId()
			e["SHALLNOTPASS"] = randomCid

			env2 := MapToEnv(e)

			cv.So(len(env2), cv.ShouldEqual, 10) // the 8 from cfg + UNTOUCHED and SHALLNOTPASS
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
			origdir, tmpdir := MakeAndMoveToTempDir()

			pwd, err := os.Getwd()
			if err != nil {
				panic(err)
			}

			testcfg := DefaultCfg()
			testcfg.Home = pwd

			ServerInit(testcfg)

			cidfn := ClusterIdFileName(testcfg)

			dire := DirExists(".goq")
			cv.So(dire, cv.ShouldEqual, true)
			if dire {
				fmt.Printf("\n confirmed that %s/.goq was made.\n", pwd)
			} else {
				fmt.Printf("\n problem: no %s/.goq was made.\n", pwd)
			}
			idokay := FileExists(pwd + "/.goq/" + cidfn)
			cv.So(idokay, cv.ShouldEqual, true)
			if idokay {
				fmt.Printf("\n confirmed that %s/.goq/%s was made.\n", cidfn, pwd)
			} else {
				fmt.Printf("\n problem: %s/.goq/%s missing???\n", cidfn, pwd)
			}

			readcid, err := ioutil.ReadFile(pwd + "/.goq/" + cidfn)
			readcidstr := string(readcid)
			if err != nil {
				panic(err)
			}
			fmt.Printf("\n    And: the .goq/%s file should contain contents matching our testcfg.ClusterId\n", cidfn)
			cv.So(readcidstr, cv.ShouldEqual, testcfg.ClusterId)

			// cleanup
			TempDirCleanup(origdir, tmpdir)
		})
	})
}

func MakeAndMoveToTempDir() (origdir string, tmpdir string) {

	// make new temp dir that will have no ".goqclusterid files in it
	var err error
	origdir, err = os.Getwd()
	if err != nil {
		panic(err)
	}
	tmpdir, err = ioutil.TempDir(".", "tempgoqtestdir")
	if err != nil {
		panic(err)
	}
	os.Chdir(tmpdir)

	return origdir, tmpdir
}

func TempDirCleanup(origdir string, tmpdir string) {
	// cleanup
	os.Chdir(origdir)
	err := os.RemoveAll(tmpdir)
	if err != nil {
		panic(err)
	}
}
