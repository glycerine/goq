package main

// copyright(c) 2014, Jason E. Aten
//
// goq : a simple queueing system in go; qsub replacement.
//

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func Shepard(dir string, cmd string, args []string, env []string) (out []string, err error) {

	var origdir string
	origdir, err = os.Getwd()
	if err != nil {
		panic(err)
	}

	if dir != "" {
		err = os.Chdir(dir)
		if err != nil {
			out = make([]string, 1)
			out[0] = fmt.Sprintf("Shepard got error trying to move to submit directory with os.Chdir('%s'): %s", dir, err)
			return out, err
		}
		// go back to our starting dir at the end of shepding this job.
		defer os.Chdir(origdir)
	}

	c := exec.Command(cmd, args...)
	c.Dir = dir
	c.Env = env
	var oe []byte
	oe, err = c.CombinedOutput()

	s := string(oe)
	strings.Trim(s, "\n")
	slen := len(s)
	out = strings.Split(s, "\n")
	// if file ended in '\n' then we now have an extra empty line to eliminate.
	N := len(out)
	if slen > 0 && s[slen-1] == '\n' && N > 0 && out[N-1] == "" {
		out = out[:N-1]
	}
	if err != nil {
		out = append(out, fmt.Sprintf("Shepard finds non-nil err on running cmd '%s' in dir '%s': %s", cmd, dir, err))
	}

	return out, err
}
