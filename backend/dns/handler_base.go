package dns

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
)

func (s *Server) handleDoTRequest(w dns.ResponseWriter, r *dns.Msg) {
	remoteKey := ""
	if w.RemoteAddr() != nil {
		remoteKey = w.RemoteAddr().String()
	}

	// Retrieve SNI subdomain stored during TLS handshake
	subdomain := "default"
	if val, ok := s.sniMap.Load(remoteKey); ok {
		subdomain = val.(string)
	}
	// Clean up after the request to prevent sniMap from growing unbounded.
	// DoT connections are typically long-lived (TCP keepalive) but one entry
	// per remote addr is negligible; we clean on each request for safety.
	// defer s.sniMap.Delete(remoteKey) // Removed to allow DoT TCP pipelining

	clientIP := remoteKey
	if host, _, err := net.SplitHostPort(remoteKey); err == nil {
		clientIP = host
	}

	if len(r.Question) == 0 {
		m := new(dns.Msg)
		m.SetReply(r)
		_ = w.WriteMsg(m)
		return
	}

	q := r.Question[0]
	qName := strings.ToLower(q.Name)

	// Guard domain itself (*.guard.zcdns.id / guard.zcdns.id): return AAAA or NODATA.
	// This is what Android's connectivity check resolves before sending DNS queries.
	guardBase := "guard." + strings.ToLower(s.cfg.BaseDomain) + "."
	if qName == guardBase || strings.HasSuffix(qName, "."+guardBase) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.Authoritative = true
		s.handleGuardDomain(m, q, strings.TrimSuffix(qName, "."))
		_ = w.WriteMsg(m)
		return
	}

	// All other queries go through the parental engine for the SNI subdomain.
	if s.parental != nil {
		resp, err := s.parental.ProcessQuery(subdomain, clientIP, r)
		if err == nil && resp != nil {
			_ = w.WriteMsg(resp)
			return
		}
	}

	// Fallback: SERVFAIL
	m := new(dns.Msg)
	m.SetReply(r)
	m.SetRcode(r, dns.RcodeServerFailure)
	_ = w.WriteMsg(m)
}

