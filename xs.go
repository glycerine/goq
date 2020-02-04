package main

import (
	//"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	rpcx "github.com/smallnest/rpcx/server"
)

// rpcx server stuff

type ServerCallbackMgr struct {
	Srv     *rpcx.Server
	cfg     *Config
	mut     sync.Mutex
	connMap map[string]*Connection

	jserv *JobServ
}

type Connection struct {
	Key     string // e.g. "tcp://127.0.0.1:34343"
	Nc      net.Conn
	ReplyCh chan *Reply
}

func (m *ServerCallbackMgr) removeClient(addr string) (del *Connection) {
	m.mut.Lock()
	defer m.mut.Unlock()
	var ok bool
	del, ok = m.connMap[addr]
	if ok {
		delete(m.connMap, addr)
	}
	return
}

var ErrNotAvail = fmt.Errorf("addr not available in Mgr connMap")

func (m *ServerCallbackMgr) get(addr string) (*Connection, error) {
	m.mut.Lock()
	defer m.mut.Unlock()

	if !strings.Contains(addr, "://") {
		addr = "tcp://" + addr
	}

	k, ok := m.connMap[addr]
	if !ok {
		return nil, ErrNotAvail
	}
	return k, nil
}

// addr is like localhost:8972
func NewServerCallbackMgr(addr string, cfg *Config) (m *ServerCallbackMgr, err error) {
	//vv("top of NewServerCallbackMgr")
	suf, err := StripNanomsgAddressPrefix(addr)
	if err == nil {
		addr = suf
	}

	// load up certs for TLS
	gp := os.Getenv("GOPATH")
	base := fmt.Sprintf("%s/src/github.com/glycerine/goq/xrpc/", gp)

	sslCA := base + "certs/ca.crt"        // path to CA cert
	sslClientCA := base + "certs/ca.crt"  // path to CA cert to verify client certs, can be same as sslCA
	sslCert := base + "certs/node.crt"    // path to server cert
	sslCertKey := base + "certs/node.key" // path to server key
	conf, err := LoadServerTLSConfig(sslCA, sslClientCA, sslCert, sslCertKey)
	panicOn(err)
	// TODO: fix this?
	conf.ServerName = "localhost"

	conf.ClientAuth = tls.RequireAndVerifyClientCert
	//insecure to turn off client cert checking with: conf.ClientAuth = tls.NoClientCert

	s := rpcx.NewServer(rpcx.WithTLSConfig(conf))
	m = &ServerCallbackMgr{
		connMap: make(map[string]*Connection),
		Srv:     s,
		cfg:     cfg,
	}
	s.RegisterName("ServerCallbackMgr", m, "")
	//s.Register(new(Mgr), "")
	waitForErr := make(chan bool)
	var serr error
	go func() {
		serr = s.Serve("tcp", addr)
		//vv("error starting rpcx server: '%s'", serr)
		// normal to see serr.Error() == "http: Server closed" here.
		close(waitForErr)
	}()
	// try hard to catch errors during server startup;
	// at least for 50 msec.
	select {
	case <-waitForErr:
		err = serr
	case <-time.After(50 * time.Millisecond):
	}
	//vv("end of NewServerCallbackMgr: err='%v'", err)
	return m, err
}

type Args struct {
	A       int64
	B       int64
	JobSerz []byte
}

type Reply struct {
	C       int64
	JobSerz []byte
	Err     error
}

// notice disconnected client connections, return these in disco
func (m *ServerCallbackMgr) gcRegistry() (disco []*Connection) {
	active := m.Srv.ActiveClientConn() // []net.Conn

	activeMap := make(map[string]bool)
	for _, a := range active {
		akey := netConnRemoteAddrAsKey(a)
		activeMap[akey] = true
	}

	m.mut.Lock()
	defer m.mut.Unlock()

	// which do we have that are not active?
	for k, cn := range m.connMap {
		if !activeMap[k] {
			disco = append(disco, cn)
			delete(m.connMap, k)
		}
	}
	return
}

