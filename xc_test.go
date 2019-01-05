package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	cv "github.com/glycerine/goconvey/convey"
	"github.com/glycerine/idem"
	rpcxClient "github.com/smallnest/rpcx/client"
	rpcxProtocol "github.com/smallnest/rpcx/protocol"
	rpcxServer "github.com/smallnest/rpcx/server"
)

var _ = fmt.Printf

var clientConn net.Conn
var connected = false

const UseTLS = false

type Mgr struct {
	srv *rpcxServer.Server

	mut     sync.Mutex
	connMap map[string]int
	connSeq []net.Conn
	keys    []string

	firstClientSeen *idem.IdemCloseChan
	Halt            *idem.Halter
}

func NewMgr(s *rpcxServer.Server) *Mgr {
	return &Mgr{
		srv:             s,
		connMap:         make(map[string]int),
		firstClientSeen: idem.NewIdemCloseChan(),
		Halt:            idem.NewHalter(),
	}
}

func (s *Mgr) register(clientConn net.Conn) {

	remote := clientConn.RemoteAddr()
	key := remote.Network() + "://" + remote.String()
	vv("registering client conn under key '%s'", key)
	s.mut.Lock()
	defer s.mut.Unlock()

	where, already := s.connMap[key]
	if already {
		// good: this happens, so the tcp connection is being reused.
		vv("we see returning registration for where=%v, '%s'=='%s'", where, s.keys[where], key)
		return // coming back, good.
	}
	s.connMap[key] = len(s.connSeq)
	s.connSeq = append(s.connSeq, clientConn)
	s.keys = append(s.keys, key)
	vv("jea debug: skipping firstClientSeen.Close()")
	//s.firstClientSeen.Close()
}

func (m *Mgr) MsgFromClient(ctx context.Context, args *Args, reply *Reply) error {
	vv("MsgFromClient() top")
	clientConn = ctx.Value(rpcxServer.RemoteConnContextKey).(net.Conn)
	m.register(clientConn)
	reply.C = args.A * args.B
	vv("server sees args.A=%v, args.B=%v, setting reply.C=%v", args.A, args.B, reply.C)

	fmt.Printf("send 2 more serer-initiated messages to clients\n")
	go m.serverToCli([]byte{byte((args.A + 1) % 256)}, clientConn)
	go m.serverToCli([]byte{byte((args.A + 2) % 256)}, clientConn)

	return nil
}

func (m *Mgr) removeClient(key string) {
	m.mut.Lock()
	defer m.mut.Unlock()
	k, ok := m.connMap[key]
	if !ok {
		return
	}
	delete(m.connMap, key)
	m.connSeq = append(m.connSeq[:k], m.connSeq[k+1:]...)
}

// for every one message this server gets, respond with 3 messages back:
// one from the initial call, and 2 more async.
// This lets us verify that the bidirectional send works.
func serverMain(serverAddr string) *idem.Halter {

	var opts []rpcxServer.OptionFn

	if UseTLS {
		gp := os.Getenv("GOPATH")
		base := fmt.Sprintf("%s/src/github.com/glycerine/goq/xrpc/", gp)

		sslCA := base + "certs/ca.crt"        // path to CA cert
		sslClientCA := base + "certs/ca.crt"  // path to CA cert to verify client certs, can be same as sslCA
		sslCert := base + "certs/node.crt"    // path to server cert
		sslCertKey := base + "certs/node.key" // path to server key

		conf, err := LoadServerTLSConfig(sslCA, sslClientCA, sslCert, sslCertKey)
		panicOn(err)
		conf.ServerName = "localhost"

		conf.ClientAuth = tls.RequireAndVerifyClientCert
		//insecure to turn off client cert checking with: conf.ClientAuth = tls.NoClientCert

		opts = append(opts, rpcxServer.WithTLSConfig(conf))
	}

	s := rpcxServer.NewServer(opts...)
	m := NewMgr(s)

	s.RegisterName("Mgr", m, "")
	//s.Register(new(Mgr), "")
	go s.Serve("tcp", serverAddr)

	// handle shutdown
	go func() {
		<-m.Halt.ReqStop.Chan
		vv("halt request detetected, shutting down server")
		s.Close()
		m.Halt.Done.Close()
	}()
	vv("serverMain() all done.")
	return m.Halt
}

func (m *Mgr) serverToCli(payload []byte, clientConn net.Conn) {
	fmt.Printf("serverToCli: start to send messages to %s\n", clientConn.RemoteAddr().String())
	meta := map[string]string{"hello": "world"}
	err := m.srv.SendMessage(clientConn, "test_service_path", "test_service_method", meta, payload)
	panicOn(err)
	if err != nil {
		fmt.Printf("failed to send messsage to %s: %v\n", clientConn.RemoteAddr().String(), err)
		return
	}
}

