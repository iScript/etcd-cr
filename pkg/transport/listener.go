package transport

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/iScript/etcd-cr/pkg/tlsutil"
	"go.uber.org/zap"
)

// 生成一个listner.
func NewListener(addr, scheme string, tlsinfo *TLSInfo) (l net.Listener, err error) {
	if l, err = newListener(addr, scheme); err != nil {
		return nil, err
	}
	return wrapTLS(scheme, tlsinfo, l)
}

func newListener(addr string, scheme string) (net.Listener, error) {
	if scheme == "unix" || scheme == "unixs" {
		// unix sockets via unix://laddr
		// 暂时用不到？
		return NewUnixListener(addr)
	}
	return net.Listen("tcp", addr)
}

func wrapTLS(scheme string, tlsinfo *TLSInfo, l net.Listener) (net.Listener, error) {
	if scheme != "https" && scheme != "unixs" {
		return l, nil
	}
	return l, nil
	// if tlsinfo != nil && tlsinfo.SkipClientSANVerify {
	// 	return NewTLSListener(l, tlsinfo)
	// }
	// return newTLSListener(l, tlsinfo, checkSAN)
}

// TSL 加密相关
type TLSInfo struct {
	CertFile           string
	KeyFile            string
	TrustedCAFile      string
	ClientCertAuth     bool
	CRLFile            string
	InsecureSkipVerify bool

	// ServerName ensures the cert matches the given host in case of discovery / virtual hosting
	ServerName string

	// HandshakeFailure is optionally called when a connection fails to handshake. The
	// connection will be closed immediately afterwards.
	HandshakeFailure func(*tls.Conn, error)

	// CipherSuites is a list of supported cipher suites.
	// If empty, Go auto-populates it by default.
	// Note that cipher suites are prioritized in the given order.
	CipherSuites []uint16

	selfCert bool

	// parseFunc exists to simplify testing. Typically, parseFunc
	// should be left nil. In that case, tls.X509KeyPair will be used.
	parseFunc func([]byte, []byte) (tls.Certificate, error)

	// AllowedCN is a CN which must be provided by a client.
	AllowedCN string

	// AllowedHostname is an IP address or hostname that must match the TLS
	// certificate provided by a client.
	AllowedHostname string

	// Logger logs TLS errors.
	// If nil, all logs are discarded.
	Logger *zap.Logger

	// EmptyCN indicates that the cert must have empty CN.
	// If true, ClientConfig() will return an error for a cert with non empty CN.
	EmptyCN bool
}

// 返回字符串，结构体被输出的时候自动调用
func (info TLSInfo) String() string {
	return fmt.Sprintf("cert = %s, key = %s, trusted-ca = %s, client-cert-auth = %v, crl-file = %s", info.CertFile, info.KeyFile, info.TrustedCAFile, info.ClientCertAuth, info.CRLFile)
}

//判断证书和key是否为空
func (info TLSInfo) Empty() bool {
	return info.CertFile == "" && info.KeyFile == ""
}

func SelfCert(lg *zap.Logger, dirpath string, hosts []string, additionalUsages ...x509.ExtKeyUsage) (info TLSInfo, err error) {
	if err = os.MkdirAll(dirpath, 0700); err != nil {
		return
	}
	info.Logger = lg

	certPath := filepath.Join(dirpath, "cert.pem")
	keyPath := filepath.Join(dirpath, "key.pem")
	_, errcert := os.Stat(certPath)
	_, errkey := os.Stat(keyPath)
	if errcert == nil && errkey == nil {
		info.CertFile = certPath
		info.KeyFile = keyPath
		info.selfCert = true
		return
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		if info.Logger != nil {
			info.Logger.Warn(
				"cannot generate random number",
				zap.Error(err),
			)
		}
		return
	}

	tmpl := x509.Certificate{
		SerialNumber: serialNumber,
		Subject:      pkix.Name{Organization: []string{"etcd"}},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(365 * (24 * time.Hour)),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           append([]x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, additionalUsages...),
		BasicConstraintsValid: true,
	}

	for _, host := range hosts {
		h, _, _ := net.SplitHostPort(host)
		if ip := net.ParseIP(h); ip != nil {
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		} else {
			tmpl.DNSNames = append(tmpl.DNSNames, h)
		}
	}

	priv, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	if err != nil {
		if info.Logger != nil {
			info.Logger.Warn(
				"cannot generate ECDSA key",
				zap.Error(err),
			)
		}
		return
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	if err != nil {
		if info.Logger != nil {
			info.Logger.Warn(
				"cannot generate x509 certificate",
				zap.Error(err),
			)
		}
		return
	}

	certOut, err := os.Create(certPath)
	if err != nil {
		info.Logger.Warn(
			"cannot cert file",
			zap.String("path", certPath),
			zap.Error(err),
		)
		return
	}
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	certOut.Close()
	if info.Logger != nil {
		info.Logger.Info("created cert file", zap.String("path", certPath))
	}

	b, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return
	}
	keyOut, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		if info.Logger != nil {
			info.Logger.Warn(
				"cannot key file",
				zap.String("path", keyPath),
				zap.Error(err),
			)
		}
		return
	}
	pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: b})
	keyOut.Close()
	if info.Logger != nil {
		info.Logger.Info("created key file", zap.String("path", keyPath))
	}
	return SelfCert(lg, dirpath, hosts)
}

