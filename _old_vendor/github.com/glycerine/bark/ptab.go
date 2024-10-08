package bark

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// process table generation

var validProcessNumber = regexp.MustCompile(`^[0-9/]+$`)

func IsNumberString(s string) bool {
	match := validProcessNumber.FindStringSubmatch(s)
	if match == nil {
		return false
	}
	return true
}

func init() {
	detectSystem()
}

var Sysname string

func detectSystem() {
	c := exec.Command("uname", "-s")
	o, err := c.Output()

	if err == nil {
		Sysname = strings.Trim(string(o), " \n\t")
	}
}

func ProcessTable() *map[int]bool {
	switch Sysname {
	case "Linux":
		return LinuxPs()

	case "Darwin":
		return DarwinPs()
	}
	m := make(map[int]bool)
	return &m
}

var regexDarwinPs = regexp.MustCompile(`^[^0-9]*[0-9]+[^0-9]+([0-9]+)`)

func DarwinPs() *map[int]bool {

	res := make(map[int]bool)
	o, err := exec.Command("/bin/ps", "-ef").Output()
	if err != nil {
		panic(err)
	}
	s := string(o)

	lines := strings.Split(s, "\n")
	for i := range lines {
		match := regexDarwinPs.FindStringSubmatch(lines[i])
		if match != nil && len(match) == 2 {
			//fmt.Printf("match = %#v\n", match[1])
			num, err := strconv.Atoi(match[1])
			if err == nil {
				res[num] = true
			}
		}
	}
	return &res
}

func LinuxPs() *map[int]bool {

	res := make(map[int]bool)

	fileInfoSlice, err := ioutil.ReadDir("/proc")
	if err != nil {
		panic(err)
	}

	for i := range fileInfoSlice {
		if fileInfoSlice[i].IsDir() {
			if IsNumberString(fileInfoSlice[i].Name()) {
				num, err := strconv.Atoi(fileInfoSlice[i].Name())
				if err != nil {
					panic("IsNumberString regex failed to filter down to only numbers")
				}
				res[num] = true
			}
		}
	}
	return &res
}

func OpenFiles(pid int) []string {
	switch Sysname {
	case "Linux":
		return LinuxOpenFiles(pid)

	case "Darwin":
		return DarwinOpenFiles(pid)
	}
	return []string{}
}

func LinuxOpenFiles(pid int) []string {

	// wait until process shows up (in /proc or ps)
	waited := 0
	for {
		pt := *ProcessTable()
		_, jsAlive := pt[pid]
		if jsAlive {
			break
		}
		time.Sleep(50 * time.Millisecond)
		waited++
		if waited > 10 {
			panic(fmt.Sprintf("jobserv with expected pid %d did not show up in /proc after 10 waits", pid))
		}
	}
	fmt.Printf("\njobserv with expected pid %d was *found* in /proc after %d waits of 50msec\n", pid, waited)

	/*
		// read all the open files
		fileInfoSlice, err := ioutil.ReadDir(fmt.Sprintf("/proc/%d/fd", pid))
		if err != nil {
			panic(err)
		}

		res := make([]string, len(fileInfoSlice))
		for i := range fileInfoSlice {
			res[i] = fileInfoSlice[i].Name()
		}

		return res
	*/

	spid := fmt.Sprintf("%d", pid)
	//fmt.Printf("about to do: lsof -p %s -Fnt\n", spid)
	o, err := exec.Command("/usr/bin/lsof", "-p", spid, "-Fnt").Output()
	if err != nil {
		panic(err)
	}
	s := strings.Split(string(o), "\n")

	// join the name and type together on one line
	// skip the header line
	s = s[1:]
	r := make([]string, 0)
	n := len(s) - 1 // empty string at end
	//fmt.Printf("len(s) = %d; s='%#v'\n", len(s), s)
	for i := 0; i < n; i += 2 {
		//fmt.Printf("on i = %d, s[i]='%s', s[i+i]='%s'\n", i, s[i], s[i+1])
		r = append(r, s[i]+":"+s[i+1])
	}

	//fmt.Printf("LinuxOpenFiles got back from lsof:\n")
	//ShowStrings(r)

	return r

}

func DarwinOpenFiles(pid int) []string {
	// wait until process shows up (in /proc or ps)
	waited := 0
	for {
		pt := *ProcessTable()
		_, jsAlive := pt[pid]
		if jsAlive {
			break
		}
		time.Sleep(50 * time.Millisecond)
		waited++
		if waited > 10 {
			panic(fmt.Sprintf("jobserv with expected pid %d did not show up in /proc after 10 waits", pid))
		}
	}
	fmt.Printf("\njobserv with expected pid %d was *found* in /proc after %d waits of 50msec\n", pid, waited)

	spid := fmt.Sprintf("%d", pid)
	//fmt.Printf("about to do: lsof -p %s -Fnt\n", spid)
	o, err := exec.Command("/usr/sbin/lsof", "-p", spid, "-Fnt").Output()
	if err != nil {
		panic(err)
	}
	s := strings.Split(string(o), "\n")

	// join the name and type together on one line
	// skip the header line
	s = s[1:]
	r := make([]string, 0)
	n := len(s) - 1 // empty string at end
	//fmt.Printf("len(s) = %d; s='%#v'\n", len(s), s)
	for i := 0; i < n; i += 2 {
		//fmt.Printf("on i = %d, s[i]='%s', s[i+i]='%s'\n", i, s[i], s[i+1])
		r = append(r, s[i]+":"+s[i+1])
	}

	//fmt.Printf("DarwinOpenFiles got back from lsof:\n")
	//ShowStrings(r)

	return r
}

func SetDiff(a, b []string) []string {
	res := make([]string, 0)
	m := make(map[string]bool)
	for i := range a {
		m[a[i]] = true
	}
	for i := range b {
		delete(m, b[i])
	}
	for s := range m {
		res = append(res, s)
	}
	return res
}

func ShowStrings(s []string) {
	for _, line := range s {
		fmt.Printf("%s\n", line)
	}
}

func WaitForShutdownWithTimeout(pid int, timeout time.Duration) error {
	//	time.Sleep(10 * time.Millisecond)
	waited := 0
	t0 := time.Now()
	for {
		pt := *ProcessTable()
		Q("pt = %#v\n", pt)
		_, alive := pt[pid]
		if !alive {
			Q("pid %d done after %d waits.\n", pid, waited)
			return nil
		}
		Q("pid %d is still alive...\n", pid)
		time.Sleep(10 * time.Millisecond)
		waited++
		if time.Since(t0) > timeout {
			return fmt.Errorf("pid %d did not disappear from process table", pid)
		}
	}
	return nil
}
