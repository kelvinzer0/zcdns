package dns

import (
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/miekg/dns"
)

func (s *Server) handleAXFR(w dns.ResponseWriter, r *dns.Msg) {
	// AXFR requires TCP connection
	if _, ok := w.RemoteAddr().(*net.TCPAddr); !ok {
		log.Printf("[AXFR] Refused request from non-TCP client: %v", w.RemoteAddr())
		m := new(dns.Msg)
		m.SetReply(r)
		m.SetRcode(r, dns.RcodeRefused)
		_ = w.WriteMsg(m)
		return
	}

	defer func() {
		_ = w.Close()
	}()

	base := dns.Fqdn(s.cfg.BaseDomain)
	if len(r.Question) > 0 {
		qName := strings.ToLower(r.Question[0].Name)
		if strings.TrimSuffix(qName, ".") != strings.ToLower(s.cfg.BaseDomain) {
			log.Printf("[AXFR] Refused request for non-zone domain %s from %s", qName, w.RemoteAddr())
			m := new(dns.Msg)
			m.SetReply(r)
			m.SetRcode(r, dns.RcodeNotAuth)
			_ = w.WriteMsg(m)
			return
		}
	}

	serial := s.GetZoneSerial()
	log.Printf("[AXFR] Initiating zone transfer for %s to %s (Serial: %d)", base, w.RemoteAddr(), serial)

	soa := &dns.SOA{
		Hdr:     dns.RR_Header{Name: base, Rrtype: dns.TypeSOA, Class: dns.ClassINET, Ttl: 3600},
		Ns:      fmt.Sprintf("ns1.%s", base),
		Mbox:    fmt.Sprintf("hostmaster.%s", base),
		Serial:  serial,
		Refresh: 300,
		Retry:   120,
		Expire:  1209600,
		Minttl:  60,
	}

	records := make([]dns.RR, 0)
	// 1. First record must be SOA (RFC 5936)
	records = append(records, soa)

	// 2. Nameservers
	for _, ns := range []string{
		"ns1.he.net.", "ns2.he.net.", "ns3.he.net.", "ns4.he.net.", "ns5.he.net.",
		fmt.Sprintf("ns1.%s", base), fmt.Sprintf("ns2.%s", base),
	} {
		records = append(records, &dns.NS{
			Hdr: dns.RR_Header{Name: base, Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: 3600},
			Ns:  ns,
		})
	}

	// 3. Base records & nameservers
	serverIP := net.ParseIP("2606:c700:4020:0098:1234:4321:73ab:0001")
	records = append(records,
		&dns.AAAA{Hdr: dns.RR_Header{Name: base, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 300}, AAAA: serverIP},
		&dns.AAAA{Hdr: dns.RR_Header{Name: fmt.Sprintf("ns1.%s", base), Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 300}, AAAA: serverIP},
		&dns.AAAA{Hdr: dns.RR_Header{Name: fmt.Sprintf("ns2.%s", base), Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 300}, AAAA: serverIP},
		&dns.AAAA{Hdr: dns.RR_Header{Name: fmt.Sprintf("www.%s", base), Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 300}, AAAA: serverIP},
		&dns.AAAA{Hdr: dns.RR_Header{Name: fmt.Sprintf("guard.%s", base), Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 300}, AAAA: serverIP},
		&dns.AAAA{Hdr: dns.RR_Header{Name: fmt.Sprintf("*.guard.%s", base), Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 300}, AAAA: serverIP},
		// Wildcard AAAA: prevents HE.net slave from replying NXDOMAIN for active subdomains
		&dns.AAAA{Hdr: dns.RR_Header{Name: fmt.Sprintf("*.%s", base), Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 60}, AAAA: serverIP},
	)

	if fallbackIP := s.GetFallbackIPv4(); fallbackIP != nil {
		records = append(records,
			// ns1/ns2 are AAAA-only; A record only on base domain, www, guard, and wildcard
			&dns.A{Hdr: dns.RR_Header{Name: base, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300}, A: fallbackIP},
			&dns.A{Hdr: dns.RR_Header{Name: fmt.Sprintf("www.%s", base), Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300}, A: fallbackIP},
			&dns.A{Hdr: dns.RR_Header{Name: fmt.Sprintf("guard.%s", base), Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300}, A: fallbackIP},
			&dns.A{Hdr: dns.RR_Header{Name: fmt.Sprintf("*.guard.%s", base), Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300}, A: fallbackIP},
			&dns.A{Hdr: dns.RR_Header{Name: fmt.Sprintf("*.%s", base), Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60}, A: fallbackIP},
		)
	}

	// 3.5. ACME TXT challenges for guard domain
	seenChallenge := make(map[string]bool)
	for _, val := range s.GetAcmeChallenges("guard") {
		if !seenChallenge[val] {
			seenChallenge[val] = true
			records = append(records, &dns.TXT{
				Hdr: dns.RR_Header{Name: fmt.Sprintf("_acme-challenge.guard.%s", base), Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: 60},
				Txt: []string{val},
			})
		}
	}

	// 4. All active user records from database
	dbRecords, err := s.db.GetAllActiveRecords()
	if err == nil {
		for _, dbr := range dbRecords {
			var fqdn string
			if dbr.Name == "@" || dbr.Name == "" {
				fqdn = fmt.Sprintf("%s.%s", dbr.Subdomain, base)
			} else {
				fqdn = fmt.Sprintf("%s.%s.%s", dbr.Name, dbr.Subdomain, base)
			}
			if rr := s.buildRR(dbr, fqdn); rr != nil {
				if dbr.Subdomain == "guard" && dbr.Name == "_acme-challenge" && dbr.Type == "TXT" {
					val := strings.Trim(dbr.Value, "\"")
					if seenChallenge[val] {
						continue
					}
					seenChallenge[val] = true
				}
				records = append(records, rr)
			}
		}
	}

	// 5. Final record must be identical SOA (RFC 5936)
	records = append(records, soa)

	ch := make(chan *dns.Envelope, 1)
	ch <- &dns.Envelope{RR: records}
	close(ch)

	tr := new(dns.Transfer)
	if err := tr.Out(w, r, ch); err != nil {
		log.Printf("[AXFR] Transfer error to %s: %v", w.RemoteAddr(), err)
	} else {
		log.Printf("[AXFR] Successfully transferred %d records to %s (Serial: %d)", len(records), w.RemoteAddr(), serial)
	}
}
