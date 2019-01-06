package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/glycerine/idem"
	rpcxClient "github.com/smallnest/rpcx/client"
	rpcxProtocol "github.com/smallnest/rpcx/protocol"
)

var bkgCtx = context.Background()

// setup client via rpcx

type ClientRpcx struct {
	Cfg  *Config
	Halt *idem.Halter

	//Cli rpcxClient.XClient // interface
	Cli *rpcxClient.Client

	Discov         rpcxClient.ServiceDiscovery
	ReadIncomingCh chan *rpcxProtocol.Message

	options rpcxClient.Option

	prefix []byte
	mut    sync.Mutex
}

var clicount int

func NewClientRpcx(cfg *Config, infWait bool) (r *ClientRpcx, err error) {

	remoteAddr := cfg.JservAddrNoProto()

	options := rpcxClient.DefaultOption
	options.Retries = 1 // hmm. Seems we gotta have at least 1.
	options.ConnectTimeout = 10 * time.Second
	if infWait {
		options.ConnectTimeout = 0
	}
	if cfg.RecvTimeoutMsec > 0 {
		//panic(fmt.Sprintf("no! we should not have cfg.RecvTimeoutMsec set here to '%v'", cfg.RecvTimeoutMsec))
		// ReadTimeout sets readdeadline for underlying net.Conns
		//options.ReadTimeout = time.Duration(cfg.RecvTimeoutMsec) * time.Millisecond
	}
	// force this, otherwise our connections from the worker to the server timeout.
	options.ReadTimeout = 0
	options.WriteTimeout = 0

	if cfg.SendTimeoutMsec > 0 {
		//panic("not! don't send send timeouts either!")
		// WriteTimeout sets writedeadline for underlying net.Conns
		//options.WriteTimeout = time.Duration(cfg.SendTimeoutMsec) * time.Millisecond
	}

	fake := ""
	//fake := "alt/" // used to check that server would reject the wrong certs.

	gp := os.Getenv("GOPATH")
	base := fmt.Sprintf("%s/src/github.com/glycerine/goq/xrpc/", gp)

	sslCA := base + "certs/ca.crt"                      // path to CA cert
	sslCert := base + fake + "certs/client.root.crt"    // path to client cert
	sslCertKey := base + fake + "certs/client.root.key" // path to client key
	//vv("sslCA = '%v'", sslCA)
	//vv("sslCert = '%v'", sslCert)
	conf, err2 := LoadClientTLSConfig(sslCA, sslCert, sslCertKey)
	panicOn(err2)
	// under test vs...?
	// without this ServerName assignment, we get
	// 2019/01/04 09:36:18 failed to call: x509: cannot validate certificate for 127.0.0.1 because it doesn't contain any IP SANs
	conf.ServerName = "localhost"

	conf.InsecureSkipVerify = false // true would be insecure
	options.TLSConfig = conf

	// server push mechanism: how we receive them.
	readCh := make(chan *rpcxProtocol.Message)

	d := rpcxClient.NewPeer2PeerDiscovery("tcp@"+remoteAddr, "")

	r = &ClientRpcx{
		Cfg:            cfg,
		ReadIncomingCh: readCh,
		Discov:         d,
		options:        options,
		Halt:           idem.NewHalter(),
	}

	r.Cli = rpcxClient.NewClient(options)
	err = r.Cli.Connect("tcp", remoteAddr)
	if err != nil {
		return r, err
	}
	r.Cli.RegisterServerMessageChan(readCh)

	//vv("NewClientRpcx() returning with local addr '%s' and remote addr '%s'", r.Cli.Conn.LocalAddr().String(), r.Cli.Conn.RemoteAddr().String())

	return r, nil
}

func (c *ClientRpcx) LocalAddr() string {
	if c == nil || c.Cli == nil || c.Cli.Conn == nil {
		panic("cannot get address from nil Conn")
	}
	la := c.Cli.Conn.LocalAddr()
	return la.Network() + "://" + la.String()
}