// client

func clientMain(serverAddr string) {

	for k := 0; k < 3; k++ {

		option := rpcxClient.DefaultOption
		option.ReadTimeout = time.Second
		option.WriteTimeout = time.Second

		if UseTLS {
			gp := os.Getenv("GOPATH")
			base := fmt.Sprintf("%s/src/github.com/glycerine/goq/xrpc/", gp)

			sslCA := base + "certs/ca.crt"               // path to CA cert
			sslCert := base + "certs/client.root.crt"    // path to client cert
			sslCertKey := base + "certs/client.root.key" // path to client key
			conf, err := LoadClientTLSConfig(sslCA, sslCert, sslCertKey)
			conf.ServerName = "localhost"
			panicOn(err)

			//conf.InsecureSkipVerify = false // true would be insecure
			option.TLSConfig = conf
		}

		// server push mechanism: how we receive them.
		fromSrvCh := make(chan *rpcxProtocol.Message)

		//	d := rpcxClient.NewPeer2PeerDiscovery("tcp@"+serverAddr, "")
		//	xclient := rpcxClient.NewBidirectionalXClient("Mgr", rpcxClient.Failtry, rpcxClient.RandomSelect, d, option, fromSrvCh)

		c := rpcxClient.NewClient(option)
		err := c.Connect("tcp", serverAddr)
		panicOn(err)
		c.RegisterServerMessageChan(fromSrvCh)

		defer c.Close()

		docall := func(xc *rpcxClient.Client, payload byte) Reply {
			args := &Args{
				A: int64(payload),
				B: 10,
			}
			var mutArgs sync.Mutex
			_ = mutArgs
			reply := &Reply{}
			// Call() is synchronous: it waits for a reply. Go() is the async option.
			err := xc.Call(context.Background(), "Mgr", "MsgFromClient", args, reply)
			if err != nil {
				log.Fatalf("failed to call: %v", err)
			}

			log.Printf("%d * %d = %d", args.A, args.B, reply.C)

			// verify that roundtrip happened; that server did the multiply
			// and we got it back
			if reply.C != args.A*args.B {
				panic("test failure: roundtrip to server did not happen: C != A*B")
			}
			return *reply
		}

		// collect all the side-band server-pushed-to-client messages
		// that arrive on fromSrvCh here.
		alsoGot := make(map[byte]int)

		r := docall(c, 1)
		vv("client sees reply r.C = %v", r.C)
		cv.So(r.C, cv.ShouldEqual, 10)

		// server sends 2 additional pushes after each call, with A+1 and A+2 as the payload.
		msg := <-fromSrvCh
		vv("receive msg from server: %v, on ServicePath='%s', ServiceMethod='%s', msg.Metadata='%#v'", int(msg.Payload[0]), msg.ServicePath, msg.ServiceMethod, msg.Metadata)
		alsoGot[msg.Payload[0]]++

		msg = <-fromSrvCh
		vv("receive msg from server: %v, on ServicePath='%s', ServiceMethod='%s', msg.Metadata='%#v'", int(msg.Payload[0]), msg.ServicePath, msg.ServiceMethod, msg.Metadata)
		alsoGot[msg.Payload[0]]++

		r = docall(c, 4)
		vv("client sees reply r.C = %v", r.C)
		cv.So(r.C, cv.ShouldEqual, 40)

		msg = <-fromSrvCh
		vv("receive msg from server: %v, on ServicePath='%s', ServiceMethod='%s', msg.Metadata='%#v'", int(msg.Payload[0]), msg.ServicePath, msg.ServiceMethod, msg.Metadata)
		alsoGot[msg.Payload[0]]++

		msg = <-fromSrvCh
		vv("receive msg from server: %v, on ServicePath='%s', ServiceMethod='%s', msg.Metadata='%#v'", int(msg.Payload[0]), msg.ServicePath, msg.ServiceMethod, msg.Metadata)
		alsoGot[msg.Payload[0]]++

		r = docall(c, 7)
		vv("client sees reply r.C = %v", r.C)
		cv.So(r.C, cv.ShouldEqual, 70)

		msg = <-fromSrvCh
		vv("receive msg from server: %v, on ServicePath='%s', ServiceMethod='%s', msg.Metadata='%#v'", int(msg.Payload[0]), msg.ServicePath, msg.ServiceMethod, msg.Metadata)
		alsoGot[msg.Payload[0]]++

		msg = <-fromSrvCh
		vv("receive msg from server: %v, on ServicePath='%s', ServiceMethod='%s', msg.Metadata='%#v'", int(msg.Payload[0]), msg.ServicePath, msg.ServiceMethod, msg.Metadata)
		alsoGot[msg.Payload[0]]++

		// got  all 6 server pushes?
		cv.So(alsoGot[2], cv.ShouldEqual, 1)
		cv.So(alsoGot[3], cv.ShouldEqual, 1)
		cv.So(alsoGot[5], cv.ShouldEqual, 1)
		cv.So(alsoGot[6], cv.ShouldEqual, 1)
		cv.So(alsoGot[8], cv.ShouldEqual, 1)
		cv.So(alsoGot[9], cv.ShouldEqual, 1)
		cv.So(len(alsoGot), cv.ShouldEqual, 6)

		// Interleaving of receives test:
		// we should be able to make a 2nd outbound call, before
		// receiving a waiting inbound on the side-channel.

		alsoGot = make(map[byte]int)

		r = docall(c, 10)
		cv.So(r.C, cv.ShouldEqual, 100)

		msg = <-fromSrvCh
		vv("receive msg from server: %v, on ServicePath='%s', ServiceMethod='%s', msg.Metadata='%#v'", int(msg.Payload[0]), msg.ServicePath, msg.ServiceMethod, msg.Metadata)
		alsoGot[msg.Payload[0]]++

		r = docall(c, 13)
		cv.So(r.C, cv.ShouldEqual, 130)

		msg = <-fromSrvCh
		vv("receive msg from server: %v, on ServicePath='%s', ServiceMethod='%s', msg.Metadata='%#v'", int(msg.Payload[0]), msg.ServicePath, msg.ServiceMethod, msg.Metadata)
		alsoGot[msg.Payload[0]]++

		r = docall(c, 16)
		cv.So(r.C, cv.ShouldEqual, 160)
		r = docall(c, 19)
		cv.So(r.C, cv.ShouldEqual, 190)

		after1 := time.After(time.Second)
	testloop:
		for {
			select {
			case msg := <-fromSrvCh:
				vv("receive msg from server: %v, on ServicePath='%s', ServiceMethod='%s', msg.Metadata='%#v'", int(msg.Payload[0]), msg.ServicePath, msg.ServiceMethod, msg.Metadata)
				alsoGot[msg.Payload[0]]++
			case <-after1:
				fmt.Printf("wrapping up async server to client receive loop\n")
				break testloop
			}
		} // end testloop.

		// got  all 6 server pushes?
		cv.So(alsoGot[11], cv.ShouldEqual, 1)
		cv.So(alsoGot[12], cv.ShouldEqual, 1)
		cv.So(alsoGot[14], cv.ShouldEqual, 1)
		cv.So(alsoGot[15], cv.ShouldEqual, 1)
		cv.So(alsoGot[17], cv.ShouldEqual, 1)
		cv.So(alsoGot[18], cv.ShouldEqual, 1)
		cv.So(alsoGot[20], cv.ShouldEqual, 1)
		cv.So(alsoGot[21], cv.ShouldEqual, 1)
		cv.So(len(alsoGot), cv.ShouldEqual, 8)

		c.Close()
	}

}

