package auth

import (
	"bytes"
	"fmt"
	"time"
)

import (
	"github.com/dgkang/rsa/rsa"
	"github.com/golang/protobuf/proto"
)

var (
	public_key_path  string = ""
	private_key_path string = ""
)

func SetPublicKey(path string) {
	public_key_path = path
}

func SetPrivateKey(path string) {
	private_key_path = path
}

func GenToken(raw *RawInfo) ([]byte, error) {
	if raw == nil {
		return nil, fmt.Errorf("raw is nil")
	}

	//编码raw
	raw_buffer, e := proto.Marshal(raw)
	if e != nil {
		return nil, e
	}

	//加密buffer生成cipher
	cipher, e := rsa.PrivateEncrypt(raw_buffer, private_key_path, rsa.RSA_PKCS1_PADDING)
	if e != nil {
		return nil, e
	}

	//编码整个token
	token := &Token{Raw: raw, Cipher: cipher}
	token_buffer, e := proto.Marshal(token)
	if e != nil {
		return nil, e
	}
	return token_buffer, nil
}

func CheckToken(buffer []byte) (*RawInfo, error) {
	token := &Token{}
	//解码buffer
	e := proto.Unmarshal(buffer, token)
	if e != nil {
		return nil, e
	}

	cipher := token.GetCipher()
	if cipher == nil {
		return nil, fmt.Errorf("can't find cipher")
	}

	raw := token.GetRaw()
	if raw == nil {
		return nil, fmt.Errorf("can't find raw")
	}

	//解密密文
	raw_buffer, e := rsa.PublicDecrypt(cipher, public_key_path, rsa.RSA_PKCS1_PADDING)
	if e != nil {
		return nil, e
	}

	//比较明文和解密的密文
	raw_buffer2, e := proto.Marshal(raw)
	if e != nil {
		return nil, e
	}

	if !bytes.Equal(raw_buffer, raw_buffer2) {
		return nil, fmt.Errorf("cipher(%v) != raw(%v)", raw_buffer, raw_buffer2)
	}

	//检查有效期
	if raw.GetExpiredTime() < time.Now().Unix() {
		return nil, fmt.Errorf("token expired in %v", raw.GetExpiredTime())
	}

	//检查uid
	if raw.GetUid() == 0 {
		return nil, fmt.Errorf("invalid uid %v", raw.GetUid())
	}

	return raw, nil
}
