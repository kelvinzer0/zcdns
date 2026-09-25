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

func loadEnvFiles(paths ...string) {
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				k := strings.TrimSpace(parts[0])
				v := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
				if os.Getenv(k) == "" {
					_ = os.Setenv(k, v)
				}
			}
		}
	}
}

func LoadConfig() *Config {
	loadEnvFiles("/opt/zcdns/.env", ".env")

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
		// Auto-detect shared-ip dummy interfaces (10.0.0.10)
		hasSharedIP := false
		if ifAddrs, err := net.InterfaceAddrs(); err == nil {
			for _, a := range ifAddrs {
				if ipNet, ok := a.(*net.IPNet); ok {
					if ipNet.IP.String() == "10.0.0.10" {
						hasSharedIP = true
						break
					}
				}
			}
		}

		if hasSharedIP {
			dnsAddrs = []string{
				fmt.Sprintf("10.0.0.10:%d", dnsPort),
				fmt.Sprintf("10.0.0.11:%d", dnsPort),
				fmt.Sprintf("[fd00::10]:%d", dnsPort),
				fmt.Sprintf("[fd00::11]:%d", dnsPort),
				fmt.Sprintf("127.0.0.1:%d", dnsPort),
			}
		} else {
			dnsAddrs = []string{fmt.Sprintf(":%d", dnsPort)}
		}
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
