package api

import (
	"fmt"
	"net"
	"regexp"
	"strings"
)

var (
	hostnameRegex = regexp.MustCompile(`^(\*|@|[a-zA-Z0-9_]([a-zA-Z0-9-_]{0,61}[a-zA-Z0-9_])?(\.[a-zA-Z0-9_]([a-zA-Z0-9-_]{0,61}[a-zA-Z0-9_])?)*)$`)
)

func (h *APIHandler) validateRecord(req RecordRequest) (name string, recordType string, value string, ttl int, err error) {
	cleanType := strings.ToUpper(strings.TrimSpace(req.Type))
	validTypes := map[string]bool{
		"A": true, "AAAA": true, "CNAME": true, "TXT": true,
		"MX": true, "NS": true, "PTR": true, "CAA": true, "SRV": true,
	}
	if !validTypes[cleanType] {
		return "", "", "", 0, fmt.Errorf("unsupported record type: %s", cleanType)
	}

	cleanName := strings.TrimSpace(strings.ToLower(req.Name))
	cleanName = strings.TrimSuffix(cleanName, ".")
	if cleanName == "" {
		cleanName = "@"
	}
	
	if !hostnameRegex.MatchString(cleanName) {
		return "", "", "", 0, fmt.Errorf("invalid record name format")
	}

	cleanValue := strings.TrimSpace(req.Value)
	if cleanValue == "" {
		return "", "", "", 0, fmt.Errorf("record value cannot be empty")
	}

	switch cleanType {
	case "A":
		ip := net.ParseIP(cleanValue)
		if ip == nil || ip.To4() == nil {
			return "", "", "", 0, fmt.Errorf("value must be a valid IPv4 address")
		}
	case "AAAA":
		ip := net.ParseIP(cleanValue)
		if ip == nil || ip.To16() == nil {
			return "", "", "", 0, fmt.Errorf("value must be a valid IPv6 address")
		}
	case "CNAME", "NS", "PTR":
		if !hostnameRegex.MatchString(strings.TrimSuffix(strings.ToLower(cleanValue), ".")) {
			return "", "", "", 0, fmt.Errorf("value must be a valid domain name")
		}
	case "TXT":
		if !strings.HasPrefix(cleanValue, "\"") {
			cleanValue = fmt.Sprintf("\"%s\"", cleanValue)
		}
	}

	ttl = req.TTL
	if ttl <= 0 {
		ttl = 60
	}
	if ttl > 86400 {
		ttl = 86400
	}

	return cleanName, cleanType, cleanValue, ttl, nil
}