func (m *ServerCallbackMgr) register(clientConn net.Conn, replyCh chan *Reply) {
	rkey := netConnRemoteAddrAsKey(clientConn)

	//vv("registering client conn under rkey '%s'", rkey)
	m.mut.Lock()
	defer m.mut.Unlock()

	c := &Connection{Nc: clientConn, ReplyCh: replyCh}
	m.connMap[rkey] = c
}

func netConnRemoteAddrAsKey(nc net.Conn) string {
	ra := nc.RemoteAddr()
	return ra.Network() + "://" + ra.String()
}

func (m *ServerCallbackMgr) pushJobToClient(addr string, j *Job) (key string, ok bool, err error) {

	nc, err := m.get(addr)
	if err != nil {
		panic(fmt.Sprintf("could not find cache net.Conn for addr '%s'", addr))
		return addr, false, err
	}
	jobSerz, err := m.cfg.jobToBytesWithStamp(j)
	if err != nil {
		return addr, false, err
	}
	return m.pushToClient(nc, jobSerz)
}

func (m *ServerCallbackMgr) pushToClient(conn *Connection, by []byte) (key string, ok bool, err error) {
	nc := conn.Nc
	key = netConnRemoteAddrAsKey(nc)

	//vv("server: pushing to '%s'", key)
	// if a synchronous call is waiting; release it.
	if conn.ReplyCh != nil {
		//vv("using conn.ReplyCh to finish a synchronous Call into the server.")
		select {
		case conn.ReplyCh <- &Reply{JobSerz: by}:
		default:
		}
	}

	//vv("doing m.Srv.SendMessage()")
	err = m.Srv.SendMessage(nc, "test_service_path", "test_service_method", nil, by)
	if err == nil {
		ok = true
	} else {
		//vv("failed to send messsage to %s: %v\n", key, err)
		//if strings.Contains(err.Error(), "use of closed connection")
		nc.Close()
		m.removeClient(key)
	}

	return
}

func (m *ServerCallbackMgr) Ready(ctx context.Context, args *Args, reply *Reply) error {
	//vv("ServerCallbackMgr: Ready() top.")
	clientConn := ctx.Value(rpcx.RemoteConnContextKey).(net.Conn)

	//reply.C = args.A * args.B
	//vv("server sees args.A=%v, args.B=%v, setting reply.C=%v", args.A, args.B, reply.C)

	var job *Job
	var err error
	// harden against cross-cluster communication
	defer func() {
		if recover() != nil {
			job = nil
			err = fmt.Errorf("unknown recovered error on receive")
		}
	}()

	// Read job submitted to the server
	job, err = m.cfg.bytesToJob(args.JobSerz)
	if err != nil {
		return fmt.Errorf("error in ServerCallbackMgr.Ready() after CapnpToJob: '%v'", err)
	}
	job.nc = clientConn

	//vv("ServerCallbackMgr: Ready() sees incoming job: '%s'", job.String())

	job.replyCh = make(chan *Reply)
	m.register(clientConn, job.replyCh)

	select {
	case m.jserv.FromRpcxServer <- job:
	case <-m.jserv.ListenerShutdown:
		return nil
	}

	select { // hung here in server when "goq sub" client stalls
	case pReply := <-job.replyCh:
		//vv("server Ready() got pReply")
		reply.JobSerz = pReply.JobSerz
		reply.Err = pReply.Err
	case <-m.jserv.ListenerShutdown:
		return nil
	}

	//vv("doing normal return from Server.Ready(). len(reply.JobSerz)=%v", len(reply.JobSerz))
	// the `reply` value has the reply back to (worker, submitter) at this point.
	return nil
}

func (m *ServerCallbackMgr) TestReady(ctx context.Context, args *Args, reply *Reply) error {
	//vv("ServerCallbackMgr: TestReady() top.")
	clientConn := ctx.Value(rpcx.RemoteConnContextKey).(net.Conn)
	m.register(clientConn, nil)
	//vv("server sees args.JobSerz='%v'", string(args.JobSerz))

	reply.JobSerz = []byte("synchronous reply")

	err := m.Srv.SendMessage(clientConn, "test_service_path", "test_service_method", nil, []byte(fmt.Sprintf("async reply")))
	panicOn(err)

	return nil
}
