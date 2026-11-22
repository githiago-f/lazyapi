package model

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"
)

// DefaultMaxBody caps response bodies read into memory.
const DefaultMaxBody int64 = 50 << 20

// HTTPOptions configure the HTTP client used for requests. The zero value
// means: verify TLS, system proxy settings, follow up to 10 redirects.
type HTTPOptions struct {
	// Insecure skips TLS certificate verification.
	Insecure bool `yaml:"insecure,omitempty"`
	// CACert is a PEM file with extra trusted CAs.
	CACert string `yaml:"caCert,omitempty"`
	// ClientCert / ClientKey are PEM files for mutual TLS.
	ClientCert string `yaml:"clientCert,omitempty"`
	ClientKey  string `yaml:"clientKey,omitempty"`
	// Proxy is an http(s):// or socks5:// URL; "" uses HTTP(S)_PROXY.
	Proxy string `yaml:"proxy,omitempty"`
	// NoRedirects returns 3xx responses instead of following them.
	NoRedirects bool `yaml:"noRedirects,omitempty"`
	// MaxRedirects bounds redirect chains (default 10).
	MaxRedirects int `yaml:"maxRedirects,omitempty"`
}

// IsZero reports whether the options are all defaults.
func (o HTTPOptions) IsZero() bool { return o == HTTPOptions{} }

// Merge returns o with the non-zero fields of override applied.
func (o HTTPOptions) Merge(override HTTPOptions) HTTPOptions {
	if override.Insecure {
		o.Insecure = true
	}
	if override.CACert != "" {
		o.CACert = override.CACert
	}
	if override.ClientCert != "" {
		o.ClientCert = override.ClientCert
	}
	if override.ClientKey != "" {
		o.ClientKey = override.ClientKey
	}
	if override.Proxy != "" {
		o.Proxy = override.Proxy
	}
	if override.NoRedirects {
		o.NoRedirects = true
	}
	if override.MaxRedirects != 0 {
		o.MaxRedirects = override.MaxRedirects
	}
	return o
}

// Client builds an *http.Client for the options. It returns nil (use the
// default client) when every option is at its default.
func (o HTTPOptions) Client() (*http.Client, error) {
	if o.IsZero() {
		return nil, nil
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()

	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if o.Insecure {
		tlsConfig.InsecureSkipVerify = true //nolint:gosec // explicitly requested with --insecure
	}
	if o.CACert != "" {
		pem, err := os.ReadFile(o.CACert)
		if err != nil {
			return nil, fmt.Errorf("reading CA certificate: %w", err)
		}
		pool, err := x509.SystemCertPool()
		if err != nil || pool == nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("%s contains no PEM certificates", o.CACert)
		}
		tlsConfig.RootCAs = pool
	}
	if o.ClientCert != "" || o.ClientKey != "" {
		if o.ClientCert == "" || o.ClientKey == "" {
			return nil, errors.New("client certificate and key must be given together")
		}
		cert, err := tls.LoadX509KeyPair(o.ClientCert, o.ClientKey)
		if err != nil {
			return nil, fmt.Errorf("loading client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}
	transport.TLSClientConfig = tlsConfig

	if o.Proxy != "" {
		proxyURL, err := url.Parse(o.Proxy)
		if err != nil || proxyURL.Host == "" {
			return nil, fmt.Errorf("invalid proxy URL %q", o.Proxy)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}

	client := &http.Client{Transport: transport}
	switch {
	case o.NoRedirects:
		client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	case o.MaxRedirects > 0:
		limit := o.MaxRedirects
		client.CheckRedirect = func(_ *http.Request, via []*http.Request) error {
			if len(via) >= limit {
				return fmt.Errorf("stopped after %d redirects", limit)
			}
			return nil
		}
	}
	return client, nil
}

// Timing breaks down where a request spent its time (zero when unknown,
// e.g. reused connections have no DNS/connect/TLS phase).
type Timing struct {
	DNS     time.Duration `json:"dns"`
	Connect time.Duration `json:"connect"`
	TLS     time.Duration `json:"tls"`
	// TTFB is from sending the request until the first response byte.
	TTFB  time.Duration `json:"ttfb"`
	Total time.Duration `json:"total"`
	// Reused is true when an idle keep-alive connection was used.
	Reused bool `json:"reused"`
}
