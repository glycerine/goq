package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"testing"
	"time"
)

// test-creation utils

/* how to setup a test:

// *** universal test cfg setup
skipbye := false
cfg := NewTestConfig()
defer cfg.ByeTestConfig(&skipbye)
// *** end universal test setup

remote := true // or false
defer CleanupServer(cfg, jobservPid, jobserv, remote, &skipbye)
defer CleanupOutdir(cfg)

// during tests, if you want to preserve output directories that would
// normally be mopped up by the deferred functions, set skipbye = true

*/

// get the origdir where the tests are being run from once,
// so that we are not likely to get flaked out on by tests
// running and the cwd changing and then we get two test dirs nested
// (which has happened, and crashed those  test runs).
// So we use a TestMain() func.
var origTestMainDir string

func TestMain(m *testing.M) {
	// Setup code

	var err error
	origTestMainDir, err = os.Getwd()
	if err != nil {
		panic(err)
	}

	// Run the tests
	exitCode := m.Run()

	// Teardown code
	//fmt.Println("Teardown code after running tests")

	// Exit with the appropriate code
	os.Exit(exitCode)
}

// make a new fake-home-temp-directory for testing
// and cd into it. Save GOQ_HOME for later restoration.
func NewTestConfig() *Config {
	cfg := NewConfig()

	cfg.origdir, cfg.tempdir = MakeAndMoveToTempDir() // cd to tempdir

	// link back to bin
	err := os.Symlink(cfg.origdir+"/bin", cfg.tempdir+"/bin")
	if err != nil {
		panic(err)
	}

	cfg.orighome = os.Getenv("GOQ_HOME")
	os.Setenv("GOQ_HOME", cfg.tempdir)

	cfg.Home = cfg.tempdir
	cfg.JservPort = 2776
	cfg.JservIP = GetExternalIP()
	cfg.DebugMode = true
	cfg.Odir = "o"
	cfg.SendTimeoutMsec = 1000
	cfg.RecvTimeoutMsec = 1000
	cfg.Heartbeat = 5

	// match outer env for UseQUIC
	cfg.UseQUIC = GetEnvBool("GOQ_USE_QUIC", false)

	GenNewCreds(cfg)

	WaitUntilAddrAvailable(cfg.JservAddr())

	// not needed. GOQ_HOME should suffice. InjectConfigIntoEnv(cfg)
	return cfg
}

// restore GOQ_HOME and previous working directory
// allow to skip if test goes awry, even if it was deferred.
func (cfg *Config) ByeTestConfig(skip *bool) {
	if skip != nil && !(*skip) {
		TempDirCleanup(cfg.origdir, cfg.tempdir)
		os.Setenv("GOQ_HOME", cfg.orighome)
	}
	VPrintf("\n ByeTestConfig done.\n")
}

func CleanupOutdir(cfg *Config) {
	if DirExists(cfg.Odir) {
		c := exec.Command("/bin/rm", "-rf", cfg.Odir)
		c.CombinedOutput()
	}
	VPrintf("\n CleanupOutdir '%s' done.\n", cfg.Odir)
}

// *important* cleanup, and wait for cleanup to finish, so the next test can run.
// skip lets us say we've already done this
func CleanupServer(cfg *Config, jobservPid int, jobserv *JobServ, remote bool, skip *bool) {

	if skip == nil || !*skip {
		if remote {
			SendShutdown(cfg)
			WaitForShutdownWithTimeout(jobservPid, cfg)

		} else {
			// this wait is really important!!! even locally! Otherwise the next test gets hosed
			// because the clients will connect to the old server which then dies.
			jobserv.Ctrl <- die
			<-jobserv.Done

		}

	}
	VPrintf("\n CleanupServer done.\n")
}

func MakeAndMoveToTempDir() (origdir string, tmpdir string) {

	// make new temp dir that will have no ".goqclusterid files in it
	var err error
	origdir = origTestMainDir
	tmpdir, err = ioutil.TempDir(origdir, "tempgoqtestdir")
	if err != nil {
		panic(err)
	}
	err = os.Chdir(tmpdir)
	if err != nil {
		panic(err)
	}

	return origdir, tmpdir
}

func TempDirCleanup(origdir string, tmpdir string) {
	// cleanup
	os.Chdir(origdir)
	err := os.RemoveAll(tmpdir)
	for i := 0; err != nil && i < 100; i++ {
		// probably somebody still writing. just try again; up
		// to 100 times (1 second).
		// reference: https://github.com/golang/go/issues/20841
		time.Sleep(10 * time.Millisecond)
		err = os.RemoveAll(tmpdir)
	}
	VPrintf("\n TempDirCleanup of '%s' done.\n", tmpdir)
}

func HelperNewJobServ(cfg *Config, remote bool) (jobserv *JobServ, jobservPid int) {

	var err error
	if remote {
		jobservPid, err = NewExternalJobServ(cfg)
		if err != nil {
			panic(err)
		}
		fmt.Printf("\n")
		fmt.Printf("[pid %d] spawned a new external JobServ with pid %d\n", os.Getpid(), jobservPid)

	} else {
		jobserv, err = NewJobServ(cfg)
		if err != nil {
			panic(err)
		}
	}

	return
}

func HelperNewWorker(cfg *Config) *Worker {
	worker, err := NewWorker(cfg, nil)
	if err != nil {
		panic(err)
	}
	return worker
}

func HelperNewWorkerMonitored(cfg *Config) *Worker {
	worker, err := NewWorker(cfg, &WorkOpts{Monitor: true})
	if err != nil {
		panic(err)
	}
	return worker
}

func HelperNewWorkerDontStart(cfg *Config) *Worker {
	worker, err := NewWorker(cfg, &WorkOpts{DontStart: true})
	if err != nil {
		panic(err)
	}
	return worker
}

func HelperNewWorkerDeaf(cfg *Config) *Worker {
	worker, err := NewWorker(cfg, &WorkOpts{IsDeaf: true})
	if err != nil {
		panic(err)
	}
	return worker
}

func HelperSnapmap(cfg *Config) map[string]string {
	serverSnap, err := SubmitGetServerSnapshot(cfg)
	if err != nil {
		panic(err)
	}
	return EnvToMap(serverSnap)
}

func HelperSubJob(j *Job, cfg *Config) (sub *Submitter) {
	sub, err := NewSubmitter(cfg, false)
	if err != nil {
		panic(err)
	}
	sub.SubmitJob(j)
	return sub
}

func HelperSubJobGetReply(j *Job, cfg *Config) (sub *Submitter, reply *Job) {
	sub, err := NewSubmitter(cfg, false)
	if err != nil {
		panic(err)
	}
	reply, _, err = sub.SubmitJobGetReply(j)
	if err != nil {
		panic(err)
	}
	return sub, reply
}
