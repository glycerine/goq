package main

import (
	"fmt"
	"time"

	nn "github.com/glycerine/go-nanomsg"
)

//var addr string = "inproc://inproc_bench"
var addr string = "tcp://127.0.0.1:9988"

func main() {

	var s2 *nn.Socket
	var err error
	if s2, err = nn.NewSocket(nn.AF_SP, nn.PAIR); err != nil {
		panic(err)
	}
	if _, err = s2.Connect(addr); err != nil {
		panic(err)
	}

	buf := []byte("hello jason")
	t0 := time.Now()
	N := 10
	for i := 0; i < N; i++ {
		if _, err = s2.Send(buf, 0); err != nil {
			panic(err)
		}
		if _, err := s2.Recv(0); err != nil {
			panic(err)
		}
	}

	if err = s2.Close(); err != nil {
		panic(err)
	}
	fmt.Printf("cli done. elap: %v\n", time.Since(t0))
}
