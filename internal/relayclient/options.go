package relayclient

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/url"
	"os"
	"strings"
)

type ClientOptions struct {
	CAPath string
}

func TLSConfigForURL(rawURL string, options ClientOptions) (*tls.Config, error) {
	if strings.TrimSpace(options.CAPath) == "" {
		return nil, nil
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse relay URL: %w", err)
	}
	if parsed.Scheme != "wss" && parsed.Scheme != "https" {
		return nil, nil
	}
	pemBytes, err := os.ReadFile(options.CAPath)
	if err != nil {
		return nil, fmt.Errorf("read relay CA file: %w", err)
	}
	pool, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("load system cert pool: %w", err)
	}
	if pool == nil {
		pool = x509.NewCertPool()
	}
	if !pool.AppendCertsFromPEM(pemBytes) {
		return nil, fmt.Errorf("append relay CA file: no PEM certificates found")
	}
	return &tls.Config{RootCAs: pool}, nil
}
