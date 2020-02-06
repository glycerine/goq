package xrpc

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	rpcxClient "github.com/smallnest/rpcx/client"
	rpcxProtocol "github.com/smallnest/rpcx/protocol"
)

var (
	addr = flag.String("addr", "localhost:8972", "server address")
)

func XclientMain() {
	flag.Parse()

	option := rpcxClient.DefaultOption

	fake := ""
	//fake := "alt/" // used to check that server would reject the wrong certs.

	gp := os.Getenv("GOPATH")
	base := fmt.Sprintf("%s/src/github.com/glycerine/goq/xrpc/", gp)

	sslCA := base + "certs/ca.crt"                      // path to CA cert
	sslCert := base + fake + "certs/client.root.crt"    // path to client cert
	sslCertKey := base + fake + "certs/client.root.key" // path to client key
	conf, err := LoadClientTLSConfig(sslCA, sslCert, sslCertKey)
	panicOn(err)

	conf.InsecureSkipVerify = true // true would be insecure
	option.TLSConfig = conf

	// server push mechanism: how we receive them.
	ch := make(chan *rpcXProtocol.Message)

	d := rpcxClient.NewPeer2PeerDiscovery("tcp@"+*addr, "")
	xclient := rpcxClient.NewBidirectionalXClient("ServerCallbackMgr", rpcxClient.Failtry, rpcxClient.RandomSelect, d, option, ch)
	defer xclient.Close()

	args := &Args{
		A: 10,
		B: 20,
	}

	reply := &Reply{}
	err = xclient.Call(context.Background(), "WorkerReady", args, reply)
	if err != nil {
		log.Fatalf("failed to call: %v", err)
	}

	log.Printf("%d * %d = %d", args.A, args.B, reply.C)

	after1 := time.After(time.Second)
	after10 := time.After(time.Second * 10)
	for {
		select {
		case msg := <-ch:
			vv("receive msg from server: %s", msg.Payload)
		case <-after1:
			vv("xclient calling WorkerReady again!")
			args.A++
			err = xclient.Call(context.Background(), "WorkerReady", args, reply)
			if err != nil {
				vv("2nd ready call failed: %v", err)
			}
			vv("%d * %d = %d", args.A, args.B, reply.C)

		case <-after10:
			fmt.Printf("wrapping up after 10 sec. thanks!!\n")
			return
		}

	}
}
