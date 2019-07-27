package tlsutil

import (
	"crypto/tls"
	"io/ioutil"
)

func NewCert(certfile, keyfile string, parseFunc func([]byte, []byte) (tls.Certificate, error)) (*tls.Certificate, error) {
	cert, err := ioutil.ReadFile(certfile)
	if err != nil {
		return nil, err
	}

	key, err := ioutil.ReadFile(keyfile)
	if err != nil {
		return nil, err
	}

	if parseFunc == nil {
		parseFunc = tls.X509KeyPair //X509 证书标准 ， 从一对PEM编码的数据中解析公钥/私钥对。
	}

	tlsCert, err := parseFunc(cert, key)
	if err != nil {
		return nil, err
	}
	return &tlsCert, nil
}
