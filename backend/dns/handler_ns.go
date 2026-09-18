package dns

import (
	"fmt"
	"net"

	"github.com/miekg/dns"
)

func (s *Server) handleNameserverDomain(m *dns.Msg, q dns.Question, name string) {
	target := dns.Fqdn(name)
	serverIP := net.ParseIP("2606:c700:4020:0098:1234:4321:73ab:0001")
	// ns1/ns2 are AAAA-only — no A (withfallback) record to avoid resolver confusion.
	// A queries return NODATA (NOERROR + SOA in Authority).
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

	switch q.Qtype {
	case dns.TypeA:
		// NODATA — ns1/ns2 have no IPv4 address
		m.SetRcode(m, dns.RcodeSuccess)
		m.Ns = append(m.Ns, soa)
	case dns.TypeAAAA:
		m.Answer = append(m.Answer, &dns.AAAA{
			Hdr:  dns.RR_Header{Name: target, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 300},
			AAAA: serverIP,
		})
	case dns.TypeANY:
		m.Answer = append(m.Answer, &dns.AAAA{
			Hdr:  dns.RR_Header{Name: target, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 300},
			AAAA: serverIP,
		})
	default:
		m.SetRcode(m, dns.RcodeSuccess)
		m.Ns = append(m.Ns, soa)
	}
}
