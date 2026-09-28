package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// ensureTLSCert loads a persisted self-signed cert/key pair from varDir,
// generating one on first run. DSM forces HTTPS for its own UI, so the
// AppWindow's iframe (see opkgui.js) needs oPKGui to be reachable over
// HTTPS too, or the browser mixed-content-blocks it. The cert is
// self-signed (there is no way to get a browser-trusted one for an
// arbitrary LAN IP) -- the browser will need a one-time manual "proceed
// anyway" the first time it's reached directly, same as any self-signed
// local service.
func ensureTLSCert(varDir string) (tls.Certificate, error) {
	certPath := filepath.Join(varDir, "opkgui-cert.pem")
	keyPath := filepath.Join(varDir, "opkgui-key.pem")

	if cert, err := tls.LoadX509KeyPair(certPath, keyPath); err == nil {
		return cert, nil
	}

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}

	template := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "oPKGui"},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// Cover every local IP so the cert matches whatever LAN address the
	// browser actually used to reach DSM.
	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, a := range addrs {
			if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				template.IPAddresses = append(template.IPAddresses, ipnet.IP)
			}
		}
	}
	if hostname, err := os.Hostname(); err == nil && hostname != "" {
		template.DNSNames = append(template.DNSNames, hostname)
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, err
	}

	certOut, err := os.OpenFile(certPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return tls.Certificate{}, err
	}
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	certOut.Close()

	keyBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return tls.Certificate{}, err
	}
	keyOut, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return tls.Certificate{}, err
	}
	pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})
	keyOut.Close()

	return tls.LoadX509KeyPair(certPath, keyPath)
}
