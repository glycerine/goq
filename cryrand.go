package main

import (
	cryptorand "crypto/rand"
	"encoding/binary"
)

// Use crypto/rand to get an random int64
func CryptoRandInt64() int64 {
	c := 8
	b := make([]byte, c)
	_, err := cryptorand.Read(b)
	if err != nil {
		panic(err)
	}
	r := int64(binary.LittleEndian.Uint64(b))
	return r
}
