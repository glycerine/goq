package main

import (
	"context"
	"fmt"
	//"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/glycerine/idem"
	rpc "github.com/glycerine/rpc25519"
)

var bkgCtx = context.Background()

// setup client via rpc

type ClientRpc struct {
	Cfg  *Config
	Halt *idem.Halter

	Cli *rpc.Client

	ReadIncomingCh chan *rpc.Message

	options rpc.Config

	prefix []byte
	mut    sync.Mutex
}

var clicount int

func NewClientRpc(name string, cfg *Config, infWait bool) (r *ClientRpc, err error) {

	remoteAddr := cfg.JservAddrNoProto()
	//vv("NewClientRpc called with remoteAddr '%v'", remoteAddr)

	tcp := insecure
	if cfg.UseQUIC {
		tcp = false
	}

	options := rpc.NewConfig()
	options.ClientDialToHostPort = remoteAddr
	options.TCPonly_no_TLS = tcp
	options.UseQUIC = cfg.UseQUIC
	options.CertPath = fixSlash(cfg.Home + "/.goq/certs")

	options.ConnectTimeout = 10 * time.Second
	if infWait {
		options.ConnectTimeout = 0
	}

	cli, err := rpc.NewClient(name, options)
	if err != nil {
		return nil, err
	}
	if cli == nil {
		return nil, fmt.Errorf("got nil rpc.Client back from rpc.NewClient(name='%v', options='%#v')", name, options)
	}
	// server push mechanism: how we receive them.
	readCh := cli.GetReadIncomingCh()

	r = &ClientRpc{
		Cfg:            cfg,
		ReadIncomingCh: readCh,
		//Discov:         d,
		options: *options,
		Halt:    idem.NewHalter(),
	}
	r.Cli = cli

	if cli.Conn != nil { // was nil once on OSX.
		vv("NewClient() returning with local addr '%s' and remote addr '%s'", r.Cli.Conn.LocalAddr().String(), r.Cli.Conn.RemoteAddr().String())
	}
	return r, nil
}

func (c *ClientRpc) LocalAddr() string {
	return c.Cli.LocalAddr()
}

func (c *ClientRpc) DoSyncCallWithTimeout(to time.Duration, j *Job) (back *Job, sentSerz []byte, err error) {
	//vv("begin DoSynCallWithTimeout")
	//defer vv("finishing DoSynCallWithTimeout")
	ctx, cancelFunc := context.WithCancel(context.Background())
	go func() {
		time.Sleep(to)
		cancelFunc()
	}()
	back, serz, err := c.DoSyncCallWithContext(ctx, j)
	cancelFunc()
	return back, serz, err
}

func (c *ClientRpc) DoSyncCall(j *Job) (back *Job, sentSerz []byte, err error) {
	return c.DoSyncCallWithContext(context.Background(), j)
}

func (c *ClientRpc) DoSyncCallWithContext(ctx context.Context, j *Job) (back *Job, sentSerz []byte, err error) {
	c.mut.Lock()
	defer c.mut.Unlock()

	sentSerz, err = c.Cfg.jobToBytesWithStamp(j)
	if err != nil {
		return nil, nil, err
	}
	args := rpc.NewMessage()
	args.JobSerz = sentSerz

	args.Subject = j.Msg.String()

	reply, err := c.Cli.SendAndGetReply(args, ctx.Done())

	if err != nil {
		return nil, nil, err
	}
	back, err = c.Cfg.bytesToJob(reply.JobSerz)
	return back, sentSerz, err
}

func (c *ClientRpc) AsyncSend(j *Job) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	jobSerz, err := c.Cfg.jobToBytesWithStamp(j)
	if err != nil {
		return err
	}
	args := rpc.NewMessage()
	args.Subject = fmt.Sprintf("client.AsyncSend('%v')", j.Msg.String())
	args.JobSerz = jobSerz
	return c.Cli.OneWaySend(args, nil)
}

func (c *ClientRpc) Close() error {
	c.Halt.ReqStop.Close()
	return c.Cli.Close()
}

func fixSlash(s string) string {
	if runtime.GOOS != "windows" {
		return s
	}
	return strings.Replace(s, "/", "\\", -1)
}
