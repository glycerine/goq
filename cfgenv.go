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
)

// grab config from env

type TmSeconds int64 // time in seconds since epoch
type Ntm int64       // time in nanoseconds since epoch

func Tmsec2Ntm(t TmSeconds) Ntm {
	return Ntm(t) * 1e9
}
func MaxNtm(a, b Ntm) Ntm {
	if a >= b {
		return a
	}
	return b
}

// data flow:
//
// GOQ_HOME -> $GOQ_HOME/.goq/{clusterid, aes key, stored-disk-config}
//
type Config struct {
	SendTimeoutMsec int        // GOQ_SENDTIMEOUT_MSEC
	RecvTimeoutMsec int        // GOQ_RECVTIMEOUT_MSEC
	JservIP         string     // GOQ_JSERV_IP
	Home            string     // GOQ_HOME
	Odir            string     // GOQ_ODIR
	JservPort       int        // GOQ_JSERV_PORT
	ClusterId       string     // from GOQ_HOME/.goq/goqclusterid
	NoSshConfig     bool       // GOQ_NOSSHCONFIG
	DebugMode       bool       // GOQ_DEBUGMODE
	Cypher          *CypherKey // from GOQ_HOME/.goq/aes

	// for TestConfig; see NewTestConfig()
	origdir  string
	tempdir  string
	orighome string

	Heartbeat TmSeconds
}

func NewConfig() *Config {
	return &Config{}
}

func CopyConfig(cfg *Config) *Config {
	cp := *cfg
	return &cp
}

