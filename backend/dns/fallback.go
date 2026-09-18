package dns

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/miekg/dns"
)

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
