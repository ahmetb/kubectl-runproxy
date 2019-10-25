package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"time"
)

func generateKeyPair() (cert tls.Certificate, err error) {
	priv, err := generatePrivateKey(2048)
	if err != nil {
		return cert,  fmt.Errorf("failed to generate private key: %v", err)
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return cert, fmt.Errorf("failed to generate serial number: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Acme Co"},
		},
		DNSNames: []string{
			"localhost",
			"lokalhost.local",
		},

		NotBefore: time.Now().Add(-time.Hour),
		NotAfter:  time.Now().AddDate(0, 0, 1),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	certDerBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, priv.Public(), priv)
	if err != nil {
		log.Fatalf("Failed to create certificate: %s", err)
	}

	cert.Certificate = append(cert.Certificate, certDerBytes)
	cert.PrivateKey = priv
	return cert, nil
}

func b64cert(cert tls.Certificate) string {
	var b bytes.Buffer
	_ = pem.Encode(&b, &pem.Block{Type: "CERTIFICATE", Bytes: cert.Certificate[0]})
	return base64.StdEncoding.EncodeToString(b.Bytes())
}

func generatePrivateKey(bits int) (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, bits)
}