//
// DiskThenEnvConfig: the usual if you want to specify home, else use DefaultCfg()
//
func DiskThenEnvConfig(home string) (cfg *Config, err error) {
	// let the disk override env

	fallback := GetEnvConfig()
	cfg, _ = GetConfigFromFile(home, fallback) // ignore the error; might not be able to read cid if it isn't there yet.

	key, err := LoadKey(cfg)
	if err != nil {
		err = fmt.Errorf("problem with LoadKey(cfg): %s", err)
	}
	cfg.Cypher = key

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

func (cfg *Config) JservAddr() string {
	return fmt.Sprintf("tcp://%s:%d", cfg.JservIP, cfg.JservPort)
}

func (cfg *Config) Setenv(env []string) []string {

	e := EnvToMap(env)

	e["GOQ_SENDTIMEOUT_MSEC"] = fmt.Sprintf("%d", cfg.SendTimeoutMsec)
	e["GOQ_RECVTIMEOUT_MSEC"] = fmt.Sprintf("%d", cfg.RecvTimeoutMsec)
	e["GOQ_JSERV_IP"] = cfg.JservIP
	e["GOQ_HOME"] = cfg.Home
	e["GOQ_ODIR"] = cfg.Odir
	e["GOQ_JSERV_PORT"] = fmt.Sprintf("%d", cfg.JservPort)
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
	e["GOQ_HEARTBEAT_SEC"] = fmt.Sprintf("%d", cfg.Heartbeat)

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

func GetEnvConfig() *Config {
	c := &Config{}
	c.SendTimeoutMsec = GetEnvNumber("GOQ_SENDTIMEOUT_MSEC", 10000)
	c.RecvTimeoutMsec = GetEnvNumber("GOQ_RECVTIMEOUT_MSEC", 2000)

	myip := GetExternalIP()
	c.JservIP = GetEnvString("GOQ_JSERV_IP", myip)
	c.Home = os.Getenv("GOQ_HOME")
	c.Odir = GetEnvString("GOQ_ODIR", "o")
	c.JservPort = GetEnvNumber("GOQ_JSERV_PORT", 1776)
	//c.JservAddr = fmt.Sprintf("tcp://%s:%d", c.JservIP, c.JservPort)
	c.NoSshConfig = GetEnvBool("GOQ_NOSSHCONFIG", false)
	c.DebugMode = GetEnvBool("GOQ_DEBUGMODE", false)
	c.Heartbeat = TmSeconds(GetEnvNumber("GOQ_HEARTBEAT_SEC", 60))

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
	alphabet := "0123456789abcdef"

	buf := make([]byte, IdSz)
	for i := 0; i < IdSz; i++ {
		buf[i] = alphabet[rand.Intn(len(alphabet)-1)]
	}
	sbuf := string(buf) + GetExternalIP()

	rcid := Sha1sum(sbuf)

	if AesOff {
		fmt.Printf("RandomClusterId generated rcid='%s'\n", rcid)
	}
	return rcid
}

var validClusterId = regexp.MustCompile(`^[0-9a-f]{40}`)

func ShellOut(cmd string, args ...string) string {
	out, err := exec.Command(cmd, args...).Output()
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

var validClusterIdFile = regexp.MustCompile(`^goqclusterid$`)

func ClusterIdFileName(cfg *Config) string {
	return "goqclusterid"
}

func GetClusterIdPath(cfg *Config) string {
	if cfg.Home == "" {
		panic("cfg.Home must be set")
	}
	if !DirExists(cfg.Home) {
		panic(fmt.Sprintf("cfg.Home('%s') must be an existing directory", cfg.Home))
	}
	return fmt.Sprintf("%s/.goq/%s", cfg.Home, ClusterIdFileName(cfg))
}

func LoadLocalClusterId(cfg *Config) (string, error) {
	fn := GetClusterIdPath(cfg)
	cid, err := ReadAndTrimFile(fn)
	if err != nil {
		return "", err
	}
	return cid, nil
}

func ReadAndTrimFile(fn string) (string, error) {
	by, err := ioutil.ReadFile(fn)
	if err != nil {
		return "", err
	}
	return strings.Trim(string(by), " \t\n"), nil
}

func SaveLocalClusterId(id string, cfg *Config) {
	fn := GetClusterIdPath(cfg)
	err := MakeDotGoqDir(cfg)
	if err != nil {
		panic(err)
	}
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

func RemoveLocalClusterId(cfg *Config) error {
	fn := GetClusterIdPath(cfg)
	return os.Remove(fn)
}

func GetClusterIdFromFile(cfg *Config) *Config {
	cfg2 := *cfg
	filecid, _ := LoadLocalClusterId(&cfg2)
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
	InjectHelper(`GOQ_RECVTIMEOUT_MSEC`, fmt.Sprintf("%d", cfg.RecvTimeoutMsec))
	InjectHelper(`GOQ_JSERV_IP`, cfg.JservIP)
	InjectHelper(`GOQ_HOME`, cfg.Home)
	InjectHelper(`GOQ_ODIR`, cfg.Odir)
	InjectHelper(`GOQ_JSERV_PORT`, fmt.Sprintf("%d", cfg.JservPort))
	InjectHelper(`GOQ_NOSSHCONFIG`, BoolToString(cfg.NoSshConfig))
	InjectHelper(`GOQ_DEBUGMODE`, BoolToString(cfg.DebugMode))
	InjectHelper(`GOQ_HEARTBEAT_SEC`, fmt.Sprintf("%d", cfg.Heartbeat))
}

func (cfg *Config) InjectConfigIntoMap(addto *map[string]string) {

	MapInjectHelper(addto, `GOQ_SENDTIMEOUT_MSEC`, fmt.Sprintf("%d", cfg.SendTimeoutMsec))
	MapInjectHelper(addto, `GOQ_RECVTIMEOUT_MSEC`, fmt.Sprintf("%d", cfg.RecvTimeoutMsec))
	MapInjectHelper(addto, `GOQ_JSERV_IP`, cfg.JservIP)
	MapInjectHelper(addto, `GOQ_HOME`, cfg.Home)
	MapInjectHelper(addto, `GOQ_ODIR`, cfg.Odir)
	MapInjectHelper(addto, `GOQ_JSERV_PORT`, fmt.Sprintf("%d", cfg.JservPort))
	MapInjectHelper(addto, `GOQ_NOSSHCONFIG`, BoolToString(cfg.NoSshConfig))
	MapInjectHelper(addto, `GOQ_DEBUGMODE`, BoolToString(cfg.DebugMode))
	MapInjectHelper(addto, `GOQ_HEARTBEAT_SEC`, fmt.Sprintf("%d", cfg.Heartbeat))

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

func GetConfigFromFile(home string, defaults *Config) (*Config, error) {

	cfg := *defaults
	cfg.Home = home
	cid, err := LoadLocalClusterId(&cfg)
	cfg.ClusterId = cid

	ReadServerLoc(&cfg)

	return &cfg, err
}

func ServerLocFile(cfg *Config) string {
	return fmt.Sprintf("%s/.goq/serverloc", cfg.Home)
}

func WriteServerLoc(cfg *Config) error {
	fn := ServerLocFile(cfg)
	// keep private, 0600
	file, err := os.OpenFile(fn, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	fmt.Fprintf(file, "export GOQ_JSERV_IP=%s\n", cfg.JservIP)
	fmt.Fprintf(file, "export GOQ_JSERV_PORT=%d\n", cfg.JservPort)
	fmt.Fprintf(file, "export GOQ_SENDTIMEOUT_MSEC=%d\n", cfg.SendTimeoutMsec)
	fmt.Fprintf(file, "export GOQ_RECVTIMEOUT_MSEC=%d\n", cfg.RecvTimeoutMsec)
	fmt.Fprintf(file, "export GOQ_HEARTBEAT_SEC=%d\n", cfg.Heartbeat)

	return nil
}

func ReadServerLoc(cfg *Config) error {
	fn := ServerLocFile(cfg)
	file, err := os.Open(fn)
	if err != nil {
		return nil
	}
	defer file.Close()
	fmt.Fscanf(file, "export GOQ_JSERV_IP=%s\n", &cfg.JservIP)
	fmt.Fscanf(file, "export GOQ_JSERV_PORT=%d\n", &cfg.JservPort)
	fmt.Fscanf(file, "export GOQ_SENDTIMEOUT_MSEC=%d\n", &cfg.SendTimeoutMsec)
	fmt.Fscanf(file, "export GOQ_RECVTIMEOUT_MSEC=%d\n", &cfg.RecvTimeoutMsec)
	fmt.Fscanf(file, "export GOQ_HEARTBEAT_SEC=%d\n", &cfg.Heartbeat)

	return nil
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

func MakeDotGoqDir(cfg *Config) error {
	if cfg.Home == "" {
		panic("cfg.Home cannot be empty")
	}
	d := cfg.Home + "/.goq"
	if !DirExists(d) {
		err := os.MkdirAll(d, 0700)
		if err != nil {
			return fmt.Errorf("error creating directory '%s': %s", d, err)
		}
	}
	return nil
}

func DeleteDotGoqDir(cfg *Config) {
	if cfg.Home == "" {
		panic("cfg.Home cannot be empty")
	}
	d := cfg.Home + "/.goq"
	if DirExists(d) {
		err := os.RemoveAll(d)
		if err != nil {
			panic(err)
		}
	}
}

func FindGoqHome() (h string, err error) {
	home := os.Getenv("GOQ_HOME")
	if home == "" {
		err = fmt.Errorf("GOQ_HOME environment variable not found")
	}
	return home, err
}

func (cfg *Config) KeyLocation() string {
	return cfg.Home + "/.goq/aes"
}

func GenNewCreds(cfg *Config) {
	var err error
	cfg.ClusterId = RandomClusterId()
	err = MakeDotGoqDir(cfg)
	if err != nil {
		panic(err)
	}
	SaveLocalClusterId(cfg.ClusterId, cfg)
	cfg.Cypher, err = NewKey(cfg)
	if err != nil {
		panic(err)
	}
	err = WriteServerLoc(cfg)
	if err != nil {
		panic(err)
	}
}
