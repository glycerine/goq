package main

import (
	"fmt"
	"time"

	nn "github.com/glycerine/go-nanomsg"
)

//var addr string = "inproc://inproc_bench"
var addr string = "tcp://127.0.0.1:9988"

func main() {

	var err error
	var s *nn.Socket
	if s, err = nn.NewSocket(nn.AF_SP, nn.PAIR); err != nil {
		panic(err)
	}
	if _, err = s.Bind(addr); err != nil {
		panic(err)
	}
	fmt.Printf("serv bound '%s'\n", addr)

	t0 := time.Now()
	//N := 100
	for {
		data, err := s.Recv(0)
		fmt.Printf("serv received data: '%s'\n", data)
		if err != nil {
			panic(err)
		}

		// echo data back
		if _, err = s.Send(data, 0); err != nil {
			panic(err)
		}
	}

	if err = s.Close(); err != nil {
		panic(err)
	}
	fmt.Printf("elap: %v\n", time.Since(t0))

	select {}
}