func TestBasicRoundTripWithRPCXClientWorks(t *testing.T) {

	cv.Convey("Round trip using rpcxClient and rcpxServer should work", t, func() {
		serverAddr := "127.0.0.1:2777"
		mgrHalt := serverMain(serverAddr)
		time.Sleep(time.Second) // give server a moment to get ready.
		clientMain(serverAddr)
		mgrHalt.RequestStop()
		<-mgrHalt.Done.Chan
	})
}

/*
func TestBasicRoundTripWithRPCXClientWorks(t *testing.T) {

	cv.Convey("Round trip using rpcxClient and rcpxServer should work", t, func() {

		//pid := os.Getpid()
		var err error
		remote := false

		// *** universal test cfg setup
		skipbye := false
		cfg := NewTestConfig()
		//cfg.SendTimeoutMsec = 5000
		defer cfg.ByeTestConfig(&skipbye)
		// *** end universal test setup

		cfg.DebugMode = true // reply to badsig packets

		jobserv, err = NewJobServ(cfg)
		defer CleanupOutdir(cfg)

		// a job that will go forever unless cancelled
		j := NewJob()
		j.Cmd = "bin/forev.sh"

		infWait := false
		sub, err := NewSubmitter(GenAddress(), cfg, infWait)
		if err != nil {
			panic(err)
		}

		cy, errsend := sendZjobToServer(sub.ServerPushSock, j, &sub.Cfg, nil)
		if errsend != nil {
			panic(errsend)
		}

		vv("done with sendZjobToServer, sub is now calling recvZjob...")
		reply, err := recvZjob(sub.Cli, &sub.Cfg)
		panicOn(err)
		vv("reply='%#v'", reply)

	})
}
*/
