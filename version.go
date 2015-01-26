package main

import "fmt"

func goq_version() string {
	return fmt.Sprintf("1.0/%s", LASTGITCOMMITHASH)
}