func (s *Server) handleDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
	if r.Opcode == dns.OpcodeNotify {
		m := new(dns.Msg)
		m.SetReply(r)
		_ = w.WriteMsg(m)
		return
	}

	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative = true
	m.RecursionAvailable = false

	if len(r.Question) == 0 {
		_ = w.WriteMsg(m)
		return
	}

	q := r.Question[0]
	qName := strings.ToLower(q.Name)
	qTypeStr := dns.TypeToString[q.Qtype]

	clientIP := "remote"
	if w.RemoteAddr() != nil {
		if host, _, err := net.SplitHostPort(w.RemoteAddr().String()); err == nil {
			clientIP = host
		}
	}

	// Handle AXFR / IXFR Zone Transfer requests
	if q.Qtype == dns.TypeAXFR || q.Qtype == dns.TypeIXFR {
		s.handleAXFR(w, r)
		return
	}

	subdomain, recordName, isMatch := s.parseDomain(qName)

	if !isMatch {
		// If query is for base domain itself (e.g. zcdns.id)
		if strings.TrimSuffix(qName, ".") == strings.ToLower(s.cfg.BaseDomain) {
			s.handleBaseDomain(m, q)
			_ = w.WriteMsg(m)
			return
		}

		// If query is for an external domain, process through parental engine
		if s.parental != nil {
			resp, err := s.parental.ProcessQuery("default", clientIP, r)
			if err == nil && resp != nil {
				_ = w.WriteMsg(resp)
				return
			}
		}

		m.SetRcode(r, dns.RcodeNameError) // NXDOMAIN
		_ = w.WriteMsg(m)
		return
	}

	// Handle queries directly targeted at authoritative nameservers (ns1.zcdns.id / ns2.zcdns.id) or www
	if (subdomain == "ns1" || subdomain == "ns2" || subdomain == "www") && (recordName == "@" || recordName == "") {
		s.handleNameserverDomain(m, q, strings.TrimSuffix(qName, "."))
		_ = w.WriteMsg(m)
		return
	}

	// Handle queries directly targeted at parental control guard domains (*.guard.zcdns.id / guard.zcdns.id)
	// Guard domains are strictly IPv6 (AAAA only) and do not provide IPv4 A fallback.
	if subdomain == "guard" {
		// 1. Check ACME challenge TXT records for guard domain (_acme-challenge.guard.zcdns.id)
		if recordName == "_acme-challenge" && (q.Qtype == dns.TypeTXT || q.Qtype == dns.TypeANY) {
			m.SetRcode(r, dns.RcodeSuccess)
			seen := make(map[string]bool)
			for _, val := range s.GetAcmeChallenges("guard") {
				if !seen[val] {
					seen[val] = true
					m.Answer = append(m.Answer, &dns.TXT{
						Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: 60},
						Txt: []string{val},
					})
				}
			}
			if dbRecs, err := s.db.GetRecordsByNameAndType("guard", "_acme-challenge", "TXT"); err == nil {
				for _, rec := range dbRecs {
					val := strings.Trim(rec.Value, "\"")
					if !seen[val] {
						seen[val] = true
						m.Answer = append(m.Answer, &dns.TXT{
							Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: uint32(rec.TTL)},
							Txt: []string{val},
						})
					}
				}
			}
			_ = w.WriteMsg(m)
			return
		}

		// 2. Check other custom records in DB for guard
		if records, err := s.db.GetRecordsByNameAndType("guard", recordName, qTypeStr); err == nil && len(records) > 0 {
			m.SetRcode(r, dns.RcodeSuccess)
			for _, rec := range records {
				if rr := s.buildRR(rec, q.Name); rr != nil {
					m.Answer = append(m.Answer, rr)
				}
			}
			_ = w.WriteMsg(m)
			return
		}

		s.handleGuardDomain(m, q, strings.TrimSuffix(qName, "."))
		_ = w.WriteMsg(m)
		return
	}

	// Check if subdomain is blocked due to abuse
	if blocked, _ := s.db.IsSubdomainBlocked(subdomain); blocked {
		m.SetRcode(r, dns.RcodeRefused)
		_ = w.WriteMsg(m)
		s.logAndBroadcast(subdomain, q.Name, qTypeStr, clientIP, "REFUSED", []string{"Subdomain blocked due to abuse violation"})
		return
	}

	// Check if user/subdomain exists
	user, err := s.db.GetUserBySubdomain(subdomain)
	if err != nil || user == nil {
		m.SetRcode(r, dns.RcodeNameError)
		_ = w.WriteMsg(m)
		s.logAndBroadcast(subdomain, q.Name, qTypeStr, clientIP, "NXDOMAIN", []string{})
		return
	}

	// Touch user last active
	s.db.TouchUser(subdomain)

	// Lookup records
	records, err := s.db.GetRecordsByNameAndType(subdomain, recordName, qTypeStr)
	answers := make([]string, 0)

	if err == nil && len(records) > 0 {
		m.SetRcode(r, dns.RcodeSuccess)
		for _, rec := range records {
			rr := s.buildRR(rec, q.Name)
			if rr != nil {
				m.Answer = append(m.Answer, rr)
				answers = append(answers, fmt.Sprintf("%s %d IN %s %s", q.Name, rec.TTL, rec.Type, rec.Value))
			}
		}
	} else {
		// Check if any record exists for this name (NODATA / NOERROR vs NXDOMAIN)
		allForName, _ := s.db.GetRecordsByNameAndType(subdomain, recordName, "ANY")
		if len(allForName) > 0 {
			// Name exists, but not this type -> NOERROR with empty answer (classic DNS)
			m.SetRcode(r, dns.RcodeSuccess)
		} else {
			// Name does not exist -> NXDOMAIN
			m.SetRcode(r, dns.RcodeNameError)
		}
	}

	// Add SOA in Authority section for negative responses or SOA queries
	if len(m.Answer) == 0 || q.Qtype == dns.TypeSOA {
		soa := &dns.SOA{
			Hdr:     dns.RR_Header{Name: fmt.Sprintf("%s.%s.", subdomain, s.cfg.BaseDomain), Rrtype: dns.TypeSOA, Class: dns.ClassINET, Ttl: 300},
			Ns:      fmt.Sprintf("ns1.%s.", s.cfg.BaseDomain),
			Mbox:    fmt.Sprintf("hostmaster.%s.", s.cfg.BaseDomain),
			Serial:  uint32(time.Now().Unix()),
			Refresh: 3600,
			Retry:   600,
			Expire:  604800,
			Minttl:  60,
		}
		if q.Qtype == dns.TypeSOA {
			m.Answer = append(m.Answer, soa)
			answers = append(answers, soa.String())
		} else {
			m.Ns = append(m.Ns, soa)
		}
	}

	_ = w.WriteMsg(m)

	rcodeStr := dns.RcodeToString[m.Rcode]
	s.logAndBroadcast(subdomain, q.Name, qTypeStr, clientIP, rcodeStr, answers)
}

