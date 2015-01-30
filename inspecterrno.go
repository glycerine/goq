package main

// #include <errno.h>
// void* addr_of_errno() {
//   return &errno;
// }
// int errno_itself() {
//   return errno;
// }
import "C"

func GetAddrErrno() uintptr {
	return uintptr(C.addr_of_errno())
}

func GetErrno() int {
	return int(C.errno_itself())
}
