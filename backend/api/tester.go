package api

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/miekg/dns"
)

type TestQueryRequest struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Nameserver string `json:"nameserver"`
	Subdomain  string `json:"subdomain"`
}

type TestQueryResponse struct {
	Status         string   `json:"status"`
	RCode          string   `json:"rcode"`
	Question       string   `json:"question"`
	Answers        []string `json:"answers"`
	Authority      []string `json:"authority"`
	ResponseTimeMs int64    `json:"response_time_ms"`
	Server         string   `json:"server"`
	Raw            string   `json:"raw"`
}

func (h *APIHandler) handleTestQuery(w http.ResponseWriter, r *http.Request) {
	h.setCorsHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	var req TestQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid request JSON")
		return
	}

	subdomain := req.Subdomain
	if subdomain == "" {
		subdomain = h.getSubdomain(r)
	}
	if subdomain == "" {
		subdomain = "default"
	}

	qName := strings.TrimSpace(req.Name)
	if qName == "" {
		writeJSONError(w, http.StatusBadRequest, "Query name is required")
		return
	}

	if !strings.HasSuffix(qName, ".") {
		qName += "."
	}

	qTypeStr := strings.ToUpper(strings.TrimSpace(req.Type))
	if qTypeStr == "" {
		qTypeStr = "A"
	}
	qType, ok := dns.StringToType[qTypeStr]
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "Invalid query type: "+qTypeStr)
		return
	}

	msg := new(dns.Msg)
	msg.SetQuestion(qName, qType)
	msg.RecursionDesired = true

	// If querying ZeroCentDNS directly (no custom nameserver specified)
	cleanQ := strings.TrimSuffix(strings.ToLower(qName), ".")
	cleanBase := strings.ToLower(h.cfg.BaseDomain)
	isInternal := strings.HasSuffix(cleanQ, cleanBase)

	if req.Nameserver == "" && h.parental != nil && !isInternal {
		// External domain query: process directly through parental control engine with the user's subdomain policy
		clientIP := "127.0.0.1"
		if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
			clientIP = host
		}

		startTime := time.Now()
		resp, err := h.parental.ProcessQuery(subdomain, clientIP, msg)
		elapsedMs := time.Since(startTime).Milliseconds()

		if err != nil || resp == nil {
			writeJSON(w, http.StatusOK, TestQueryResponse{
				Status:         "ERROR",
				RCode:          "SERVFAIL",
				Question:       fmt.Sprintf("%s %s", qTypeStr, qName),
				Answers:        []string{},
				Authority:      []string{},
				ResponseTimeMs: elapsedMs,
				Server:         fmt.Sprintf("ZeroCentDNS Parental (%s)", subdomain),
				Raw:            fmt.Sprintf("Parental query failed: %v", err),
			})
			return
		}

		answers := make([]string, 0)
		for _, rr := range resp.Answer {
			answers = append(answers, rr.String())
		}
		authority := make([]string, 0)
		for _, rr := range resp.Ns {
			authority = append(authority, rr.String())
		}

		writeJSON(w, http.StatusOK, TestQueryResponse{
			Status:         "SUCCESS",
			RCode:          dns.RcodeToString[resp.Rcode],
			Question:       fmt.Sprintf("%s %s", qTypeStr, qName),
			Answers:        answers,
			Authority:      authority,
			ResponseTimeMs: elapsedMs,
			Server:         fmt.Sprintf("ZeroCentDNS Parental (%s)", subdomain),
			Raw:            resp.String(),
		})
		return
	}

	displayServer := req.Nameserver
	if displayServer == "" {
		displayServer = fmt.Sprintf("ns1.%s:53", h.cfg.BaseDomain)
	}

	serverAddr := ""
	if req.Nameserver != "" {
		serverAddr = req.Nameserver
		if !strings.Contains(serverAddr, ":") {
			serverAddr += ":53"
		}
	} else if len(h.cfg.DNSAddrs) > 0 {
		serverAddr = h.cfg.DNSAddrs[0]
	} else {
		serverAddr = fmt.Sprintf("127.0.0.1:%d", h.cfg.DNSPort)
	}

	client := &dns.Client{
		Net:     "udp",
		Timeout: 2 * time.Second,
	}

	startTime := time.Now()
	resp, rtt, err := client.Exchange(msg, serverAddr)
	elapsedMs := time.Since(startTime).Milliseconds()
	if rtt > 0 {
		elapsedMs = rtt.Milliseconds()
	}

	if err != nil {
		writeJSON(w, http.StatusOK, TestQueryResponse{
			Status:         "ERROR",
			RCode:          "SERVFAIL",
			Question:       fmt.Sprintf("%s %s", qTypeStr, qName),
			Answers:        []string{},
			Authority:      []string{},
			ResponseTimeMs: elapsedMs,
			Server:         displayServer,
			Raw:            fmt.Sprintf("DNS Query Failed: %s", err.Error()),
		})
		return
	}

	answers := make([]string, 0)
	for _, rr := range resp.Answer {
		answers = append(answers, rr.String())
	}

	authority := make([]string, 0)
	for _, rr := range resp.Ns {
		authority = append(authority, rr.String())
	}

	rcodeStr := dns.RcodeToString[resp.Rcode]

	writeJSON(w, http.StatusOK, TestQueryResponse{
		Status:         "SUCCESS",
		RCode:          rcodeStr,
		Question:       fmt.Sprintf("%s %s", qTypeStr, qName),
		Answers:        answers,
		Authority:      authority,
		ResponseTimeMs: elapsedMs,
		Server:         displayServer,
		Raw:            resp.String(),
	})
}
