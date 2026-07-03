package ssl

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"testing"
	"time"
)

func generateTestCertificate(notBefore, notAfter time.Time) (tls.Certificate, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, err
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return tls.Certificate{}, err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Test Co"},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})

	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return tls.Certificate{}, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})

	return tls.X509KeyPair(certPEM, keyPEM)
}

func startLocalTLSServer(cert tls.Certificate) (string, func(), error) {
	config := &tls.Config{Certificates: []tls.Certificate{cert}}
	ln, err := tls.Listen("tcp", "127.0.0.1:0", config)
	if err != nil {
		return "", nil, err
	}

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				tlsConn, ok := c.(*tls.Conn)
				if !ok {
					return
				}
				if err := tlsConn.Handshake(); err != nil {
					return
				}
				_, _ = tlsConn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n"))
			}(conn)
		}
	}()

	url := "https://" + ln.Addr().String()
	cleanup := func() {
		_ = ln.Close()
	}
	return url, cleanup, nil
}

func TestCheck(t *testing.T) {
	// Enable InsecureSkipVerify for tests using self-signed certificates
	oldTLSConfig := TLSConfig
	TLSConfig = &tls.Config{
		InsecureSkipVerify: true,
	}
	defer func() {
		TLSConfig = oldTLSConfig
	}()

	// 1. Test Invalid URLs
	t.Run("InvalidURL", func(t *testing.T) {
		info := Check("invalid-url-string")
		if info.Status != "unavailable" || info.Error == "" {
			t.Errorf("expected unavailable and error, got: %+v", info)
		}
	})

	t.Run("NonHTTPS", func(t *testing.T) {
		info := Check("http://google.com")
		if info.Status != "unavailable" || info.Error != "not HTTPS" {
			t.Errorf("expected unavailable and not HTTPS error, got: %+v", info)
		}
	})

	t.Run("MissingHost", func(t *testing.T) {
		info := Check("https://")
		if info.Status != "unavailable" || info.Error == "" {
			t.Errorf("expected unavailable, got: %+v", info)
		}
	})

	// 2. Test Valid SSL certificate
	t.Run("ValidCert", func(t *testing.T) {
		now := time.Now()
		cert, err := generateTestCertificate(now.Add(-24*time.Hour), now.Add(100*24*time.Hour))
		if err != nil {
			t.Fatalf("failed to generate cert: %v", err)
		}

		url, cleanup, err := startLocalTLSServer(cert)
		if err != nil {
			t.Fatalf("failed to start server: %v", err)
		}
		defer cleanup()

		info := Check(url)
		if !info.Valid {
			t.Errorf("expected certificate to be valid, got: %+v", info)
		}
		if info.Status != "valid" {
			t.Errorf("expected status 'valid', got: %s", info.Status)
		}
		if info.DaysRemaining < 98 || info.DaysRemaining > 100 {
			t.Errorf("expected approx 99 days remaining, got: %d", info.DaysRemaining)
		}
	})

	// 3. Test Expired SSL certificate
	t.Run("ExpiredCert", func(t *testing.T) {
		now := time.Now()
		cert, err := generateTestCertificate(now.Add(-10*24*time.Hour), now.Add(-1*24*time.Hour))
		if err != nil {
			t.Fatalf("failed to generate cert: %v", err)
		}

		url, cleanup, err := startLocalTLSServer(cert)
		if err != nil {
			t.Fatalf("failed to start server: %v", err)
		}
		defer cleanup()

		info := Check(url)
		if info.Valid {
			t.Errorf("expected certificate to be invalid, got: %+v", info)
		}
		if info.Status != "expired" {
			t.Errorf("expected status 'expired', got: %s", info.Status)
		}
	})

	// 4. Test Warning SSL certificate (less than 30 days remaining)
	t.Run("WarningCert", func(t *testing.T) {
		now := time.Now()
		cert, err := generateTestCertificate(now.Add(-24*time.Hour), now.Add(15*24*time.Hour))
		if err != nil {
			t.Fatalf("failed to generate cert: %v", err)
		}

		url, cleanup, err := startLocalTLSServer(cert)
		if err != nil {
			t.Fatalf("failed to start server: %v", err)
		}
		defer cleanup()

		info := Check(url)
		if !info.Valid {
			t.Errorf("expected certificate to be valid, got: %+v", info)
		}
		if info.Status != "warning" {
			t.Errorf("expected status 'warning', got: %s", info.Status)
		}
		if info.DaysRemaining < 13 || info.DaysRemaining > 15 {
			t.Errorf("expected approx 14 days remaining, got: %d", info.DaysRemaining)
		}
	})

	// 5. Test Dial Failure
	t.Run("DialFailure", func(t *testing.T) {
		// Attempt checking a closed local port
		info := Check("https://127.0.0.1:59999")
		if info.Valid {
			t.Errorf("expected invalid, got: %+v", info)
		}
		if info.Status != "unavailable" {
			t.Errorf("expected status 'unavailable', got: %s", info.Status)
		}
	})
}
