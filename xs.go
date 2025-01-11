package main

import (
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	schema "github.com/glycerine/goq/schema"
	rpc "github.com/glycerine/rpc25519"
	//"github.com/quic-go/quic-go"
)

var insecure = false

// ServerCallbackMgr handles interacting with the rpc25519
// server for transport.
type ServerCallbackMgr struct {
	Srv     *rpc.Server
	cfg     *Config
	mut     sync.Mutex
	connMap map[string]*Nexus

	jserv *JobServ
}

// Nexus tracks clients we have seen.
type Nexus struct {
	Key     string // e.g. "tcp://127.0.0.1:34343"
	Nc      net.Conn
	Seqno   uint64 // of the request, not yet incremented.
	ReplyCh chan *rpc.Message
}

func (m *ServerCallbackMgr) removeClient(addr string) (del *Nexus) {
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

func (m *ServerCallbackMgr) get(addr string) (*Nexus, error) {
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

	tcp := insecure
	if cfg.UseQUIC {
		tcp = false
	}
	scfg := rpc.NewConfig()
	scfg.ServerAddr = addr
	scfg.TCPonly_no_TLS = tcp
	scfg.UseQUIC = cfg.UseQUIC
	scfg.CertPath = fixSlash(cfg.Home + "/.goq/certs")
	scfg.PreSharedKeyPath = fixSlash(cfg.Home + "/.goq/goqclusterid")
	scfg.ServerSendKeepAlive = 10 * time.Second

	serverName := os.Getenv("GOQ_TESTNAME") // which test is not closing server?
	s := rpc.NewServer(serverName, scfg)

	m = &ServerCallbackMgr{
		connMap: make(map[string]*Nexus),
		Srv:     s,
		cfg:     cfg,
	}

	// we now call start below separately, to avoid data race.
	return m, err
}

func (m *ServerCallbackMgr) start() error {
	s := m.Srv
	gotAddr, err := s.Start()
	_ = gotAddr
	//vv("rpc server start got addr='%v'; err='%v'", gotAddr, err)

	// Ready handles all callbacks from rpc25519.
	s.Register2Func("Ready2", m.Ready2)
	s.Register1Func("Ready1", m.Ready1)
	return err
}

/* jea: never called, comment out. But some kind of gc might be needed
// with alot of workers in the future.
// notice disconnected client connections, return these in disco
func (m *ServerCallbackMgr) gcRegistry() (disco []*Nexus) {
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
*/

func (m *ServerCallbackMgr) register(clientConn net.Conn, seqno uint64, replyCh chan *rpc.Message) {
	rkey := netConnRemoteAddrAsKey(clientConn)

	//vv("registering client conn under rkey '%s'", rkey)
	m.mut.Lock()
	defer m.mut.Unlock()

	c := &Nexus{Nc: clientConn, Seqno: seqno, ReplyCh: replyCh}
	m.connMap[rkey] = c
}

func (m *ServerCallbackMgr) Close() {
	//vv("SCM.Close() called")
	m.mut.Lock()
	defer m.mut.Unlock()
	for _, c := range m.connMap {
		//vv("closing c.Nc %T local='%v' remote='%v'", c.Nc, local(c.Nc), remote(c.Nc))
		quicConn, ok := c.Nc.(*rpc.NetConnWrapper)
		if ok {
			//vv("sending quicConn.CloseWithError server shutdown.")
			quicConn.Connection.CloseWithError(0, "server shutdown")
			//vv("back from sending quicConn.CloseWithError server shutdown.")
		} else {
			//vv("sending Nc.Close()") // seen.
			// Error accepting stream: INTERNAL_ERROR (local): write udp 100.86.202.68:2776->100.86.202.68:55685: use of closed network connection
			c.Nc.Close()
			//vv("done sending Nc.Close()") // seen.
		}
	}
}

func netConnRemoteAddrAsKey(nc net.Conn) string {
	ra := nc.RemoteAddr()
	return ra.Network() + "://" + ra.String()
}

func (m *ServerCallbackMgr) pushJobToClient(callID, addr string, j *Job) (key string, ok bool, err error) {

	subject := j.Msg.String()

	//vv("server: pushJobToClient: to addr:'%v' job='%v'; callID='%v'", addr, j.String(), callID)

	nex, err := m.get(addr)
	if err != nil {
		panic(fmt.Sprintf("could not find cache net.Conn for addr '%s'", addr))
		//return addr, false, err
	}
	jobSerz, err := m.cfg.jobToBytesWithStamp(j)
	if err != nil {
		return addr, false, err
	}
	//vv("jobSerz is %v bytes", len(jobSerz))
	return m.pushToClient(callID, subject, nex, jobSerz)
}

func (m *ServerCallbackMgr) pushToClient(callID, subject string, nex *Nexus, by []byte) (key string, ok bool, err error) {
	nc := nex.Nc
	key = netConnRemoteAddrAsKey(nc)

	//vv("pushToClient is doing m.Srv.SendMessage(); nex.Seqno=%v", nex.Seqno)
	errWriteDur := time.Second * 5
	err = m.Srv.SendMessage(callID, subject, key, by, nex.Seqno, &errWriteDur)
	//vv("err from m.Srv.SendMessage() was '%v'", err)
	if err == nil {
		ok = true
	} else {
		vv("failed to send messsage to %s: %v\n", key, err)
		nc.Close()
		m.removeClient(key)
	}

	return
}

func (m *ServerCallbackMgr) Ready1(args *rpc.Message) {
	_ = m.readyCommon(args)
}

func (m *ServerCallbackMgr) readyCommon(args *rpc.Message) *Job {

	//vv("ServerCallbackMgr: Ready1() top. args.MID='%v'", args.MID)
	clientConn := args.HDR.Nc

	var job *Job
	var err error
	// harden against cross-cluster communication, where bytesToJob errors out.
	defer func() {
		if recover() != nil {
			job = nil
			err = fmt.Errorf("unknown recovered error on receive")
		}
	}()

	// Read job submitted to the server
	job, err = m.cfg.bytesToJob(args.JobSerz)
	panicOn(err)
	//if err != nil {
	//	return fmt.Errorf("error in ServerCallbackMgr.Ready() after CapnpToJob: '%v'", err)
	//}
	job.nc = clientConn
	job.callid = args.HDR.CallID
	job.callSeqno = args.HDR.Seqno

	//vv("ServerCallbackMgr: Ready() sees incoming job: '%s'", job.String())

	// NAT may have changed the address we have to reply to
	remoteAfterNAT := remote(job.nc)
	if job.Submitaddr != "" && remoteAfterNAT != job.Submitaddr {
		//vv("updating job.Submitaddr from '%v' to '%v' to allow us to reply through NAT", job.Submitaddr, remoteAfterNAT)
		job.Submitaddr = remoteAfterNAT
	}
	if job.Msg == schema.JOBMSG_REQUESTFORWORK {
		if remoteAfterNAT != job.Workeraddr {
			//vv("updating job.Workeraddr from '%v' to '%v' to allow us to reply through NAT", job.Workeraddr, remoteAfterNAT)
			job.Workeraddr = remoteAfterNAT
		}
	}
	m.register(clientConn, args.HDR.Seqno, nil)

	select {
	case m.jserv.FromRpcServer <- job:
	case <-m.jserv.ListenerShutdown:
		//vv("we see jserv.ListenerShutdown")
	}

	return job
}

func (m *ServerCallbackMgr) Ready2(args, reply *rpc.Message) error {
	//vv("ServerCallbackMgr: Ready2() top. args.HDR='%v'", args.HDR)

	job := m.readyCommon(args)

	// wait for reply
	select { // hung here in server when "goq sub" client stalls
	case pReply := <-job.replyCh:
		//vv("server Ready2() got pReply: '%v'", pReply)
		reply.JobSerz = pReply.JobSerz
		reply.JobErrs = pReply.JobErrs
	case <-m.jserv.ListenerShutdown:
	}
	return nil
}

type localRemoteAddr interface {
	RemoteAddr() net.Addr
	LocalAddr() net.Addr
}

func remote(nc localRemoteAddr) string {
	ra := nc.RemoteAddr()
	return ra.Network() + "://" + ra.String()
}

func local(nc localRemoteAddr) string {
	la := nc.LocalAddr()
	return la.Network() + "://" + la.String()
}
