package dns

import (
	"fmt"

	"github.com/miekg/dns"
)

func (s *Server) handleGuardDomain(m *dns.Msg, q dns.Question, name string) {
	target := dns.Fqdn(name)
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

	cnameTarget := dns.Fqdn(FallbackHost)
	cnameRecord := &dns.CNAME{
		Hdr:    dns.RR_Header{Name: target, Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: 300},
		Target: cnameTarget,
	}

	switch q.Qtype {
	case dns.TypeA, dns.TypeAAAA, dns.TypeCNAME, dns.TypeANY:
		m.Answer = append(m.Answer, cnameRecord)
	default:
		m.SetRcode(m, dns.RcodeSuccess)
		m.Ns = append(m.Ns, soa)
	}
}
