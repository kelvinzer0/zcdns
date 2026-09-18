package dns

import (
	"encoding/json"
	"net"
	"strconv"
	"strings"
	"time"
	"zcdns-backend/db"

	"github.com/google/uuid"
	"github.com/miekg/dns"
)

func (s *Server) AddAcmeChallenge(subdomain, value string) {
	s.mu.Lock()
	if s.acmeChallenges == nil {
		s.acmeChallenges = make(map[string][]string)
	}
	subdomain = strings.ToLower(strings.TrimSpace(subdomain))
	value = strings.Trim(strings.TrimSpace(value), "\"")
	found := false
	for _, v := range s.acmeChallenges[subdomain] {
		if v == value {
			found = true
			break
		}
	}
	if !found {
		s.acmeChallenges[subdomain] = append(s.acmeChallenges[subdomain], value)
		s.zoneSerial++
	}
	s.mu.Unlock()
	if !found {
		s.NotifySlaves()
	}
}

func (s *Server) GetAcmeChallenges(subdomain string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.acmeChallenges == nil {
		return nil
	}
	subdomain = strings.ToLower(strings.TrimSpace(subdomain))
	result := make([]string, len(s.acmeChallenges[subdomain]))
	copy(result, s.acmeChallenges[subdomain])
	return result
}

func (s *Server) parseDomain(qName string) (subdomain string, recordName string, ok bool) {
	cleanQ := strings.TrimSuffix(qName, ".")
	cleanBase := strings.ToLower(s.cfg.BaseDomain)

	if !strings.HasSuffix(cleanQ, cleanBase) {
		return "", "", false
	}

	prefix := strings.TrimSuffix(cleanQ, cleanBase)
	prefix = strings.TrimSuffix(prefix, ".")
	if prefix == "" {
		return "", "", false
	}

	parts := strings.Split(prefix, ".")
	// The last component of prefix is the subdomain
	// e.g. "foo" -> subdomain="foo", recordName="@"
	// e.g. "blog.foo" -> subdomain="foo", recordName="blog"
	// e.g. "a.b.foo" -> subdomain="foo", recordName="a.b"
	subdomain = parts[len(parts)-1]
	if len(parts) == 1 {
		recordName = "@"
	} else {
		recordName = strings.Join(parts[:len(parts)-1], ".")
	}

	return subdomain, recordName, true
}

func (s *Server) buildRR(r db.Record, fqdn string) dns.RR {
	hdr := dns.RR_Header{
		Name:   fqdn,
		Class:  dns.ClassINET,
		Ttl:    uint32(r.TTL),
		Rrtype: dns.StringToType[r.Type],
	}

	switch r.Type {
	case "A":
		ip := net.ParseIP(r.Value)
		if ip == nil {
			return nil
		}
		return &dns.A{Hdr: hdr, A: ip.To4()}

	case "AAAA":
		ip := net.ParseIP(r.Value)
		if ip == nil {
			return nil
		}
		return &dns.AAAA{Hdr: hdr, AAAA: ip.To16()}

	case "CNAME":
		return &dns.CNAME{Hdr: hdr, Target: dns.Fqdn(r.Value)}

	case "TXT":
		cleanVal := strings.Trim(r.Value, "\"")
		return &dns.TXT{Hdr: hdr, Txt: []string{cleanVal}}

	case "MX":
		parts := strings.Fields(r.Value)
		pref := uint16(10)
		host := r.Value
		if len(parts) >= 2 {
			if p, err := strconv.Atoi(parts[0]); err == nil {
				pref = uint16(p)
			}
			host = parts[1]
		}
		return &dns.MX{Hdr: hdr, Preference: pref, Mx: dns.Fqdn(host)}

	case "NS":
		return &dns.NS{Hdr: hdr, Ns: dns.Fqdn(r.Value)}

	case "PTR":
		return &dns.PTR{Hdr: hdr, Ptr: dns.Fqdn(r.Value)}

	case "CAA":
		// Format: "0 issue letsencrypt.org"
		parts := strings.Fields(r.Value)
		flag := uint8(0)
		tag := "issue"
		val := "letsencrypt.org"
		if len(parts) >= 3 {
			if f, err := strconv.Atoi(parts[0]); err == nil {
				flag = uint8(f)
			}
			tag = parts[1]
			val = strings.Join(parts[2:], " ")
		}
		return &dns.CAA{Hdr: hdr, Flag: flag, Tag: tag, Value: val}

	case "SRV":
		// Format: "priority weight port target" e.g. "10 60 5060 bigbox.example.com."
		parts := strings.Fields(r.Value)
		if len(parts) >= 4 {
			prio, _ := strconv.Atoi(parts[0])
			weight, _ := strconv.Atoi(parts[1])
			port, _ := strconv.Atoi(parts[2])
			return &dns.SRV{
				Hdr:      hdr,
				Priority: uint16(prio),
				Weight:   uint16(weight),
				Port:     uint16(port),
				Target:   dns.Fqdn(parts[3]),
			}
		}
	}

	return nil
}

func (s *Server) logAndBroadcast(subdomain, qname, qtype, clientIP, rcode string, answers []string) {
	reqID := uuid.New().String()
	reqLog := &db.RequestLog{
		ID:        reqID,
		Subdomain: subdomain,
		QName:     qname,
		QType:     qtype,
		ClientIP:  clientIP,
		RCode:     rcode,
		Answers:   answers,
		CreatedAt: time.Now(),
	}

	ansJSON, _ := json.Marshal(answers)
	_ = s.db.LogRequest(reqLog, string(ansJSON))

	if s.broadcaster != nil {
		s.broadcaster.BroadcastRequest(subdomain, reqLog)
	}
}
