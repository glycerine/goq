package main

import (
	"crypto/sha1"
	"fmt"
)

// check that message came from someone with
// who signed it with the same clusterid. The
// signature itself has to be blanked first.
func GetJobSignature(j *Job, cfg *Config) string {
	jobcopy := *j
	SignJob(&jobcopy, cfg)
	return jobcopy.Signature
}

func JobSignatureOkay(j *Job, cfg *Config) bool {
	sig := j.Signature

	computed := GetJobSignature(j, cfg)
	if computed == sig {
		return true
	}
	return false
}

// sign by filling in the j.Signature field
func SignJob(j *Job, cfg *Config) {
	j.Signature = ""
	str := fmt.Sprintf("%#v%s", j, cfg.ClusterId)
	j.Signature = Sha1sum(str)
}

func Sha1sum(s string) string {
	sum := sha1.Sum([]byte(s))
	slsum := sum[:]
	return fmt.Sprintf("%x", slsum)
}
