package random

import "os"

var source *os.File

func init() {
	var err error
	source, err = os.Open("/dev/urandom")
	if err != nil {
		panic(err)
	}
}

func GetRandomBytes(len int) []byte {
	x := make([]byte, len)
	source.Read(x)
	return x
}
