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
//
// have to zero out both Signature and DesinationSocket before signing.
//
// SignJob() cannot modify Job, since it is used on the reciever for verification too.
//
func SignJob(j *Job, cfg *Config) {
	j.Signature = ""
	saveSock := j.destinationSock
	j.destinationSock = nil

	str := fmt.Sprintf("%#v\nclusterid:%s", *j, cfg.ClusterId)
	if ShowSig {
		TSPrintf("\n SignJob() signing this: '%s'\n", str)
	}
	j.Signature = Sha1sum(str)
	j.destinationSock = saveSock
}

func Sha1sum(s string) string {
	sum := sha1.Sum([]byte(s))
	slsum := sum[:]
	return fmt.Sprintf("%x", slsum)
}
