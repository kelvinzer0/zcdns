package parental

import (
	"os"
	"testing"

	"github.com/miekg/dns"
	"zcdns-backend/config"
	"zcdns-backend/db"
)

func TestRobloxParentalFilter(t *testing.T) {
	dbFile := "./test_parental.sqlite"
	_ = os.Remove(dbFile)
	defer os.Remove(dbFile)

	database, err := db.InitDB(dbFile)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer database.Close()

	subdomain := "gamertest"
	if err := database.CreateUser("uid-gamer", subdomain); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	cfg := &config.Config{
		BaseDomain: "zcdns.id",
	}
	engine := NewEngine(cfg, database, nil)

	// Case 1: Default config has BlockGaming = false
	pCfg, err := database.GetParentalConfig(subdomain)
	if err != nil {
		t.Fatalf("GetParentalConfig failed: %v", err)
	}

	blocked, reason := engine.CheckDomain(pCfg, "roblox.com.")
	if blocked {
		t.Fatalf("Expected roblox.com to NOT be blocked by default, got blocked: %s", reason)
	}

	// Case 2: Enable BlockGaming = true
	pCfg.BlockGaming = true
	if err := database.SaveParentalConfig(pCfg); err != nil {
		t.Fatalf("SaveParentalConfig failed: %v", err)
	}

	// Re-check CheckDomain
	blocked, reason = engine.CheckDomain(pCfg, "roblox.com.")
	if !blocked {
		t.Fatalf("Expected roblox.com. to be blocked when BlockGaming=true, got not blocked")
	}
	t.Logf("roblox.com blocked successfully: %s", reason)

	// Check subdomain variations
	blockedSub, _ := engine.CheckDomain(pCfg, "www.roblox.com.")
	if !blockedSub {
		t.Fatalf("Expected www.roblox.com. to be blocked")
	}

	blockedCdn, _ := engine.CheckDomain(pCfg, "setup.rbxcdn.com.")
	if !blockedCdn {
		t.Fatalf("Expected setup.rbxcdn.com. to be blocked")
	}

	// Case 3: Test ProcessQuery returns sinkhole 0.0.0.0
	req := new(dns.Msg)
	req.SetQuestion("roblox.com.", dns.TypeA)

	resp, err := engine.ProcessQuery(subdomain, "127.0.0.1", req)
	if err != nil {
		t.Fatalf("ProcessQuery failed: %v", err)
	}
	if len(resp.Answer) == 0 {
		t.Fatalf("Expected sinkhole answer, got none")
	}
	aRec, ok := resp.Answer[0].(*dns.A)
	if !ok {
		t.Fatalf("Expected *dns.A record, got %T", resp.Answer[0])
	}
	if aRec.A.String() != "0.0.0.0" {
		t.Fatalf("Expected 0.0.0.0 sinkhole IP for blocked roblox.com, got %s", aRec.A.String())
	}
	t.Logf("ProcessQuery successfully sinkholed roblox.com to %s", aRec.A.String())

	// Case 4: Whitelist roblox.com explicitly (CustomAllowed takes precedence)
	pCfg.CustomAllowed = []string{"roblox.com"}
	blockedAllowed, _ := engine.CheckDomain(pCfg, "roblox.com.")
	if blockedAllowed {
		t.Fatalf("Expected whitelisted roblox.com to NOT be blocked even with BlockGaming=true")
	}
	t.Logf("Whitelist successfully bypassed BlockGaming for roblox.com")
}
