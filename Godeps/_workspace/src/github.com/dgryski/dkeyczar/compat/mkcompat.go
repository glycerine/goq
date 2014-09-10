package main

import (
	"fmt"
	"os"
	"strconv"
	"github.com/dgryski/dkeyczar"
)

var TESTDATA = ""
var PLAINTEXT = "This is test data"

func init() {
	TESTDATA = os.Getenv("KEYCZAR_TESTDATA")
}

func writeHeader() {
	fmt.Println(`
from keyczar import keyczar
from keyczar import readers
from keyczar import errors

class JSONReader(readers.Reader):
    def __init__(self, meta, keys):
        self.meta = meta
        self.keys = keys

    def GetMetadata(self):
        return self.meta

    def GetKey(self, version):
        return self.keys[version]

    def Close():
        pass

def check_verify(reader, what, plaintext, signature):
    try:
        verifier = keyczar.Verifier(reader)
        valid = verifier.Verify(plaintext, signature)
        if valid:
            print "ok verify: ", what
        else:
            print "FAIL VERIFY: ", what
    except errors.KeyczarError as e:
        print "FAIL VERIFY (exception): ", what, ": ", e

def check_attached_verify(reader, what, plaintext, signeddata, nonce):
    try:
        verifier = keyczar.Verifier(reader)
        msg = verifier.AttachedVerify(signeddata, nonce)
        if msg == plaintext:
            print "ok attached verify: ", what
        else:
            print "FAIL ATTACHED VERIFY: ", what
    except errors.KeyczarError as e:
        print "FAIL ATTACHED VERIFY (exception): ", what, ": ", e


def check_unversioned_verify(reader, what, plaintext, signeddata):
    try:
        verifier = keyczar.UnversionedVerifier(reader)
        valid = verifier.Verify(plaintext, signeddata)
        if valid:
            print "ok unversioned verify: ", what
        else:
            print "FAIL UNVERSIONED VERIFY: ", what
    except errors.KeyczarError as e:
        print "FAIL UNVERSIONED VERIFY (exception): ", what, ": ", e


def check_decrypt(reader, what, plaintext, ciphertext):
    try:
        crypter = keyczar.Crypter(reader)
        decrypted = crypter.Decrypt(ciphertext)
        if decrypted == plaintext:
            print "ok decrypt: ", what
        else:
            print "FAIL DECRYPT: ", what
    except errors.KeyczarError as e:
        print "FAIL DECRYPT (exception): ", what, ": ", e

`)
}

func writeDecryptTest(dir string) {

	fulldir := TESTDATA + dir

	r := dkeyczar.NewFileReader(fulldir)
	crypter, _ := dkeyczar.NewCrypter(r)

	ciphertext, _ := crypter.Encrypt([]byte(PLAINTEXT))
	fmt.Println(`
check_decrypt(readers.FileReader("` + fulldir + `"),
    "` + dir + `",
    "` + PLAINTEXT + `",
    "` + ciphertext + `",
)
`)

}

func writeVerifyTest(dir string) {

	fulldir := TESTDATA + dir

	r := dkeyczar.NewFileReader(fulldir)

	signer, _ := dkeyczar.NewSigner(r)

	signature, _ := signer.Sign([]byte(PLAINTEXT))

	fmt.Println(`
check_verify(readers.FileReader("` + fulldir + `"),
    "` + dir + `",
    "` + PLAINTEXT + `",
    "` + signature + `",
)`)

	attachedSig, _ := signer.AttachedSign([]byte(PLAINTEXT), []byte{0, 1, 2, 3, 4, 5, 6, 7})

	fmt.Println(`
check_attached_verify(readers.FileReader("` + fulldir + `"),
    "` + dir + `",
    "` + PLAINTEXT + `",
    "` + attachedSig + `",
    "\x00\x01\x02\x03\x04\x05\x06\x07",
)`)

	attachedSig, _ = signer.AttachedSign([]byte(PLAINTEXT), nil)

	fmt.Println(`
check_attached_verify(readers.FileReader("` + fulldir + `"),
    "` + dir + `",
    "` + PLAINTEXT + `",
    "` + attachedSig + `",
    None
)`)

	unversionedSig, _ := signer.UnversionedSign([]byte(PLAINTEXT))

	fmt.Println(`
check_unversioned_verify(readers.FileReader("` + fulldir + `"),
    "` + dir + `",
    "` + PLAINTEXT + `",
    "` + unversionedSig + `",
)`)

}

func writeKeyczartTest(dir string) {

	fulldir := TESTDATA + dir

	km := dkeyczar.NewKeyManager()

	r := dkeyczar.NewFileReader(fulldir)

	km.Load(r)

	json := km.ToJSONs(nil)

	fmt.Println(`

meta = """` + json[0] + `"""
keys={`)

	for i := 1; i < len(json); i++ {
		fmt.Println("    " + strconv.Itoa(i) + `: """` + json[i] + `""",`)
	}
	fmt.Println(`}
r = JSONReader(meta, keys)`)
	signer, _ := dkeyczar.NewSigner(r)
	if signer != nil {

		signature, _ := signer.Sign([]byte(PLAINTEXT))

		fmt.Println(
			`check_verify(r,
        "json ` + dir + `",
        "` + PLAINTEXT + `",
        "` + signature + `",
)`)
	} else {
		crypter, _ := dkeyczar.NewCrypter(r)

		ciphertext, _ := crypter.Encrypt([]byte(PLAINTEXT))
		fmt.Println(
			`check_decrypt(r,
    "json ` + dir + `",
    "` + PLAINTEXT + `",
    "` + ciphertext + `",
)`)
	}

}

func writeFooter() {
	// empty
}

func main() {

	writeHeader()

	for _, k := range []string{"aes", "rsa"} {
		writeDecryptTest(k)
	}

	for _, k := range []string{"hmac", "rsa-sign", "dsa"} {
		writeVerifyTest(k)
	}

	for _, k := range []string{"aes", "rsa", "hmac", "rsa-sign", "dsa"} {
		writeKeyczartTest(k)
	}

	writeFooter()
}
