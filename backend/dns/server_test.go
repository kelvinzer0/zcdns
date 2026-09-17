package dns

import (
	"net"
	"os"
	"testing"
	"time"

	"github.com/miekg/dns"
	"zcdns-backend/config"
	"zcdns-backend/db"
)

func TestAXFR(t *testing.T) {
	dbFile := "./test_axfr.sqlite"
	_ = os.Remove(dbFile)
	defer os.Remove(dbFile)

	database, err := db.InitDB(dbFile)
	if err != nil {
		t.Fatalf("Failed to create db: %v", err)
	}
	defer database.Close()

	// Add a test user and record
	if err := database.CreateUser("uid1", "testsub"); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}
	if err := database.AddRecord(&db.Record{
		ID:        "rec1",
		Subdomain: "testsub",
		Name:      "@",
		Type:      "A",
		Value:     "1.2.3.4",
		TTL:       300,
	}); err != nil {
		t.Fatalf("Failed to add record: %v", err)
	}

	cfg := &config.Config{
		DNSAddrs:   []string{"127.0.0.1:15354"},
		BaseDomain: "zcdns.id",
	}

	srv := NewServer(cfg, database, nil, nil)
	if err := srv.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer srv.Stop()

	// Give it a moment to bind
	time.Sleep(100 * time.Millisecond)

	// Test SOA query over TCP
	c := new(dns.Client)
	c.Net = "tcp"
	m := new(dns.Msg)
	m.SetQuestion("zcdns.id.", dns.TypeSOA)

	resp, _, err := c.Exchange(m, "127.0.0.1:15354")
	if err != nil {
		t.Fatalf("TCP SOA query failed: %v", err)
	}
	if len(resp.Answer) == 0 {
		t.Fatalf("Expected SOA in answer, got 0 answers")
	}
	soa, ok := resp.Answer[0].(*dns.SOA)
	if !ok {
		t.Fatalf("Expected SOA record type, got %T", resp.Answer[0])
	}
	t.Logf("SOA Serial: %d", soa.Serial)

	// Test AXFR transfer
	tr := new(dns.Transfer)
	axfrMsg := new(dns.Msg)
	axfrMsg.SetAxfr("zcdns.id.")

	channel, err := tr.In(axfrMsg, "127.0.0.1:15354")
	if err != nil {
		t.Fatalf("AXFR In failed: %v", err)
	}

	count := 0
	for env := range channel {
		if env.Error != nil {
			t.Fatalf("AXFR envelope error: %v", env.Error)
		}
		for _, rr := range env.RR {
			count++
			t.Logf("AXFR Record: %s", rr.String())
		}
	}

	if count < 4 {
		t.Fatalf("Expected at least 4 records in AXFR, got %d", count)
	}
	t.Logf("AXFR successfully received %d records", count)
}

func TestFallbackIPv4(t *testing.T) {
	dbFile := "./test_fallback.sqlite"
	_ = os.Remove(dbFile)
	defer os.Remove(dbFile)

	database, err := db.InitDB(dbFile)
	if err != nil {
		t.Fatalf("Failed to create db: %v", err)
	}
	defer database.Close()

	cfg := &config.Config{
		DNSAddrs:   []string{"127.0.0.1:15355"},
		BaseDomain: "zcdns.id",
	}

	srv := NewServer(cfg, database, nil, nil)
	srv.SetFallbackIPv4(net.ParseIP("45.33.22.33"))
	if err := srv.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer srv.Stop()

	time.Sleep(100 * time.Millisecond)

	c := new(dns.Client)
	c.Net = "udp"

	testCases := []string{
		"zcdns.id.",
		"ns1.zcdns.id.",
		"ns2.zcdns.id.",
		"www.zcdns.id.",
		"guard.zcdns.id.",
		"testsub.guard.zcdns.id.",
	}

	for _, domain := range testCases {
		m := new(dns.Msg)
		m.SetQuestion(domain, dns.TypeA)
		resp, _, err := c.Exchange(m, "127.0.0.1:15355")
		if err != nil {
			t.Fatalf("Query %s failed: %v", domain, err)
		}
		if len(resp.Answer) == 0 {
			t.Fatalf("Expected A record for %s, got none", domain)
		}
		aRec, ok := resp.Answer[0].(*dns.A)
		if !ok {
			t.Fatalf("Expected *dns.A record for %s, got %T", domain, resp.Answer[0])
		}
		if aRec.A.String() != "45.33.22.33" {
			t.Fatalf("Expected 45.33.22.33 for %s, got %s", domain, aRec.A.String())
		}
		t.Logf("Successfully verified %s -> %s", domain, aRec.A.String())

		// Verify AAAA record matches serverIP
		mAAAA := new(dns.Msg)
		mAAAA.SetQuestion(domain, dns.TypeAAAA)
		respAAAA, _, err := c.Exchange(mAAAA, "127.0.0.1:15355")
		if err != nil {
			t.Fatalf("Query AAAA %s failed: %v", domain, err)
		}
		if len(respAAAA.Answer) == 0 {
			t.Fatalf("Expected AAAA record for %s, got none", domain)
		}
		aaaaRec, ok := respAAAA.Answer[0].(*dns.AAAA)
		if !ok {
			t.Fatalf("Expected *dns.AAAA record for %s, got %T", domain, respAAAA.Answer[0])
		}
		if aaaaRec.AAAA.String() != "2606:c700:4020:98:1234:4321:73ab:1" {
			t.Fatalf("Expected 2606:c700:4020:98:1234:4321:73ab:1 for %s, got %s", domain, aaaaRec.AAAA.String())
		}
	}
}