func (s *Server) handleBaseDomain(m *dns.Msg, q dns.Question) {
	base := dns.Fqdn(s.cfg.BaseDomain)
	soa := &dns.SOA{
		Hdr:     dns.RR_Header{Name: base, Rrtype: dns.TypeSOA, Class: dns.ClassINET, Ttl: 3600},
		Ns:      fmt.Sprintf("ns1.%s", base),
		Mbox:    fmt.Sprintf("hostmaster.%s", base),
		Serial:  s.GetZoneSerial(),
		Refresh: 300,
		Retry:   120,
		Expire:  1209600,
		Minttl:  60,
	}

	nsRecords := []dns.RR{
		&dns.NS{Hdr: dns.RR_Header{Name: base, Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: 3600}, Ns: "ns1.he.net."},
		&dns.NS{Hdr: dns.RR_Header{Name: base, Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: 3600}, Ns: "ns2.he.net."},
		&dns.NS{Hdr: dns.RR_Header{Name: base, Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: 3600}, Ns: "ns3.he.net."},
		&dns.NS{Hdr: dns.RR_Header{Name: base, Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: 3600}, Ns: "ns4.he.net."},
		&dns.NS{Hdr: dns.RR_Header{Name: base, Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: 3600}, Ns: "ns5.he.net."},
		&dns.NS{Hdr: dns.RR_Header{Name: base, Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: 3600}, Ns: fmt.Sprintf("ns1.%s", base)},
		&dns.NS{Hdr: dns.RR_Header{Name: base, Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: 3600}, Ns: fmt.Sprintf("ns2.%s", base)},
	}

	aaaaRecord := &dns.AAAA{
		Hdr:  dns.RR_Header{Name: base, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 300},
		AAAA: net.ParseIP("2606:c700:4020:0098:1234:4321:73ab:0001"),
	}

	fallbackIP := s.GetFallbackIPv4()
	var aRecord *dns.A
	if fallbackIP != nil {
		aRecord = &dns.A{
			Hdr: dns.RR_Header{Name: base, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
			A:   fallbackIP,
		}
	}

	switch q.Qtype {
	case dns.TypeNS:
		m.Answer = append(m.Answer, nsRecords...)
	case dns.TypeSOA:
		m.Answer = append(m.Answer, soa)
		m.Ns = append(m.Ns, nsRecords...)
	case dns.TypeA:
		if aRecord != nil {
			m.Answer = append(m.Answer, aRecord)
		} else {
			m.Rcode = dns.RcodeSuccess
			m.Ns = append(m.Ns, soa)
		}
	case dns.TypeAAAA:
		m.Answer = append(m.Answer, aaaaRecord)
	case dns.TypeANY:
		m.Answer = append(m.Answer, soa)
		m.Answer = append(m.Answer, nsRecords...)
		if aRecord != nil {
			m.Answer = append(m.Answer, aRecord)
		}
		m.Answer = append(m.Answer, aaaaRecord)
	default:
		// NODATA: return NOERROR with SOA in Authority section
		m.Rcode = dns.RcodeSuccess
		m.Ns = append(m.Ns, soa)
	}
}
