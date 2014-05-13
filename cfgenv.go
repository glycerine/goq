package main

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// grab config from env

type Config struct {
	SendTimeoutMsec int    // GOQ_SENDTIMEOUT_MSEC
	JservIP         string // GOQ_JSERV_IP
	JservPort       int    // GOQ_JSERV_PORT
	JservAddr       string //  made from JservIP and JservPort
	ClusterId       string // GOQ_CLUSTERID
	NoSshConfig     bool   // GOQ_NOSSHCONFIG
}

func GetEnvConfig() *Config {
	c := &Config{}
	c.SendTimeoutMsec = GetEnvNumber("GOQ_SENDTIMEOUT_MSEC", 30000)

	myip := GetExternalIP()
	c.JservIP = GetEnvString("GOQ_JSERV_IP", myip)
	c.JservPort = GetEnvNumber("GOQ_JSERV_PORT", 1776)
	c.JservAddr = fmt.Sprintf("tcp://%s:%d", c.JservIP, c.JservPort)
	c.ClusterId = GetEnvString("GOQ_CLUSTERID", RandomClusterId())
	c.NoSshConfig = GetEnvBool("GOQ_NOSSHCONFIG", false)

	if myip != c.JservIP {
		//
	}

	//fmt.Printf("GetEnvConfig returning %#v\n", c)

	return c
}

func GetEnvNumber(envvar string, def int) int {
	to := os.Getenv(envvar)
	if to != "" {
		toi, err := strconv.Atoi(to)
		if err == nil {
			return toi
		}
	}
	return def
}

func GetEnvString(envvar string, def string) string {
	s := os.Getenv(envvar)
	if s != "" {
		return s
	}
	return def
}

func GetEnvBool(envvar string, def bool) bool {
	s := os.Getenv(envvar)
	if s == "" {
		return def
	}
	if s == "true" || s == "TRUE" || s == "True" {
		return true
	}
	return false
}

const IdSz = 40

func RandomClusterId() string {
	rand.Seed(time.Now().UnixNano())
	alphabet := "0123456789abcdef"

	buf := make([]byte, IdSz)
	for i := 0; i < IdSz; i++ {
		buf[i] = alphabet[rand.Intn(len(alphabet)-1)]
	}
	return Sha1sum(string(buf))
}

var validClusterId = regexp.MustCompile(`^[0-9a-f]{40}`)

func ShellOutForClusterId() string {
	out, err := exec.Command(GoqExeName, "clusterid").Output()
	if err != nil {
		panic(err)
	}

	return strings.Trim(string(out), " \t\n")
}

func IsValidClusterId(id string) bool {
	if len(id) != IdSz {
		return false
	}
	match := validClusterId.FindStringSubmatch(id)
	if match != nil {
		return true
	}
	return false
}

// ssh into server and get clusterid
func SshFetchClusterId(server string, home string, port string) string {
	return ""
}

func LocalClusterIdFile() string {
	return fmt.Sprintf(".goqclusterid.port%d", os.Getpid(), Cfg.JservPort)
}

func GetClusterIdPath(home string) string {
	if home == "" {
		home := os.Getenv("HOME")
		if home == "" {
			panic("HOME env var must be set if home param not supplied")
		}
	}
	return home + "/" + LocalClusterIdFile()
}

func LoadLocalClusterId(home string) string {
	fn := GetClusterIdPath(home)
	by, err := ioutil.ReadFile(fn)
	if err != nil {
		panic(err)
	}
	return strings.Trim(string(by), " \t\n")
}

func SaveLocalClusterId(id string, home string) {
	fn := GetClusterIdPath(home)
	// keep private, 0600
	f, err := os.OpenFile(fn, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	n, err := f.Write([]byte(id))
	if err != nil {
		panic(err)
	}
	if n != len(id) {
		panic(fmt.Sprintf("write truncated atempting to write clusterid to file '%s'", fn))
	}
}

func RemoveLocalClusterId(home string) error {
	fn := GetClusterIdPath(home)
	return os.Remove(fn)
}
