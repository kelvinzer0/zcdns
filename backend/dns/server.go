package dns

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/miekg/dns"
	"zcdns-backend/config"
	"zcdns-backend/db"
	"zcdns-backend/parental"
)

type QueryBroadcaster interface {
	BroadcastRequest(subdomain string, req *db.RequestLog)
}

type Server struct {
	cfg         *config.Config
	db          *db.DB
	broadcaster QueryBroadcaster
	parental    *parental.Engine
	servers     []*dns.Server
	mu          sync.RWMutex
}

func NewServer(cfg *config.Config, database *db.DB, broadcaster QueryBroadcaster, pe *parental.Engine) *Server {
	return &Server{
		cfg:         cfg,
		db:          database,
		broadcaster: broadcaster,
		parental:    pe,
	}
}

func (s *Server) Start() error {
	dns.HandleFunc(".", s.handleDNSRequest)

	errChan := make(chan error, len(s.cfg.DNSAddrs)*2)

	for _, addr := range s.cfg.DNSAddrs {
		udpSrv := &dns.Server{Addr: addr, Net: "udp"}
		tcpSrv := &dns.Server{Addr: addr, Net: "tcp"}
		s.servers = append(s.servers, udpSrv, tcpSrv)

		go func(srv *dns.Server, address string) {
			log.Printf("[DNS] Starting UDP DNS listener on %s (domain: *.%s)", address, s.cfg.BaseDomain)
			if err := srv.ListenAndServe(); err != nil {
				errChan <- fmt.Errorf("UDP error on %s: %w", address, err)
			}
		}(udpSrv, addr)

		go func(srv *dns.Server, address string) {
			log.Printf("[DNS] Starting TCP DNS listener on %s", address)
			if err := srv.ListenAndServe(); err != nil {
				errChan <- fmt.Errorf("TCP error on %s: %w", address, err)
			}
		}(tcpSrv, addr)
	}

	select {
	case err := <-errChan:
		return err
	case <-time.After(150 * time.Millisecond):
		return nil
	}
}

func (s *Server) Stop() {
	for _, srv := range s.servers {
		_ = srv.Shutdown()
	}
}

func (s *Server) handleDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
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

	// Handle queries directly targeted at authoritative nameservers (ns1.zcdns.id / ns2.zcdns.id)
	if (subdomain == "ns1" || subdomain == "ns2") && (recordName == "@" || recordName == "") {
		s.handleNameserverDomain(m, q, strings.TrimSuffix(qName, "."))
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
	switch q.Qtype {
	case dns.TypeNS:
		m.Answer = append(m.Answer,
			&dns.NS{Hdr: dns.RR_Header{Name: base, Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: 3600}, Ns: fmt.Sprintf("ns1.%s", base)},
			&dns.NS{Hdr: dns.RR_Header{Name: base, Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: 3600}, Ns: fmt.Sprintf("ns2.%s", base)},
		)
	case dns.TypeSOA:
		m.Answer = append(m.Answer, &dns.SOA{
			Hdr:     dns.RR_Header{Name: base, Rrtype: dns.TypeSOA, Class: dns.ClassINET, Ttl: 3600},
			Ns:      fmt.Sprintf("ns1.%s", base),
			Mbox:    fmt.Sprintf("hostmaster.%s", base),
			Serial:  2026091701,
			Refresh: 7200,
			Retry:   3600,
			Expire:  1209600,
			Minttl:  300,
		})
	case dns.TypeAAAA:
		m.Answer = append(m.Answer, &dns.AAAA{
			Hdr:  dns.RR_Header{Name: base, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 300},
			AAAA: net.ParseIP("2606:c700:4020:0098:1234:4321:73ab:0001"),
		})
	default:
		m.SetRcode(m, dns.RcodeSuccess)
	}
}

func (s *Server) handleNameserverDomain(m *dns.Msg, q dns.Question, name string) {
	target := dns.Fqdn(name)
	switch q.Qtype {
	case dns.TypeAAAA:
		m.Answer = append(m.Answer, &dns.AAAA{
			Hdr:  dns.RR_Header{Name: target, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 300},
			AAAA: net.ParseIP("2606:c700:4020:0098:1234:4321:73ab:0001"),
		})
	default:
		m.SetRcode(m, dns.RcodeSuccess)
	}
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
