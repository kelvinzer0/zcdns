package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/miekg/dns"
)

type TestQueryRequest struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Nameserver string `json:"nameserver"`
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

	serverAddr := fmt.Sprintf("127.0.0.1:%d", h.cfg.DNSPort)
	if req.Nameserver != "" {
		serverAddr = req.Nameserver
		if !strings.Contains(serverAddr, ":") {
			serverAddr += ":53"
		}
	}

	msg := new(dns.Msg)
	msg.SetQuestion(qName, qType)
	msg.RecursionDesired = true

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
			Server:         serverAddr,
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
		Server:         serverAddr,
		Raw:            resp.String(),
	})
}
