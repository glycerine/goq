package main

import (
	"fmt"
	"os"

	schema "github.com/glycerine/goq/schema"
)

// Submitter represents all other queries beside those from workers.
// Their principle purpose is to supply jobs to-be-done using JOBMSG_INITIALSUBMIT.
//
// However, submitters can do many other miscelanous other things.
// They can query the server for a snapshot of status using
// JOBMSG_TAKESNAPSHOT, for instance. The 'goq stat' command issues that query.
//
type Submitter struct {
	Name string
	Addr string
	Cli  *ClientRpcx

	ServerName string
	ServerAddr string

	ToServerSubmit chan *Job

	// set Cfg *once*, before any goroutines start, then
	// treat it as immutable and never changing.
	Cfg         Config
	LastSentMsg []byte
}

func NewSubmitter(cfg *Config, infWait bool) (*Submitter, error) {

	cli, err := NewClientRpcx(cfg, infWait)
	if err != nil {
		panic(err)
	}
	localAddr := cli.LocalAddr()

	sub := &Submitter{
		Name: fmt.Sprintf("submitter.pid.%d", os.Getpid()),
		Addr: localAddr,
		Cli:  cli,
		Cfg:  *CopyConfig(cfg),
	}

	return sub, nil
}

func (sub *Submitter) Bye() {
	if sub.Cli != nil {
		sub.Cli.Close()
		sub.Cli = nil // allow 2x Bye() during shutdown.
	}
}

func (sub *Submitter) SubmitJob(j *Job) {
	j.Msg = schema.JOBMSG_INITIALSUBMIT
	j.Submitaddr = sub.Addr
	if sub.Addr != "" {

		_, errsend := sub.Cli.AsyncSend(j)
		if errsend != nil {
			panic(fmt.Errorf("err during submit job: %s\n", errsend))
		}
		//sub.LastSentMsg = cy
	} else {
		sub.ToServerSubmit <- j
	}
}

func (sub *Submitter) SubmitJobGetReply(j *Job) (*Job, []byte, error) {
	j.Msg = schema.JOBMSG_INITIALSUBMIT
	j.Submitaddr = sub.Addr

	// don't pass the env along anymore, let that be set locally.
	// used to be: grab the local env, without any GOQ stuff.
	// j.Env = GetNonGOQEnv(os.Environ(), sub.Cfg.ClusterId)

	if sub.Addr != "" {
		return sub.Cli.DoSyncCall(j)
	} else {
		sub.ToServerSubmit <- j
	}
	return nil, nil, nil
}

// returns only after the request for job has been registered (assuming error is nil)
// i.e. if error is non-nil, request might not have gone through.
// if there was an error, the chan will supply a job with Id == -1 and
// an error message in Out[0]
func (sub *Submitter) WaitForJob(jobidToWaitFor int64) (chan *Job, error) {
	res := make(chan *Job)

	j := NewJob()
	j.Msg = schema.JOBMSG_OBSERVEJOBFINISH
	j.Submitaddr = sub.Addr
	j.Aboutjid = jobidToWaitFor
	if sub.Addr != "" {
		_, _, err := sub.Cli.DoSyncCall(j)
		panicOn(err)
		close(res)
		return res, err
	} else {
		sub.ToServerSubmit <- j
	}
	return res, nil
}

func (sub *Submitter) SubmitShutdownJob() error {
	j := NewJob()
	j.Msg = schema.JOBMSG_SHUTDOWNSERV
	j.Submitaddr = sub.Addr
	j.Serveraddr = sub.ServerAddr

	// try to speed up the timeout, when its already down.
	//sub.SetServerPushTimeoutMsec(100)

	if sub.Addr != "" {
		_, _, err := sub.Cli.DoSyncCall(j)
		return err
	} else {
		sub.ToServerSubmit <- j
	}
	return nil
}

func (sub *Submitter) SubmitSnapJob(maxShow int) ([]string, error) {
	j := NewJob()
	j.Msg = schema.JOBMSG_TAKESNAPSHOT
	j.Submitaddr = sub.Addr
	j.Serveraddr = sub.ServerAddr
	j.MaxShow = int64(maxShow)
	if AesOff {
		j.Out = append(j.Out, "clusterid:"+sub.Cfg.ClusterId)
	}

	if sub.Addr != "" {
		//sub.Cli.SetRecvTimeout(60000 * time.Millisecond) // wait 60 seconds
		jstat, _, err := sub.Cli.DoSyncCall(j)
		// return to normal? sub.Cli.SetRecvTimeout(time.Duration(cfg.RecvTimeoutMsec) * time.Millisecond)
		if err == nil {
			return jstat.Out, nil
		}
		fmt.Printf("\n err in SubmitSnapJob on receiving reply: '%s'\n", err)
		return []string{}, err
	} else {
		fmt.Printf("local server stat not implemented.\n")
		//sub.ToServerSubmit <- j
	}
	return []string{}, nil
}

