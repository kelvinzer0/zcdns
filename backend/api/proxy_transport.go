package api

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

type tlsForwardDialer struct {
	tlsConfig *tls.Config
}

func (d *tlsForwardDialer) Dial(network, addr string) (net.Conn, error) {
	return d.DialContext(context.Background(), network, addr)
}

func (d *tlsForwardDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	var netDialer net.Dialer
	rawConn, err := netDialer.DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}

	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}

	cfg := d.tlsConfig
	if cfg == nil {
		cfg = &tls.Config{}
	} else {
		cfg = cfg.Clone()
	}
	if cfg.ServerName == "" {
		cfg.ServerName = host
	}

	tlsConn := tls.Client(rawConn, cfg)
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		_ = rawConn.Close()
		return nil, fmt.Errorf("tls handshake with proxy '%s' failed: %w", addr, err)
	}

	return tlsConn, nil
}

// createHTTPTransport creates an http.Transport configured with an optional SOCKS5, SOCKS5-TLS, or HTTP proxy.
func createHTTPTransport(proxyAddr string) (*http.Transport, error) {
	proxyAddr = strings.TrimSpace(proxyAddr)
	if proxyAddr == "" {
		return &http.Transport{
			DisableCompression: true,
		}, nil
	}

	raw := proxyAddr
	if !strings.Contains(raw, "://") {
		raw = "socks5://" + raw
	}

	parsedURL, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy URL '%s': %w", proxyAddr, err)
	}

	scheme := strings.ToLower(parsedURL.Scheme)
	switch scheme {
	case "socks5", "socks5h":
		var auth *proxy.Auth
		if parsedURL.User != nil {
			auth = &proxy.Auth{
				User: parsedURL.User.Username(),
			}
			if pass, ok := parsedURL.User.Password(); ok {
				auth.Password = pass
			}
		}
		dialer, err := proxy.SOCKS5("tcp", parsedURL.Host, auth, proxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize SOCKS5 dialer for '%s': %w", proxyAddr, err)
		}

		return &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				if cd, ok := dialer.(proxy.ContextDialer); ok {
					return cd.DialContext(ctx, network, addr)
				}
				return dialer.Dial(network, addr)
			},
			DisableCompression: true,
		}, nil

	case "socks5tls", "socks5+tls", "socks5s", "tls+socks5":
		var auth *proxy.Auth
		if parsedURL.User != nil {
			auth = &proxy.Auth{
				User: parsedURL.User.Username(),
			}
			if pass, ok := parsedURL.User.Password(); ok {
				auth.Password = pass
			}
		}

		// Parse TLS parameters from query if specified (e.g. ?insecure=true, ?skip_verify=1)
		q := parsedURL.Query()
		insecure := q.Get("insecure") == "true" || q.Get("insecure") == "1" ||
			q.Get("skip_verify") == "true" || q.Get("skip_verify") == "1" ||
			q.Get("tls_insecure") == "true" || q.Get("tls_insecure") == "1"

		host, _, hErr := net.SplitHostPort(parsedURL.Host)
		if hErr != nil {
			host = parsedURL.Host
		}
		tlsCfg := &tls.Config{
			ServerName:         host,
			InsecureSkipVerify: insecure,
		}

		tlsDialer := &tlsForwardDialer{tlsConfig: tlsCfg}
		dialer, err := proxy.SOCKS5("tcp", parsedURL.Host, auth, tlsDialer)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize SOCKS5-TLS dialer for '%s': %w", proxyAddr, err)
		}

		return &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				if cd, ok := dialer.(proxy.ContextDialer); ok {
					return cd.DialContext(ctx, network, addr)
				}
				return dialer.Dial(network, addr)
			},
			DisableCompression: true,
		}, nil

	case "http", "https":
		return &http.Transport{
			Proxy:              http.ProxyURL(parsedURL),
			DisableCompression: true,
		}, nil

	default:
		return nil, fmt.Errorf("unsupported proxy scheme '%s' in '%s' (use socks5://, socks5tls://, or http://)", scheme, proxyAddr)
	}
}

// createProxyHTTPClient returns an http.Client with the given timeout and optional SOCKS5/SOCKS5-TLS/HTTP proxy.
func createProxyHTTPClient(proxyAddr string, timeout time.Duration) (*http.Client, error) {
	transport, err := createHTTPTransport(proxyAddr)
	if err != nil {
		return nil, err
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}, nil
}
