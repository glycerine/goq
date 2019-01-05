package xrpc

import (
	"context"
	"crypto/tls"
	"fmt"
	"math/rand"
	"net"
	"os"
	"sync"
	"time"

	rpcxServer "github.com/smallnest/rpcx/server"
)

type ServerCallbackMgr struct {
	RpcXsrv *rpcxServer.Server

	mut     sync.Mutex
	connMap map[string]int
	connSeq []net.Conn
	keys    []string
}

func (s *ServerCallbackMgr) register(clientConn net.Conn) {
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
}

func (m *ServerCallbackMgr) removeClient(key string) {
	m.mut.Lock()
	defer m.mut.Unlock()
	k, ok := m.connMap[key]
	if !ok {
		return
	}
	m.connSeq = append(m.connSeq[:k], m.connSeq[k+1:]...)
}

// addr is like localhost:8972
func NewRpcXServer(addr string) *ServerCallbackMgr {

	// load up certs for TLS
	gp := os.Getenv("GOPATH")
	base := fmt.Sprintf("%s/src/github.com/glycerine/goq/xrpc/", gp)

	sslCA := base + "certs/ca.crt"        // path to CA cert
	sslClientCA := base + "certs/ca.crt"  // path to CA cert to verify client certs, can be same as sslCA
	sslCert := base + "certs/node.crt"    // path to server cert
	sslCertKey := base + "certs/node.key" // path to server key
	conf, err := LoadServerTLSConfig(sslCA, sslClientCA, sslCert, sslCertKey)
	panicOn(err)

	conf.ClientAuth = tls.RequireAndVerifyClientCert
	//insecure to turn off client cert checking with: conf.ClientAuth = tls.NoClientCert

	s := rpcxServer.NewServer(rpcxServer.WithTLSConfig(conf))
	m := &ServerCallbackMgr{
		connMap: make(map[string]int),
		RpcXsrv: s,
	}
	s.RegisterName("ServerCallbackMgr", m, "")
	//s.Register(new(ServerCallbackMgr), "")
	go s.Serve("tcp", addr)

	// %s\n", clientConn.RemoteAddr().String())
	fmt.Printf("randomly send messages to clients\n")
	for {
		time.Sleep(100 * time.Millisecond)

		var k int
		var key string
		var conn net.Conn
		m.mut.Lock()
		n := len(m.connSeq)
		if n > 0 {
			k = rand.Intn(n)
			conn = m.connSeq[k]
			key = m.keys[k]
		}
		m.mut.Unlock()
		if n == 0 {
			continue
		}

		vv("server: sending to '%s'", key)
		err := s.SendMessage(conn, "test_service_path", "test_service_method", nil, []byte(fmt.Sprintf("abcde to client '%s',k=%v\n", key, k)))
		if err != nil {
			vv("failed to send messsage to %s: %v\n", key, err)
			//if strings.Contains(err.Error(), "use of closed connection")
			conn.Close()
			m.removeClient(key)

		}
	}
}

func serverToCli(s *rpcxServer.Server, clientConn net.Conn) {
	fmt.Printf("serverToCli: start to send messages to %s\n", clientConn.RemoteAddr().String())
	for i := 0; i < 5; i++ {
		err := s.SendMessage(clientConn, "test_service_path", "test_service_method", nil, []byte(fmt.Sprintf("abcde i=%v\n", i)))
		if err != nil {
			fmt.Printf("failed to send messsage to %s: %v\n", clientConn.RemoteAddr().String(), err)
			return
		}
	}
}
