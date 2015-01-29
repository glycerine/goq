package main

// #include <errno.h>
// void* addr_of_errno() {
//   return &errno;
// }
import "C"

func GetAddrErrno() uintptr {
	return uintptr(C.addr_of_errno())
}
