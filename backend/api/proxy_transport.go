package api

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

// createHTTPTransport creates an http.Transport configured with an optional SOCKS5 or HTTP proxy.
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

	case "http", "https":
		return &http.Transport{
			Proxy:              http.ProxyURL(parsedURL),
			DisableCompression: true,
		}, nil

	default:
		return nil, fmt.Errorf("unsupported proxy scheme '%s' in '%s' (use socks5:// or http://)", scheme, proxyAddr)
	}
}

// createProxyHTTPClient returns an http.Client with the given timeout and optional SOCKS5/HTTP proxy.
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
