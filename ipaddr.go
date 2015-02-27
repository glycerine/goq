package main

import (
	"fmt"
	"net"
	"regexp"
)

var validIPv4addr = regexp.MustCompile(`^[0-9]+[.][0-9]+[.][0-9]+[.][0-9]+$`)

var privateIPv4addr = regexp.MustCompile(`(^127\.0\.0\.1)|(^10\.)|(^172\.1[6-9]\.)|(^172\.2[0-9]\.)|(^172\.3[0-1]\.)|(^192\.168\.)`)

// IsRoutableIPv4 returns true if the string in ip represents an IPv4 address that is not
// private. See http://en.wikipedia.org/wiki/Private_network#Private_IPv4_address_spaces
// for the numeric ranges that are private. 127.0.0.1, 192.168.0.1, and 172.16.0.1 are
// examples of non-routables IP addresses.
func IsRoutableIPv4(ip string) bool {
	match := privateIPv4addr.FindStringSubmatch(ip)
	if match != nil {
		return false
	}
	return true
}

// GetExternalIP tries to determine the external IP address
// used on this host.
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
	case 1:
		return valid[0]
	default:
		// try to get a routable ip if possible.
		for _, ip := range valid {
			if IsRoutableIPv4(ip) {
				return ip
			}
		}
		// give up, just return the first.
		return valid[0]
	}
}

// GetExternalIPAsInt calls GetExternalIP() and then converts
// the resulting IPv4 string into an integer.
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

// GetAvailPort asks the OS for an unused port.
// There's a race here, where the port could be grabbed by someone else
// before the caller gets to Listen on it, but in practice such races
// are rare. Uses net.Listen("tcp", ":0") to determine a free port, then
// releases it back to the OS with Listener.Close().
func GetAvailPort() int {
	l, _ := net.Listen("tcp", ":0")
	r := l.Addr()
	l.Close()
	return r.(*net.TCPAddr).Port
}

// GenAddress generates a local address by calling GetAvailPort() and
// GetExternalIP(), then prefixing them with 'tcp://'.
func GenAddress() string {
	port := GetAvailPort()
	ip := GetExternalIP()
	s := fmt.Sprintf("tcp://%s:%d", ip, port)
	//fmt.Printf("GenAddress returning '%s'\n", s)
	return s
}

// reduce `tcp://blah:port` to `blah:port`
var validSplitOffProto = regexp.MustCompile(`^[^:]*://(.*)$`)

// StripNanomsgAddressPrefix removes the 'tcp://' prefix from
// nanomsgAddr.
func StripNanomsgAddressPrefix(nanomsgAddr string) (suffix string, err error) {

	match := validSplitOffProto.FindStringSubmatch(nanomsgAddr)
	if match == nil || len(match) != 2 {
		return "", fmt.Errorf("could not strip prefix tcp:// from nanomsg address '%s'", nanomsgAddr)
	}
	return match[1], nil
}
