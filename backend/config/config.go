package config

import (
	"fmt"
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
}

func LoadConfig() *Config {
	httpPort := getEnv("PORT", "8085")
	if !strings.Contains(httpPort, ":") {
		httpPort = "127.0.0.1:" + httpPort
	}

	dnsPortStr := getEnv("DNS_PORT", "5354")
	dnsPort, err := strconv.Atoi(dnsPortStr)
	if err != nil {
		dnsPort = 5354
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
	if len(dnsAddrs) == 0 {
		dnsAddrs = []string{fmt.Sprintf(":%d", dnsPort)}
	}

	baseDomain := getEnv("BASE_DOMAIN", "zcdns.id")
	dbPath := getEnv("DB_PATH", "./zcdns.sqlite")
	staticDir := getEnv("STATIC_DIR", "")

	return &Config{
		HTTPPort:      httpPort,
		DNSPort:       dnsPort,
		DNSAddrs:      dnsAddrs,
		BaseDomain:    baseDomain,
		DBPath:        dbPath,
		StaticDir:     staticDir,
		CleanInterval: 60,
		RecordTTLMax:  86400,
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
