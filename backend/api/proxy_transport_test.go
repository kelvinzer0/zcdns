package api

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"zcdns-backend/db"
)

func TestProxyTransport_Config(t *testing.T) {
	// 1. Empty proxy: direct transport
	trDirect, err := createHTTPTransport("")
	if err != nil {
		t.Fatalf("unexpected error for empty proxy: %v", err)
	}
	if !trDirect.DisableCompression {
		t.Errorf("expected DisableCompression to be true")
	}
	if trDirect.DialContext != nil {
		t.Errorf("expected direct DialContext to be nil")
	}

	// 2. SOCKS5 address without scheme
	trSocks1, err := createHTTPTransport("127.0.0.1:1080")
	if err != nil {
		t.Fatalf("unexpected error for 127.0.0.1:1080: %v", err)
	}
	if trSocks1.DialContext == nil {
		t.Errorf("expected DialContext to be set for SOCKS5 dialer")
	}

	// 3. SOCKS5 address with auth
	trSocks2, err := createHTTPTransport("socks5://myuser:secret123@proxy.example.com:1080")
	if err != nil {
		t.Fatalf("unexpected error for socks5 with auth: %v", err)
	}
	if trSocks2.DialContext == nil {
		t.Errorf("expected DialContext to be set for SOCKS5 dialer")
	}

	// 4. HTTP proxy
	trHTTP, err := createHTTPTransport("http://10.0.0.1:8080")
	if err != nil {
		t.Fatalf("unexpected error for HTTP proxy: %v", err)
	}
	if trHTTP.Proxy == nil {
		t.Errorf("expected Proxy function to be set for HTTP proxy")
	}

	// 5. SOCKS5-TLS schemes
	tlsSchemes := []string{
		"socks5tls://proxy.example.com:443",
		"socks5+tls://user:pass@proxy.example.com:8443?insecure=true",
		"socks5s://proxy.example.com:443",
		"tls+socks5://proxy.example.com:443?skip_verify=1",
	}
	for _, raw := range tlsSchemes {
		tr, err := createHTTPTransport(raw)
		if err != nil {
			t.Fatalf("unexpected error for %s: %v", raw, err)
		}
		if tr.DialContext == nil {
			t.Errorf("expected DialContext to be set for %s", raw)
		}
	}

	// 6. Unsupported scheme
	_, err = createHTTPTransport("ftp://127.0.0.1:21")
	if err == nil {
		t.Fatalf("expected error for ftp scheme, got nil")
	}
}

func TestProxyTransport_Socks5TLS_Handshake(t *testing.T) {
	// Setup a TLS listener with self-signed certificate
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	addr := ts.Listener.Addr().String()

	// 1. Dial without insecure skip verify should fail because cert is self-signed
	dialerStrict := &tlsForwardDialer{
		tlsConfig: &tls.Config{
			ServerName: "example.com",
		},
	}
	_, err := dialerStrict.DialContext(context.Background(), "tcp", addr)
	if err == nil {
		t.Fatalf("expected TLS verification failure for self-signed cert without InsecureSkipVerify")
	}

	// 2. Dial with InsecureSkipVerify: true should succeed
	dialerInsecure := &tlsForwardDialer{
		tlsConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	conn, err := dialerInsecure.DialContext(context.Background(), "tcp", addr)
	if err != nil {
		t.Fatalf("expected successful TLS handshake with InsecureSkipVerify, got: %v", err)
	}
	defer conn.Close()

	// 3. Test through createHTTPTransport with ?insecure=true
	tr, err := createHTTPTransport("socks5tls://" + addr + "?insecure=true")
	if err != nil {
		t.Fatalf("createHTTPTransport failed: %v", err)
	}
	if tr.DialContext == nil {
		t.Fatalf("expected DialContext to be non-nil")
	}
}

func TestProxyTransport_ProviderConnectionSocks5(t *testing.T) {
	handler, database, cleanup := setupTestHandler(t)
	defer cleanup()

	subdomain := "socks-sub"

	// 1. Add connection with socks5_proxy
	connID, err := database.AddAIRouterConnection(&db.AIRouterConnection{
		Subdomain:   subdomain,
		Provider:    "openai",
		APIType:     "openai",
		Name:        "Proxied OpenAI",
		APIKey:      "sk-test-proxy-key",
		BaseURL:     "https://api.openai.com",
		Socks5Proxy: "socks5://127.0.0.1:59999",
		ModelsJSON:  `["gpt-4o"]`,
		Status:      "active",
	})
	if err != nil {
		t.Fatalf("AddAIRouterConnection failed: %v", err)
	}

	// 2. Retrieve and verify socks5_proxy is preserved
	conn, err := database.GetAIRouterConnectionByID(subdomain, connID)
	if err != nil {
		t.Fatalf("GetAIRouterConnectionByID failed: %v", err)
	}
	if conn.Socks5Proxy != "socks5://127.0.0.1:59999" {
		t.Errorf("expected socks5_proxy 'socks5://127.0.0.1:59999', got '%s'", conn.Socks5Proxy)
	}

	// 3. Verify GetAIRouterConnections includes socks5_proxy
	conns, err := database.GetAIRouterConnections(subdomain)
	if err != nil || len(conns) == 0 {
		t.Fatalf("GetAIRouterConnections failed: %v", err)
	}
	if conns[0].Socks5Proxy != "socks5://127.0.0.1:59999" {
		t.Errorf("expected socks5_proxy in list, got '%s'", conns[0].Socks5Proxy)
	}

	// 4. Test connection via testProviderConnection to an unreachable local socks5 proxy
	// Should fail gracefully with network error without crashing
	success, _, _, msg := testProviderConnection("openai", "openai", "sk-test", "https://api.openai.com", "socks5://127.0.0.1:59999")
	if success {
		t.Errorf("expected failure when connecting via nonexistent SOCKS5 proxy")
	}
	if msg == "" {
		t.Errorf("expected error message describing failure")
	}

	// 5. Test testProviderConnection with invalid proxy URL
	successInv, _, _, msgInv := testProviderConnection("openai", "openai", "sk-test", "https://api.openai.com", "invalid-scheme://::bad-url::")
	if successInv {
		t.Errorf("expected failure for invalid proxy URL")
	}
	if msgInv == "" {
		t.Errorf("expected error message for invalid proxy URL")
	}

	// 5b. Test testProviderConnection with unreachable socks5tls proxy
	successTls, _, _, msgTls := testProviderConnection("openai", "openai", "sk-test", "https://api.openai.com", "socks5tls://127.0.0.1:59998?insecure=true")
	if successTls {
		t.Errorf("expected failure when connecting via nonexistent SOCKS5-TLS proxy")
	}
	if msgTls == "" {
		t.Errorf("expected error message describing SOCKS5-TLS failure")
	}

	// 6. Test with a mock upstream server via direct (no proxy)
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data": [{"id": "gpt-4o-mock"}]}`))
	}))
	defer mockServer.Close()

	directSuccess, latency, models, directMsg := testProviderConnection("openai", "openai", "sk-valid", mockServer.URL, "")
	if !directSuccess {
		t.Fatalf("expected direct test to succeed, got: %s", directMsg)
	}
	if latency < 0 {
		t.Errorf("expected non-negative latency, got %d", latency)
	}
	if len(models) != 1 || models[0] != "gpt-4o-mock" {
		t.Errorf("expected model gpt-4o-mock, got %v", models)
	}

	_ = handler
	_ = context.Background()
	_ = time.Second
}
