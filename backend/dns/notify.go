package dns

import (
	"log"
	"time"

	"github.com/miekg/dns"
)

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
