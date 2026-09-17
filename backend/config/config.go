package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	HTTPPort      string
	DNSPort       int
	DNSAddrs      []string
	BaseDomain    string
	DBPath        string
	StaticDir     string
	CleanInterval int // in minutes
	RecordTTLMax  int
	AdminKey      string

	// DoT (DNS-over-TLS) — used for *.guard.<BaseDomain> Private DNS on Android
	DoTPort     int
	TLSCertFile string
	TLSKeyFile  string
}

func LoadConfig() *Config {
	httpPort := getEnv("PORT", "8085")
	if !strings.Contains(httpPort, ":") {
		httpPort = "127.0.0.1:" + httpPort
	}

	var dnsAddrs []string
	if rawAddrs := os.Getenv("DNS_ADDRS"); rawAddrs != "" {
		for _, addr := range strings.Split(rawAddrs, ",") {
			trimmed := strings.TrimSpace(addr)
			if trimmed != "" {
				dnsAddrs = append(dnsAddrs, trimmed)
			}
		}
	}

	dnsPortStr := getEnv("DNS_PORT", "53")
	dnsPort, err := strconv.Atoi(dnsPortStr)
	if err != nil {
		dnsPort = 53
	}
	if len(dnsAddrs) > 0 {
		_, p, err := net.SplitHostPort(dnsAddrs[0])
		if err == nil {
			if parsedP, err := strconv.Atoi(p); err == nil {
				dnsPort = parsedP
			}
		}
	}

	if len(dnsAddrs) == 0 {
		dnsAddrs = []string{fmt.Sprintf(":%d", dnsPort)}
	}

	baseDomain := getEnv("BASE_DOMAIN", "zcdns.id")
	dbPath := getEnv("DB_PATH", "./zcdns.sqlite")
	staticDir := getEnv("STATIC_DIR", "")

	dotPortStr := getEnv("DOT_PORT", "853")
	dotPort, err := strconv.Atoi(dotPortStr)
	if err != nil {
		dotPort = 853
	}

	return &Config{
		HTTPPort:      httpPort,
		DNSPort:       dnsPort,
		DNSAddrs:      dnsAddrs,
		BaseDomain:    baseDomain,
		DBPath:        dbPath,
		StaticDir:     staticDir,
		CleanInterval: 60,
		RecordTTLMax:  86400,
		AdminKey:      getEnv("ADMIN_KEY", "@Kelvin123"),
		DoTPort:       dotPort,
		TLSCertFile:   getEnv("TLS_CERT_FILE", "/etc/letsencrypt/live/guard.zcdns.id/fullchain.pem"),
		TLSKeyFile:    getEnv("TLS_KEY_FILE", "/etc/letsencrypt/live/guard.zcdns.id/privkey.pem"),
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
