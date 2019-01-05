package xrpc

type Args struct {
	A int
	B int
}

type Reply struct {
	C   int
	Err error
}
