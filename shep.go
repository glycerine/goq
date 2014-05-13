package main

// copyright(c) 2014, Jason E. Aten
//
// goq : a simple queueing system in go; qsub replacement.
//

import (
	"fmt"
	"os/exec"
	"strings"
)

func Shepard(dir string, cmd string) (out []string, err error) {
	c := exec.Command(cmd)
	c.Dir = dir
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
