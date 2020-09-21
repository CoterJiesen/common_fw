package auth

import (
	"math/rand"
	"os"
	"testing"
	"time"
)

import proto "github.com/golang/protobuf/proto"

func doTest(t testing.TB) {
	rawinfo := &RawInfo{}
	rawinfo.Uid = proto.Uint64(uint64(rand.Int63n(int64(100000000)) + int64(1)))

	fp, err := os.Open("/dev/urandom")
	if err != nil {
		t.Fatalf("open random file failed: %v", err)
		t.Fail()
	}
	defer fp.Close()
	seq := make([]byte, rand.Intn(int(32))+int(1))
	fp.Read(seq)
	rand_str := string(seq)

	rawinfo.Deviceid = &rand_str

	rawinfo.ExpiredTime = proto.Int64(rand.Int63n(int64(100000000)) + time.Now().Unix())

	token, e := GenToken(rawinfo)
	if e != nil {
		t.Fatalf("gen token failed: %v", e)
		t.Fail()
	}

	rawinfo_new, e := CheckToken(token)
	if e != nil {
		t.Fatalf("check token failed: %v", e)
		t.Fail()
	}

	if rawinfo_new.GetUid() != rawinfo.GetUid() {
		t.Fatalf("uid not match %v != %v", rawinfo_new.GetUid(), rawinfo.GetUid())
		t.Fail()
	}

	//t.Logf("%v\n", rawinfo_new)
}

func doTest2(t testing.TB) {
	rawinfo := &RawInfo{}
	rawinfo.Uid = proto.Uint64(uint64(rand.Int63n(int64(100000000)) + int64(1)))

	fp, err := os.Open("/dev/urandom")
	if err != nil {
		t.Fatalf("open random file failed: %v", err)
		t.Fail()
	}
	defer fp.Close()
	seq := make([]byte, rand.Intn(int(32))+int(1))
	fp.Read(seq)
	rand_str := string(seq)

	rawinfo.Deviceid = &rand_str

	rawinfo.ExpiredTime = proto.Int64(time.Now().Unix() - rand.Int63n(int64(100000)))

	token, e := GenToken(rawinfo)
	if e != nil {
		t.Fatalf("gen token failed: %v", e)
		t.Fail()
	}

	_, e = CheckToken(token)
	if e == nil {
		t.Fatalf("check token success: wrong")
		t.Fail()
	}
	//t.Logf("%v\n", rawinfo_new)
}

func TestAll(t *testing.T) {
	SetPublicKey("/tmp/public.pem")
	SetPrivateKey("/tmp/private.pem")
	for i := 1; i < 1000; i++ {
		doTest(t)
		doTest2(t)
	}
}
