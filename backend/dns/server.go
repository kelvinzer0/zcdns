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

const FallbackHost = "2606-0c70-0020-0098-1234-4321-73ab-0001.withfallback.com"

type Server struct {
	cfg            *config.Config
	db             *db.DB
	broadcaster    QueryBroadcaster
	parental       *parental.Engine
	servers        []*dns.Server
	mu             sync.RWMutex
	zoneSerial     uint32
	fallbackIPv4   net.IP
	acmeChallenges map[string][]string
	stopChan       chan struct{}
}

func NewServer(cfg *config.Config, database *db.DB, broadcaster QueryBroadcaster, pe *parental.Engine) *Server {
	return &Server{
		cfg:         cfg,
		db:          database,
		broadcaster: broadcaster,
		parental:    pe,
		zoneSerial:  uint32(time.Now().Unix()),
		acmeChallenges: map[string][]string{
			"guard": {"Iu6XSSqIx5-IKIceXWVoT4Z54S9DwsMewAo781Pk-DE"},
		},
		stopChan: make(chan struct{}),
	}
}

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

func (s *Server) GetFallbackIPv4() net.IP {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.fallbackIPv4
}

func (s *Server) SetFallbackIPv4(ip net.IP) {
	s.mu.Lock()
	s.fallbackIPv4 = ip
	s.mu.Unlock()
}

func resolveFallbackIPv4(hostname string) (net.IP, error) {
	resolvers := []string{
		"[2606:4700:4700::1111]:53",
		"[2001:4860:4860::8888]:53",
		"1.1.1.1:53",
		"8.8.8.8:53",
	}

	client := &dns.Client{Timeout: 3 * time.Second}
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(hostname), dns.TypeA)

	for _, res := range resolvers {
		resp, _, err := client.Exchange(m, res)
		if err == nil && resp != nil && len(resp.Answer) > 0 {
			for _, ans := range resp.Answer {
				if a, ok := ans.(*dns.A); ok && a.A != nil {
					return a.A, nil
				}
			}
		}
	}

	ips, err := net.LookupIP(hostname)
	if err == nil {
		for _, ip := range ips {
			if v4 := ip.To4(); v4 != nil {
				return v4, nil
			}
		}
	}

	return nil, fmt.Errorf("unable to resolve IPv4 A record for %s", hostname)
}

func (s *Server) updateFallbackIPv4() {
	newIP, err := resolveFallbackIPv4(FallbackHost)
	if err != nil {
		log.Printf("[DNS-FALLBACK] Failed to resolve %s: %v", FallbackHost, err)
		return
	}

	s.mu.Lock()
	oldIP := s.fallbackIPv4
	changed := oldIP == nil || !oldIP.Equal(newIP)
	if changed {
		s.fallbackIPv4 = newIP
		s.zoneSerial++
	}
	serial := s.zoneSerial
	s.mu.Unlock()

	if changed {
		log.Printf("[DNS-FALLBACK] Updated fallback IPv4 to %s (old: %v, Serial: %d)", newIP.String(), oldIP, serial)
		s.NotifySlaves()
	}
}

func (s *Server) startFallbackUpdater() {
	go s.updateFallbackIPv4()

	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.updateFallbackIPv4()
			case <-s.stopChan:
				return
			}
		}
	}()
}

func (s *Server) GetZoneSerial() uint32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.zoneSerial
}

func (s *Server) IncrementSerialAndNotify() {
	s.mu.Lock()
	s.zoneSerial++
	s.mu.Unlock()
	s.NotifySlaves()
}

func (s *Server) NotifySlaves() {
	base := dns.Fqdn(s.cfg.BaseDomain)
	msg := new(dns.Msg)
	msg.SetNotify(base)

	slaves := []string{
		"216.218.133.2:53",
		"[2001:470:600::2]:53",
		"ns1.he.net:53",
		"ns2.he.net:53",
		"ns3.he.net:53",
		"ns4.he.net:53",
		"ns5.he.net:53",
	}

	client := new(dns.Client)
	client.Timeout = 3 * time.Second

	for _, slave := range slaves {
		go func(target string) {
			_, _, _ = client.Exchange(msg, target)
		}(slave)
	}
	log.Printf("[DNS-NOTIFY] Sent NOTIFY for %s to %d slaves (Serial: %d)", base, len(slaves), s.GetZoneSerial())
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
		s.startFallbackUpdater()
		return nil
	}
}

func (s *Server) Stop() {
	select {
	case <-s.stopChan:
	default:
		close(s.stopChan)
	}
	for _, srv := range s.servers {
		_ = srv.Shutdown()
	}
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
			&dns.A{Hdr: dns.RR_Header{Name: base, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300}, A: fallbackIP},
			&dns.A{Hdr: dns.RR_Header{Name: fmt.Sprintf("ns1.%s", base), Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300}, A: fallbackIP},
			&dns.A{Hdr: dns.RR_Header{Name: fmt.Sprintf("ns2.%s", base), Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300}, A: fallbackIP},
			&dns.A{Hdr: dns.RR_Header{Name: fmt.Sprintf("www.%s", base), Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300}, A: fallbackIP},
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

func (s *Server) handleGuardDomain(m *dns.Msg, q dns.Question, name string) {
	target := dns.Fqdn(name)
	serverIP := net.ParseIP("2606:c700:4020:0098:1234:4321:73ab:0001")
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
	case dns.TypeA:
		// Guard domains (*.guard.zcdns.id) are strictly IPv6 (AAAA only).
		// Return NOERROR NODATA for IPv4 A queries so withfallback is not triggered.
		m.SetRcode(m, dns.RcodeSuccess)
		m.Ns = append(m.Ns, soa)
	default:
		m.SetRcode(m, dns.RcodeSuccess)
		m.Ns = append(m.Ns, soa)
	}
}

func (s *Server) handleNameserverDomain(m *dns.Msg, q dns.Question, name string) {
	target := dns.Fqdn(name)
	serverIP := net.ParseIP("2606:c700:4020:0098:1234:4321:73ab:0001")
	fallbackIP := s.GetFallbackIPv4()

	var aRecord *dns.A
	if fallbackIP != nil {
		aRecord = &dns.A{
			Hdr: dns.RR_Header{Name: target, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
			A:   fallbackIP,
		}
	}

	switch q.Qtype {
	case dns.TypeA:
		if aRecord != nil {
			m.Answer = append(m.Answer, aRecord)
		} else {
			m.SetRcode(m, dns.RcodeSuccess)
		}
	case dns.TypeAAAA:
		m.Answer = append(m.Answer, &dns.AAAA{
			Hdr:  dns.RR_Header{Name: target, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 300},
			AAAA: serverIP,
		})
	case dns.TypeANY:
		if aRecord != nil {
			m.Answer = append(m.Answer, aRecord)
		}
		m.Answer = append(m.Answer, &dns.AAAA{
			Hdr:  dns.RR_Header{Name: target, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 300},
			AAAA: serverIP,
		})
	default:
		m.SetRcode(m, dns.RcodeSuccess)
		base := dns.Fqdn(s.cfg.BaseDomain)
		m.Ns = append(m.Ns, &dns.SOA{
			Hdr:     dns.RR_Header{Name: base, Rrtype: dns.TypeSOA, Class: dns.ClassINET, Ttl: 3600},
			Ns:      fmt.Sprintf("ns1.%s", base),
			Mbox:    fmt.Sprintf("hostmaster.%s", base),
			Serial:  s.GetZoneSerial(),
			Refresh: 300,
			Retry:   120,
			Expire:  1209600,
			Minttl:  60,
		})
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