/*
func (sub *Submitter) setServerPrivate(pushaddr string) error {

	var err error
	var pushsock *ClientRpcx
	if pushaddr != "" {
		pushsock, err = NewClientRpcx(pushaddr, &sub.Cfg, false)
		if err != nil {
			panic(err)
			//return err
		}
		sub.ServerAddr = pushaddr
		if sub.Cli != nil {
			sub.Cli.Close()
		}
		sub.Cli = pushsock
		sub.ServerName = "JSERV"
	}
	return nil
}

func (sub *Submitter) SetServerPushTimeoutMsec(msec int) error {
	// crashing, so disable for now.
	//return sub.Cli.SetSendTimeout(time.Duration(msec) * time.Millisecond)
	return nil
}
*/

func NewLocalSubmitter(js *JobServ) (*Submitter, error) {
	sub := &Submitter{
		ToServerSubmit: js.Submit,
	}
	return sub, nil
}

func SendKill(cfg *Config, jid int64) {
	sub, err := NewSubmitter(cfg, false)
	if err != nil {
		panic(err)
	}
	sub.SubmitKillJob(jid)
}

func (sub *Submitter) SubmitKillJob(jid int64) {
	j := NewJob()
	j.Msg = schema.JOBMSG_CANCELSUBMIT
	j.Submitaddr = sub.Addr
	j.Serveraddr = sub.ServerAddr
	j.Aboutjid = jid

	//sub.SetServerPushTimeoutMsec(100)

	if sub.Addr != "" {
		jconfirm, _, err := sub.Cli.DoSyncCall(j)
		if err == nil {
			if jconfirm.Msg == schema.JOBMSG_ACKCANCELSUBMIT {
				VPrintf("[pid %d] cancellation of job %d at '%s' succeeded.\n", os.Getpid(), jid, sub.ServerAddr)
			}
		}

	} else {
		sub.ToServerSubmit <- j
	}
}

func (sub *Submitter) SubmitImmoJob() error {
	j := NewJob()
	j.Msg = schema.JOBMSG_IMMOLATEAWORKERS
	j.Submitaddr = sub.Addr
	j.Serveraddr = sub.ServerAddr
	if AesOff {
		j.Out = append(j.Out, "clusterid:"+sub.Cfg.ClusterId)
	}

	if sub.Addr != "" {
		jimmoack, _, err := sub.Cli.DoSyncCall(j)
		if err != nil {
			return err
		}
		if jimmoack.Msg != schema.JOBMSG_IMMOLATEACK {
			panic(fmt.Sprintf("expected JOBMSG_IMMOLATEACK but got: %s", jimmoack))
		}
		return nil
	} else {
		fmt.Printf("local server 'immolate workers' not implemented.\n")
	}
	return nil
}

func (sub *Submitter) SubmitCancelJob(jid int64) error {
	j := NewJob()
	j.Msg = schema.JOBMSG_CANCELSUBMIT
	j.Aboutjid = jid
	j.Submitaddr = sub.Addr
	j.Serveraddr = sub.ServerAddr
	if AesOff {
		j.Out = append(j.Out, "clusterid:"+sub.Cfg.ClusterId)
	}

	if sub.Addr != "" {
		jimmoack, _, err := sub.Cli.DoSyncCall(j)
		_ = jimmoack
		_ = err
		/* might timeout
		jimmoack, err := recvZjob(sub.Cli, &sub.Cfg)
		if err != nil {
			fmt.Printf("error during receiving confirmation of cancel job: '%s'\n", err)
			return err
		}
		if jimmoack.Msg != schema.JOBMSG_ACKCANCELSUBMIT {
			panic(fmt.Sprintf("expected JOBMSG_ACKCANCELSUBMIT but got: %s", jimmoack))
		}
		*/

		return nil
	} else {
		fmt.Printf("local server 'cancelsubmit' not implemented.\n")
	}
	return nil
}
