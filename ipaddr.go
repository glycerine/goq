package main

import (
	"fmt"
	"net"
	"regexp"
)

var validIPv4addr = regexp.MustCompile(`^[0-9]+[.][0-9]+[.][0-9]+[.][0-9]+$`)

func GetExternalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		panic(err)
	}

	valid := []string{}

	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			addr := ipnet.IP.String()
			match := validIPv4addr.FindStringSubmatch(addr)
			if match != nil {
				if addr != "127.0.0.1" {
					valid = append(valid, addr)
				}
			}
		}
	}
	switch len(valid) {
	case 0:
		return "127.0.0.1"
	default:
		return valid[0]
	}
}

func GetExternalIPAsInt() int {
	s := GetExternalIP()
	ip := net.ParseIP(s).To4()
	if ip == nil {
		return 0
	}
	sum := 0
	for i := 0; i < 4; i++ {
		mult := 1 << (8 * uint64(3-i))
		//fmt.Printf("mult = %d\n", mult)
		sum += int(mult) * int(ip[i])
		//fmt.Printf("sum = %d\n", sum)
	}
	//fmt.Printf("GetExternalIPAsInt() returns %d\n", sum)
	return sum
}

// sure there's a race here, but should be okay.
// :0 asks the OS to give us a free port.
func GetAvailPort() int {
	l, _ := net.Listen("tcp", ":0")
	r := l.Addr()
	l.Close()
	return r.(*net.TCPAddr).Port
}

func GenAddress() string {
	port := GetAvailPort()
	ip := GetExternalIP()
	s := fmt.Sprintf("tcp://%s:%d", ip, port)
	//fmt.Printf("GenAddress returning '%s'\n", s)
	return s
}

// reduce `tcp://blah:port` to `blah:port`
var validSplitOffProto = regexp.MustCompile(`^[^:]*://(.*)$`)

func StripNanomsgAddressPrefix(nanomsgAddr string) (suffix string, err error) {

	match := validSplitOffProto.FindStringSubmatch(nanomsgAddr)
	if match == nil || len(match) != 2 {
		return "", fmt.Errorf("could not strip prefix tcp:// from nanomsg address '%s'", nanomsgAddr)
	}
	return match[1], nil
}
