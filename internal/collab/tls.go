package collab

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Hosts use a self-signed certificate kept in the settings folder, so its
// fingerprint stays the same between sessions: invite codes carry it, and
// guests check that they talk to the host they were invited to.

// fingerprintBytes is how much of the SHA-256 of the certificate a
// fingerprint keeps (128 bits).
const fingerprintBytes = 16

func fingerprintOf(der []byte) string {
	sum := sha256.Sum256(der)
	return hex.EncodeToString(sum[:fingerprintBytes])
}

// LoadCertificate reads the host certificate at path, creating it first
// when there is none ("" keeps a new one in memory only).
func LoadCertificate(path string) (tls.Certificate, string, error) {
	if path != "" {
		if data, err := os.ReadFile(path); err == nil {
			if cert, err := tls.X509KeyPair(data, data); err == nil && len(cert.Certificate) > 0 {
				return cert, fingerprintOf(cert.Certificate[0]), nil
			}
		}
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, "", err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	if err != nil {
		return tls.Certificate{}, "", err
	}
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "OTS Creator map session"},
		NotBefore:    time.Now().Add(-24 * time.Hour),
		NotAfter:     time.Now().AddDate(30, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, "", err
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return tls.Certificate{}, "", err
	}
	data := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	data = append(data, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})...)
	if path != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err == nil {
			_ = os.WriteFile(path, data, 0o600)
		}
	}
	cert, err := tls.X509KeyPair(data, data)
	if err != nil {
		return tls.Certificate{}, "", err
	}
	return cert, fingerprintOf(der), nil
}

func serverTLS(cert tls.Certificate) *tls.Config {
	return &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS13}
}

// ErrFingerprint means the host is not the one of the invite code.
var ErrFingerprint = errors.New("the host's certificate does not match the invite code: it may be someone else at that address")

// clientTLS pins the host certificate to fingerprint; with an empty one any
// certificate is accepted and seen gets its fingerprint (trust on first
// use, for addresses typed by hand).
func clientTLS(fingerprint string, seen *string) *tls.Config {
	return &tls.Config{
		MinVersion:         tls.VersionTLS13,
		InsecureSkipVerify: true, // self-signed: checked by fingerprint below
		VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			if len(rawCerts) == 0 {
				return errors.New("the host sent no certificate")
			}
			got := fingerprintOf(rawCerts[0])
			if seen != nil {
				*seen = got
			}
			if fingerprint != "" && !strings.EqualFold(got, fingerprint) {
				return ErrFingerprint
			}
			return nil
		},
	}
}
