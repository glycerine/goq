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
	Odir            string // GOQ_ODIR
	JservPort       int    // GOQ_JSERV_PORT
	JservAddr       string //  made from JservIP and JservPort
	ClusterId       string // GOQ_CLUSTERID
	NoSshConfig     bool   // GOQ_NOSSHCONFIG
	DebugMode       bool   // GOQ_DEBUGMODE
}

func CopyConfig(cfg *Config) *Config {
	cp := *cfg
	return &cp
}

//
// DiskThenEnvConfig: the usual if you want to specify home, else use DefaultCfg()
//
func DiskThenEnvConfig(home string) (cfg *Config, err error) {
	// let the disk override what we find in the env, so read the env first.
	cfg = GetEnvConfig(IdFromEnvIfPossible)
	cfg, err = GetClusterIdAndPortFromFileIfSingleFile(home, cfg)
	return cfg, err
}

func ErrorCheckedPwd() string {
	pwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return pwd
}

// DefaultCfg
//  convenience wrapper, most server creation calls should use this.
//
func DefaultCfg() *Config {
	pwd := ErrorCheckedPwd()
	cfg, err := DiskThenEnvConfig(pwd)
	if err != nil {
		// ignore errors from DiskThenEnv -- just couldn't find any extant .goqclusterid files.
		//fmt.Printf("cfg = %#v\n", cfg)
	}

	return cfg
}

var regexSplitEnv = regexp.MustCompile(`^([^=]*)[=](.*)$`)

func (cfg *Config) Setenv(env []string) []string {

	e := EnvToMap(env)

	e["GOQ_SENDTIMEOUT_MSEC"] = fmt.Sprintf("%d", cfg.SendTimeoutMsec)
	e["GOQ_JSERV_IP"] = cfg.JservIP
	e["GOQ_ODIR"] = cfg.Odir
	e["GOQ_JSERV_PORT"] = fmt.Sprintf("%d", cfg.JservPort)
	e["GOQ_CLUSTERID"] = cfg.ClusterId
	if cfg.NoSshConfig {
		e["GOQ_NOSSHCONFIG"] = "true"
	} else {
		e["GOQ_NOSSHCONFIG"] = "false"
	}
	if cfg.DebugMode {
		e["GOQ_DEBUGMODE"] = "true"
	} else {
		e["GOQ_DEBUGMODE"] = "false"
	}

	return MapToEnv(e)
}

func EnvToMap(env []string) map[string]string {
	m := make(map[string]string)

	for _, v := range env {
		match := regexSplitEnv.FindStringSubmatch(v)
		if match != nil {
			//fmt.Printf("match = %#v\n", match)
			if len(match) != 3 {
				panic("regexSplitEnv must return two groups")
			}
			m[match[1]] = match[2]
		}
	}

	return m
}

func MapToEnv(m map[string]string) []string {
	env := make([]string, len(m))
	i := 0
	for k, v := range m {
		env[i] = fmt.Sprintf("%s=%s", k, v)
		i++
	}
	return env
}

type getEnvConfigT int

const (
	IdFromEnvIfPossible getEnvConfigT = iota
	RandId
)

