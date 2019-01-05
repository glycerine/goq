package main

/* todo
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

func serverMainSSH(serverAddr string) *idem.Halter {

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

// client

func clientMainSSH(serverAddr string) {

	option := rpcxClient.DefaultOption

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

	d := rpcxClient.NewPeer2PeerDiscovery("tcp@"+serverAddr, "")
	xclient := rpcxClient.NewBidirectionalXClient("Mgr", rpcxClient.Failtry, rpcxClient.RandomSelect, d, option, fromSrvCh)
	defer xclient.Close()

	docall := func(payload byte) Reply {
		args := &Args{
			A: int64(payload),
			B: 10,
		}
		var mutArgs sync.Mutex
		_ = mutArgs
		reply := &Reply{}
		err := xclient.Call(context.Background(), "MsgFromClient", args, reply)
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

	r := docall(1)
	vv("client sees reply r.C = %v", r.C)
	cv.So(r.C, cv.ShouldEqual, 10)

	// server sends 2 additional pushes after each call, with A+1 and A+2 as the payload.
	msg := <-fromSrvCh
	vv("receive msg from server: %v, on ServicePath='%s', ServiceMethod='%s', msg.Metadata='%#v'", int(msg.Payload[0]), msg.ServicePath, msg.ServiceMethod, msg.Metadata)
	alsoGot[msg.Payload[0]]++

	msg = <-fromSrvCh
	vv("receive msg from server: %v, on ServicePath='%s', ServiceMethod='%s', msg.Metadata='%#v'", int(msg.Payload[0]), msg.ServicePath, msg.ServiceMethod, msg.Metadata)
	alsoGot[msg.Payload[0]]++

	r = docall(4)
	vv("client sees reply r.C = %v", r.C)
	cv.So(r.C, cv.ShouldEqual, 40)

	msg = <-fromSrvCh
	vv("receive msg from server: %v, on ServicePath='%s', ServiceMethod='%s', msg.Metadata='%#v'", int(msg.Payload[0]), msg.ServicePath, msg.ServiceMethod, msg.Metadata)
	alsoGot[msg.Payload[0]]++

	msg = <-fromSrvCh
	vv("receive msg from server: %v, on ServicePath='%s', ServiceMethod='%s', msg.Metadata='%#v'", int(msg.Payload[0]), msg.ServicePath, msg.ServiceMethod, msg.Metadata)
	alsoGot[msg.Payload[0]]++

	r = docall(7)
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

	r = docall(10)
	cv.So(r.C, cv.ShouldEqual, 100)

	msg = <-fromSrvCh
	vv("receive msg from server: %v, on ServicePath='%s', ServiceMethod='%s', msg.Metadata='%#v'", int(msg.Payload[0]), msg.ServicePath, msg.ServiceMethod, msg.Metadata)
	alsoGot[msg.Payload[0]]++

	r = docall(13)
	cv.So(r.C, cv.ShouldEqual, 130)

	msg = <-fromSrvCh
	vv("receive msg from server: %v, on ServicePath='%s', ServiceMethod='%s', msg.Metadata='%#v'", int(msg.Payload[0]), msg.ServicePath, msg.ServiceMethod, msg.Metadata)
	alsoGot[msg.Payload[0]]++

	r = docall(16)
	cv.So(r.C, cv.ShouldEqual, 160)
	r = docall(19)
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
}

func TestBasicSSHWrappedRoundTripWithRPCXClientWorks(t *testing.T) {

	cv.Convey("Round trip using rpcxClient and rcpxServer should work", t, func() {
		serverAddr := "127.0.0.1:2777"
		mgrHalt := serverMainSSH(serverAddr)
		time.Sleep(time.Second) // give server a moment to get ready.
		clientMainSSH(serverAddr)
		mgrHalt.RequestStop()
		<-mgrHalt.Done.Chan
	})
}
*/
