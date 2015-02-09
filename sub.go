package main

import (
	"fmt"
	"os"
	"time"

	schema "github.com/glycerine/goq/schema"
	//nn "github.com/glycerine/go-nanomsg"
	nn "github.com/glycerine/mangos/compat"
)

// Submitter represents all other queries beside those from workers.
// Their principle purpose is to supply jobs to-be-done using JOBMSG_INITIALSUBMIT.
//
// However, submitters can do many other miscelanous other things.
// They can query the server for a snapshot of status using
// JOBMSG_TAKESNAPSHOT, for instance. The 'goq stat' command issues that query.
//
type Submitter struct {
	Name   string
	Addr   string
	Nnsock *nn.Socket

	ServerName     string
	ServerAddr     string
	ServerPushSock *nn.Socket

	ToServerSubmit chan *Job

	// set Cfg *once*, before any goroutines start, then
	// treat it as immutable and never changing.
	Cfg         Config
	LastSentMsg []byte
}

func NewSubmitter(pulladdr string, cfg *Config, infWait bool) (*Submitter, error) {

	var err error

	var pullsock *nn.Socket
	if pulladdr != "" {
		pullsock, err = MkPullNN(pulladdr, cfg, infWait)
		if err != nil {
			panic(err)
		}
	}
	sub := &Submitter{
		Name:   fmt.Sprintf("submitter.pid.%d", os.Getpid()),
		Addr:   pulladdr,
		Nnsock: pullsock,
		Cfg:    *CopyConfig(cfg),
	}

	sub.setServerPrivate(cfg.JservAddr())

	return sub, nil
}

func (sub *Submitter) Bye() {
	if sub.Nnsock != nil {
		sub.Nnsock.Close()
		sub.Nnsock = nil // allow 2x Bye() during shutdown.
	}
	if sub.ServerPushSock != nil {
		sub.ServerPushSock.Close()
		sub.ServerPushSock = nil
	}
}

func (sub *Submitter) SubmitJob(j *Job) {
	j.Msg = schema.JOBMSG_INITIALSUBMIT
	j.Submitaddr = sub.Addr
	if sub.Addr != "" {
		cy, errsend := sendZjob(sub.ServerPushSock, j, &sub.Cfg)
		if errsend != nil {
			panic(fmt.Errorf("err during submit job: %s\n", errsend))
		}
		sub.LastSentMsg = cy
	} else {
		sub.ToServerSubmit <- j
	}
}

func (sub *Submitter) SubmitJobGetReply(j *Job) (*Job, error) {
	j.Msg = schema.JOBMSG_INITIALSUBMIT
	j.Submitaddr = sub.Addr

	// don't pass the env along anymore, let that be set locally.
	// used to be: grab the local env, without any GOQ stuff.
	// j.Env = GetNonGOQEnv(os.Environ(), sub.Cfg.ClusterId)

	if sub.Addr != "" {
		cy, errsend := sendZjob(sub.ServerPushSock, j, &sub.Cfg)
		if errsend != nil {
			r := fmt.Errorf("err during submit job: %s\n", errsend)
			//fmt.Printf("%s\n", r)
			return nil, r
		}
		sub.LastSentMsg = cy
		reply, err := recvZjob(sub.Nnsock, &sub.Cfg)
		return reply, err
	} else {
		sub.ToServerSubmit <- j
	}
	return nil, nil
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

		_, err := sendZjob(sub.ServerPushSock, j, &sub.Cfg)
		if err != nil {
			close(res)
			return res, err
		}

		//		infPullAddr := GenAddress()
		//		infPullsock, err := MkPullNN(infPullAddr, &sub.Cfg, true)
		//		if err != nil {
		//			panic(err)
		//		}

		go func(sock *nn.Socket) {
			defer sock.Close()
			reply, err := recvZjob(sock, &sub.Cfg)
			if err != nil {
				errjob := NewJob()
				errjob.Id = -1
				errjob.Out = []string{err.Error()}
				res <- errjob
				close(res)
				return
			}
			res <- reply
			close(res)
		}(sub.Nnsock)
	} else {
		sub.ToServerSubmit <- j
	}
	return res, nil
}

func (sub *Submitter) SubmitShutdownJob() {
	j := NewJob()
	j.Msg = schema.JOBMSG_SHUTDOWNSERV
	j.Submitaddr = sub.Addr
	j.Serveraddr = sub.ServerAddr

	// try to speed up the timeout, when its already down.
	sub.SetServerPushTimeoutMsec(100)

	if sub.Addr != "" {
		sendZjob(sub.ServerPushSock, j, &sub.Cfg)
	} else {
		sub.ToServerSubmit <- j
	}
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
		sendZjob(sub.ServerPushSock, j, &sub.Cfg)

		sub.Nnsock.SetRecvTimeout(60000 * time.Millisecond) // wait 60 seconds
		jstat, err := recvZjob(sub.Nnsock, &sub.Cfg)
		// return to normal? sub.Nnsock.SetRecvTimeout(time.Duration(cfg.RecvTimeoutMsec) * time.Millisecond)
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

func (sub *Submitter) setServerPrivate(pushaddr string) error {

	var err error
	var pushsock *nn.Socket
	if pushaddr != "" {
		pushsock, err = MkPushNN(pushaddr, &sub.Cfg, false)
		if err != nil {
			panic(err)
			//return err
		}
		sub.ServerAddr = pushaddr
		if sub.ServerPushSock != nil {
			sub.ServerPushSock.Close()
		}
		sub.ServerPushSock = pushsock
		sub.ServerName = "JSERV"
	}
	return nil
}

func (sub *Submitter) SetServerPushTimeoutMsec(msec int) error {
	// crashing, so disable for now.
	//return sub.ServerPushSock.SetSendTimeout(time.Duration(msec) * time.Millisecond)
	return nil
}

func NewLocalSubmitter(js *JobServ) (*Submitter, error) {
	sub := &Submitter{
		ToServerSubmit: js.Submit,
	}
	return sub, nil
}

func SendKill(cfg *Config, jid int64) {
	sub, err := NewSubmitter(GenAddress(), cfg, false)
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

	sub.SetServerPushTimeoutMsec(100)

	if sub.Addr != "" {
		sendZjob(sub.ServerPushSock, j, &sub.Cfg)
		jconfirm, err := recvZjob(sub.Nnsock, &sub.Cfg)
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
		sendZjob(sub.ServerPushSock, j, &sub.Cfg)
		jimmoack, err := recvZjob(sub.Nnsock, &sub.Cfg)
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
		sendZjob(sub.ServerPushSock, j, &sub.Cfg)
		recvZjob(sub.Nnsock, &sub.Cfg)

		/* might timeout
		jimmoack, err := recvZjob(sub.Nnsock, &sub.Cfg)
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