func (info TLSInfo) baseConfig() (*tls.Config, error) {
	if info.KeyFile == "" || info.CertFile == "" {
		return nil, fmt.Errorf("KeyFile and CertFile must both be present[key: %v, cert: %v]", info.KeyFile, info.CertFile)
	}
	if info.Logger == nil {
		info.Logger = zap.NewNop()
	}

	_, err := tlsutil.NewCert(info.CertFile, info.KeyFile, info.parseFunc)
	if err != nil {
		return nil, err
	}

	cfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
		ServerName: info.ServerName,
	}

	if len(info.CipherSuites) > 0 {
		cfg.CipherSuites = info.CipherSuites
	}

	var verifyCertificate func(*x509.Certificate) bool
	if info.AllowedCN != "" {
		if info.AllowedHostname != "" {
			return nil, fmt.Errorf("AllowedCN and AllowedHostname are mutually exclusive (cn=%q, hostname=%q)", info.AllowedCN, info.AllowedHostname)
		}
		verifyCertificate = func(cert *x509.Certificate) bool {
			return info.AllowedCN == cert.Subject.CommonName
		}
	}
	if info.AllowedHostname != "" {
		verifyCertificate = func(cert *x509.Certificate) bool {
			return cert.VerifyHostname(info.AllowedHostname) == nil
		}
	}
	if verifyCertificate != nil {
		cfg.VerifyPeerCertificate = func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			for _, chains := range verifiedChains {
				if len(chains) != 0 {
					if verifyCertificate(chains[0]) {
						return nil
					}
				}
			}
			return errors.New("client certificate authentication failed")
		}
	}

	// ..

	return cfg, nil
}

// cafiles returns a list of CA file paths.
func (info TLSInfo) cafiles() []string {
	cs := make([]string, 0)
	if info.TrustedCAFile != "" {
		cs = append(cs, info.TrustedCAFile)
	}
	return cs
}

// func (info TLSInfo) ServerConfig() (*tls.Config, error) {
// 	cfg, err := info.baseConfig()
// 	if err != nil {
// 		return nil, err
// 	}

// 	cfg.ClientAuth = tls.NoClientCert
// 	if info.TrustedCAFile != "" || info.ClientCertAuth {
// 		cfg.ClientAuth = tls.RequireAndVerifyClientCert
// 	}

// 	cs := info.cafiles()
// 	if len(cs) > 0 {
// 		cp, err := tlsutil.NewCertPool(cs)
// 		if err != nil {
// 			return nil, err
// 		}
// 		cfg.ClientCAs = cp
// 	}

// 	// "h2" NextProtos is necessary for enabling HTTP2 for go's HTTP server
// 	cfg.NextProtos = []string{"h2"}

// 	return cfg, nil
// }

// 生成一个tls.Config对象供http客户端使用
func (info TLSInfo) ClientConfig() (*tls.Config, error) {
	var cfg *tls.Config
	var err error

	//TSLInfo 默认为空，走else
	if !info.Empty() {
		cfg, err = info.baseConfig()
		if err != nil {
			return nil, err
		}
	} else {
		cfg = &tls.Config{ServerName: info.ServerName}
	}

	cfg.InsecureSkipVerify = info.InsecureSkipVerify

	cs := info.cafiles() //默认空数组
	if len(cs) > 0 {
		// cfg.RootCAs, err = tlsutil.NewCertPool(cs)
		// if err != nil {
		// 	return nil, err
		// }
	}

	if info.selfCert {
		cfg.InsecureSkipVerify = true
	}

	if info.EmptyCN {
		//..
	}

	return cfg, nil
}