func (c *ClientRpcx) DoSyncCallWithTimeout(to time.Duration, j *Job) (back *Job, sentSerz []byte, err error) {

	ctx, cancelFunc := context.WithCancel(context.Background())
	go func() {
		time.Sleep(to)
		cancelFunc()
	}()
	back, serz, err := c.DoSyncCallWithContext(ctx, j)
	cancelFunc()
	return back, serz, err
}

func (c *ClientRpcx) DoSyncCall(j *Job) (back *Job, sentSerz []byte, err error) {
	return c.DoSyncCallWithContext(context.Background(), j)
}

func (c *ClientRpcx) DoSyncCallWithContext(ctx context.Context, j *Job) (back *Job, sentSerz []byte, err error) {
	c.mut.Lock()
	defer c.mut.Unlock()

	sentSerz, err = c.Cfg.jobToBytesWithStamp(j)
	if err != nil {
		return nil, nil, err
	}
	//vv(`top of ClientRpcx.Write: about to do c.Cli.Call(context.Background(), "Ready", args, reply)`)
	defer func() {
		//vv("end of ClientRpcx.Write, nw=%v, err='%v'", nw, err)
	}()
	args := &Args{
		JobSerz: sentSerz,
	}

	reply := &Reply{}

	// from ~/go/src/github.com/smallnest/rpcx/client/xclient.go:355
	//
	// Call() invokes the named function, waits for it to complete, and returns its error status.
	// It handles errors base on FailMode.
	//
	// in constrast:
	// Go() invokes the function asynchronously. It returns the Call structure representing the invocation. The done channel will signal when the call is complete by returning the same Call object. If done is nil, Go will allocate a new channel. If non-nil, done must be buffered or Go will deliberately crash.
	// It does not use FailMode.
	//func (c *xClient) Go(ctx context.Context, serviceMethod string, args interface{}, reply interface{}, done chan *Call) (*Call, error)
	//
	//vv("client making the Call()")
	err = c.Cli.Call(ctx, "ServerCallbackMgr", "Ready", args, reply)
	if err != nil {
		return nil, nil, err
	}
	back, err = c.Cfg.bytesToJob(reply.JobSerz)
	return back, sentSerz, err
}

func (c *ClientRpcx) AsyncSend(j *Job) (*rpcxClient.Call, error) {
	c.mut.Lock()
	defer c.mut.Unlock()

	jobSerz, err := c.Cfg.jobToBytesWithStamp(j)
	if err != nil {
		return nil, err
	}
	//vv(`top of ClientRpcx.AsyncSend: about to do c.Cli.Go(context.Background(), "ServerCallbackMgr", "Ready", args, reply)`)
	defer func() {
		//vv("end of ClientRpcx.AsyncSend, nw=%v, err='%v'", nw, err)
	}()
	args := &Args{
		JobSerz: jobSerz,
	}

	reply := &Reply{}

	// from ~/go/src/github.com/smallnest/rpcx/client/xclient.go:355
	//
	// Call() invokes the named function, waits for it to complete, and returns its error status.
	// It handles errors base on FailMode.
	//
	// in constrast:
	// Go() invokes the function asynchronously. It returns the Call structure representing the invocation. The done channel will signal when the call is complete by returning the same Call object. If done is nil, Go will allocate a new channel. If non-nil, done must be buffered or Go will deliberately crash.
	// It does not use FailMode.
	//func (c *xClient) Go(ctx context.Context, serviceMethod string, args interface{}, reply interface{}, done chan *Call) (*Call, error)
	//
	//vv("client doing Go() in AsyncSend()")

	return c.Cli.Go(context.Background(), "ServerCallbackMgr", "Ready", args, reply, nil), nil
}

func (c *ClientRpcx) Close() error {
	//vv("ClientRpcx.Close() called.")
	c.Halt.ReqStop.Close()
	return c.Cli.Close()
}

/*
func (c *ClientRpcx) SetRecvTimeout(to time.Duration) error {
	return c.SetReadDeadline(time.Now().Add(to))
}

func (c *ClientRpcx) SetReadDeadline(tm time.Time) error {
	return c.Cli.Conn.SetReadDeadline(tm)
}

func (c *ClientRpcx) SetWriteDeadline(tm time.Time) error {
	return c.Cli.Conn.SetWriteDeadline(tm)
}
*/
