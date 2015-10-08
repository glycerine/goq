package main

import (
	cryptorand "crypto/rand"
	"crypto/sha256"

	nacl "golang.org/x/crypto/nacl/secretbox"
)

const LEN_NONCE_BYTES = 24

func generateNonce24(nonce *[LEN_NONCE_BYTES]byte) {
	_, err := cryptorand.Read(nonce[:])
	if err != nil {
		panic(err)
	}
}

func HashAlotSha256(input []byte) [32]byte {
	var res [32]byte
	hashprev := input
	for i := 0; i < 100; i++ {
		res = sha256.Sum256(hashprev)
		hashprev = res[:]
		//fmt.Printf("hashprev = '%v'\n", string(hashprev))
	}
	return res
}

func NaClEncryptWithRandomNoncePrepended(message []byte, key *[32]byte) []byte {
	var nonce [LEN_NONCE_BYTES]byte
	generateNonce24(&nonce)
	return nacl.Seal(nonce[:], message, &nonce, key)
}

func NaclDecryptWithNoncePrepended(box []byte, key *[32]byte) ([]byte, bool) {
	if len(box) < LEN_NONCE_BYTES {
		panic("box too small!: must have 24 byte nonce prepended to it")
		return []byte{}, false
	}
	var nonce [LEN_NONCE_BYTES]byte
	copy(nonce[:], box[:LEN_NONCE_BYTES])
	return nacl.Open(nil, box[LEN_NONCE_BYTES:], &nonce, key)
}
