package app

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"strings"
	"sync"

	"golang.org/x/crypto/acme/autocert"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
)

// TLSConfig returns a TLS configuration that works with LetsEncrypt automatic SSL cert issuer
// as well as any provided certificate files from providers.
//
// The certs are provided in the form of paths where .pem and .key files exist
func TLSConfig(m *autocert.Manager, certs ...string) (tc *tls.Config) {
	certMap := make(map[string]*tls.Certificate)
	var mx sync.Mutex

	for _, certPath := range certs {
		if certPath == "" {
			continue
		}

		var err error
		var c tls.Certificate

		// Load certificate and key files
		if c, err = tls.LoadX509KeyPair(
			certPath+".pem", certPath+".key",
		); chk.E(err) {
			log.E.F("failed to load certificate from %s: %v", certPath, err)
			continue
		}

		// Extract domain names from certificate
		if len(c.Certificate) > 0 {
			if x509Cert, err := x509.ParseCertificate(c.Certificate[0]); err == nil {
				// Use the common name as the primary domain
				if x509Cert.Subject.CommonName != "" {
					certMap[x509Cert.Subject.CommonName] = &c
					log.I.F("loaded certificate for domain: %s", x509Cert.Subject.CommonName)
				}
				// Also add any subject alternative names
				for _, san := range x509Cert.DNSNames {
					if san != "" {
						certMap[san] = &c
						log.I.F("loaded certificate for SAN domain: %s", san)
					}
				}
			}
		}
	}

	if m == nil {
		// Create a basic TLS config without autocert
		tc = &tls.Config{
			GetCertificate: func(helo *tls.ClientHelloInfo) (*tls.Certificate, error) {
				mx.Lock()
				defer mx.Unlock()

				// Check for exact match first
				if cert, exists := certMap[helo.ServerName]; exists {
					return cert, nil
				}

				// Check for wildcard matches
				for domain, cert := range certMap {
					if strings.HasPrefix(domain, "*.") {
						baseDomain := domain[2:] // Remove "*."
						if strings.HasSuffix(helo.ServerName, baseDomain) {
							return cert, nil
						}
					}
				}

				return nil, fmt.Errorf("no certificate found for %s", helo.ServerName)
			},
		}
	} else {
		tc = m.TLSConfig()
		tc.GetCertificate = func(helo *tls.ClientHelloInfo) (*tls.Certificate, error) {
			mx.Lock()

			// Check for exact match first
			if cert, exists := certMap[helo.ServerName]; exists {
				mx.Unlock()
				return cert, nil
			}

			// Check for wildcard matches
			for domain, cert := range certMap {
				if strings.HasPrefix(domain, "*.") {
					baseDomain := domain[2:] // Remove "*."
					if strings.HasSuffix(helo.ServerName, baseDomain) {
						mx.Unlock()
						return cert, nil
					}
				}
			}

			mx.Unlock()

			// Fall back to autocert for domains not in our certificate map
			return m.GetCertificate(helo)
		}
	}

	return tc
}

// ValidateTLSConfig checks if the TLS configuration is valid
func ValidateTLSConfig(domains []string, certs []string) (err error) {
	if len(domains) == 0 {
		return fmt.Errorf("no TLS domains specified")
	}

	// Validate domain names
	for _, domain := range domains {
		if domain == "" {
			continue
		}
		if strings.Contains(domain, " ") || strings.Contains(domain, "\t") {
			return fmt.Errorf("invalid domain name: %s", domain)
		}
	}

	return nil
}