func GetEnvConfig(ty getEnvConfigT) *Config {
	c := &Config{}
	c.SendTimeoutMsec = GetEnvNumber("GOQ_SENDTIMEOUT_MSEC", 1000)

	myip := GetExternalIP()
	c.JservIP = GetEnvString("GOQ_JSERV_IP", myip)
	c.Odir = GetEnvString("GOQ_ODIR", "o")
	c.JservPort = GetEnvNumber("GOQ_JSERV_PORT", 1776)
	c.JservAddr = fmt.Sprintf("tcp://%s:%d", c.JservIP, c.JservPort)

	if ty == RandId {

		cid := os.Getenv("GOQ_CLUSTERID")
		randomCid := GetRandomCidDistinctFrom(cid)
		c.ClusterId = randomCid

	} else {
		cid := os.Getenv("GOQ_CLUSTERID")
		if cid != "" {
			Vprintf("\n[pid %d] using clusterid from env var GOQ_CLUSTERID: '%s'\n", os.Getpid(), cid)
			c.ClusterId = cid
		} else {
			c.ClusterId = GetEnvString("GOQ_CLUSTERID", RandomClusterId())
		}
	}

	c.NoSshConfig = GetEnvBool("GOQ_NOSSHCONFIG", false)
	c.DebugMode = GetEnvBool("GOQ_DEBUGMODE", false)

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

var validClusterIdFile = regexp.MustCompile(`^[.]goqclusterid[.]port([0-9]+)$`)

func LocalClusterIdFile(cfg *Config) string {
	return fmt.Sprintf(".goqclusterid.port%d", cfg.JservPort)
}

func GetClusterIdPath(home string, cfg *Config) string {
	if home == "" {
		home := os.Getenv("HOME")
		if home == "" {
			panic("HOME env var must be set if home param not supplied")
		}
	}
	return fmt.Sprintf("%s/%s", home, LocalClusterIdFile(cfg))
}

func LoadLocalClusterId(home string, cfg *Config) string {
	fn := GetClusterIdPath(home, cfg)
	cid, err := ReadAndTrimFile(fn)
	if err != nil {
		panic(err)
	}
	return cid
}

func ReadAndTrimFile(fn string) (string, error) {
	by, err := ioutil.ReadFile(fn)
	if err != nil {
		return "", err
	}
	return strings.Trim(string(by), " \t\n"), nil
}

func SaveLocalClusterId(id string, home string, cfg *Config) {
	fn := GetClusterIdPath(home, cfg)
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

func RemoveLocalClusterId(home string, cfg *Config) error {
	fn := GetClusterIdPath(home, cfg)
	return os.Remove(fn)
}

func GetClusterIdFromFile(home string, cfg *Config) *Config {
	cfg2 := *cfg
	filecid := LoadLocalClusterId(home, &cfg2)
	if filecid != "" {
		cfg2.ClusterId = filecid
	}
	return &cfg2
}

func InjectHelper(key, val string) {
	var err error
	err = os.Setenv(key, val)
	if err != nil {
		panic(err)
	}
}

// panics on error
func InjectConfigIntoEnv(cfg *Config) {

	InjectHelper(`GOQ_SENDTIMEOUT_MSEC`, fmt.Sprintf("%d", cfg.SendTimeoutMsec))
	InjectHelper(`GOQ_JSERV_IP`, cfg.JservIP)
	InjectHelper(`GOQ_ODIR`, cfg.Odir)
	InjectHelper(`GOQ_JSERV_PORT`, fmt.Sprintf("%d", cfg.JservPort))
	InjectHelper(`GOQ_CLUSTERID`, cfg.ClusterId)
	InjectHelper(`GOQ_NOSSHCONFIG`, BoolToString(cfg.NoSshConfig))
	InjectHelper(`GOQ_DEBUGMODE`, BoolToString(cfg.DebugMode))
}

func (cfg *Config) InjectConfigIntoMap(addto *map[string]string) {

	MapInjectHelper(addto, `GOQ_SENDTIMEOUT_MSEC`, fmt.Sprintf("%d", cfg.SendTimeoutMsec))
	MapInjectHelper(addto, `GOQ_JSERV_IP`, cfg.JservIP)
	MapInjectHelper(addto, `GOQ_ODIR`, cfg.Odir)
	MapInjectHelper(addto, `GOQ_JSERV_PORT`, fmt.Sprintf("%d", cfg.JservPort))
	MapInjectHelper(addto, `GOQ_CLUSTERID`, cfg.ClusterId)
	MapInjectHelper(addto, `GOQ_NOSSHCONFIG`, BoolToString(cfg.NoSshConfig))
	MapInjectHelper(addto, `GOQ_DEBUGMODE`, BoolToString(cfg.DebugMode))
}

func MapInjectHelper(m *map[string]string, key, val string) {
	(*m)[key] = val
}

func BoolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func GetClusterIdAndPortFromFileIfSingleFile(home string, cfg *Config) (*Config, error) {

	fn, port := GetClusterIdFileNameFromHomeDir(home)
	if fn == "" {
		return cfg, fmt.Errorf("home dir '%s' had no .goqclusterid files in it", home)
	}
	cid, err := ReadAndTrimFile(fn)
	if err != nil {
		return cfg, fmt.Errorf("home dir '%s', could not read .goqclusterid file, error: %s", home, err)
	}

	cfg.JservPort = port
	cfg.ClusterId = cid
	return cfg, nil
}

func GetClusterIdFileNameFromHomeDir(home string) (clusteridFilename string, port int) {
	var err error
	dir, err := os.Open(home)
	if err != nil {
		panic(err)
	}
	fn, err := dir.Readdirnames(-1)
	if err != nil {
		panic(err)
	}

	alreadyFound := false

	for i := range fn {
		match := validClusterIdFile.FindStringSubmatch(fn[i])
		if match != nil {
			if alreadyFound {
				fmt.Fprintf(os.Stderr, "[pid %d] error: more than one .goqclusterid file present, aborting.\n", os.Getpid())
				os.Exit(1)
			}

			if len(match) != 2 {
				panic(fmt.Sprintf("[pid %d] regex problem with validClusterIdFile: must give match len of 2, instead match was len %d", os.Getpid(), len(match)))
			}
			port, err = strconv.Atoi(match[1])
			if err != nil {
				// should never get here now that the regex checks for numbers
				panic(fmt.Sprintf("[pid %d] error: could not parse port number in .goqclusterid.port file named '%s': %s", os.Getpid(), fn[i], err))
			}
			clusteridFilename = fn[i]
			alreadyFound = true
		}
	}

	return clusteridFilename, port
}

func GetRandomCidDistinctFrom(avoidcid string) string {
	randomCid := RandomClusterId()

	if avoidcid != "" {
		// don't collide with the avoidcid (e.g. from the env, even by chance)
		for {
			if avoidcid == randomCid {
				randomCid = RandomClusterId()
			} else {
				break
			}
		}
	}
	return randomCid
}

var regexStartsWithCOG = regexp.MustCompile(`GOQ_`)

func GetNonGOQEnv(env []string, omitid string) []string {
	res := make([]string, 0)

	var filterid = regexp.MustCompile(omitid)

	for i := range env {
		match := regexStartsWithCOG.FindStringSubmatch(env[i])
		if match == nil {
			m2 := filterid.FindStringSubmatch(env[i])
			if m2 == nil {
				res = append(res, env[i])
			}
		}
	}
	return res
}
