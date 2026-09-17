package parental

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
	"zcdns-backend/config"
	"zcdns-backend/db"
)

type QueryBroadcaster interface {
	BroadcastRequest(subdomain string, req *db.RequestLog)
}

type Engine struct {
	cfg         *config.Config
	db          *db.DB
	broadcaster QueryBroadcaster
	dnsClient   *dns.Client
	upstream    string
	cache       map[string]*cacheEntry
	cacheMu     sync.RWMutex
}

type cacheEntry struct {
	msg     *dns.Msg
	expires time.Time
}

func NewEngine(cfg *config.Config, database *db.DB, broadcaster QueryBroadcaster) *Engine {
	return &Engine{
		cfg:         cfg,
		db:          database,
		broadcaster: broadcaster,
		dnsClient: &dns.Client{
			Net:     "udp",
			Timeout: 2 * time.Second,
		},
		upstream: "1.1.1.1:53",
		cache:    make(map[string]*cacheEntry),
	}
}

// Curated blocklists
var adultKeywords = []string{
	"porn", "xxx", "xvideos", "xnxx", "chaturbate", "stripchat", "redtube",
	"youporn", "onlyfans", "livejasmin", "bonga", "camsoda", "spankbang",
	"beeg", "tubegalore", "hentai", "jav", "bokep", "nekopoi", "pornhub",
}

var gamblingKeywords = []string{
	"slot", "judi", "sbobet", "bet365", "poker", "casino", "togel",
	"pragmatic", "1xbet", "betway", "stake", "roobet", "gacor", "zeus",
	"roulette", "blackjack", "parlay", "taruhan", "bandar",
}

var adKeywords = []string{
	"doubleclick.net", "googleadservices.com", "googlesyndication.com",
	"adservice.google", "adform.net", "popads.net", "popcash.net",
	"taboola.com", "outbrain.com", "mgid.com", "adnxs.com", "criteo.com",
	"rubiconproject.com", "pubmatic.com", "advertising.com",
}

var malwareKeywords = []string{
	"malware", "phishing", "cryptominer", "coinhive", "trojan", "ransomware",
}

var defaultUpstreams = []string{
	"1.1.1.1:53",
	"8.8.8.8:53",
	"9.9.9.9:53",
	"[2606:4700:4700::1111]:53",
	"[2001:4860:4860::8888]:53",
}

var socialKeywords = []string{
	"tiktok.com", "instagram.com", "facebook.com", "fb.com", "twitter.com",
	"x.com", "snapchat.com", "reddit.com", "pinterest.com", "t.co",
}

var gamingKeywords = []string{
	"roblox.com", "rbxcdn.com", "rbx.com", "robloxlabs.com",
	"steampowered.com", "steamcommunity.com", "epicgames.com",
	"riotgames.com", "valorant.com", "twitch.tv", "discord.com", "discord.gg",
	"genshin.hoyoverse.com", "supercell.com", "minecraft.net", "mojang.com",
	"battlenet.com", "blizzard.com", "ea.com", "pubg.com",
}

func cleanDomainInput(d string) string {
	d = strings.ToLower(strings.TrimSpace(d))
	d = strings.TrimPrefix(d, "http://")
	d = strings.TrimPrefix(d, "https://")
	if idx := strings.Index(d, "/"); idx != -1 {
		d = d[:idx]
	}
	if idx := strings.Index(d, ":"); idx != -1 {
		d = d[:idx]
	}
	return strings.TrimSuffix(d, ".")
}

func (e *Engine) CheckDomain(pConfig *db.ParentalConfig, qName string) (blocked bool, reason string) {
	if !pConfig.Enabled {
		return false, ""
	}

	clean := cleanDomainInput(qName)

	// 1. Check custom allowlist (always takes precedence)
	for _, allowed := range pConfig.CustomAllowed {
		allowedClean := cleanDomainInput(allowed)
		if allowedClean != "" && (clean == allowedClean || strings.HasSuffix(clean, "."+allowedClean)) {
			return false, ""
		}
	}

	// 2. Check custom blocklist
	for _, blockedDomain := range pConfig.CustomBlocked {
		blockedClean := cleanDomainInput(blockedDomain)
		if blockedClean != "" && (clean == blockedClean || strings.HasSuffix(clean, "."+blockedClean)) {
			return true, fmt.Sprintf("Custom Blocklist: %s", blockedClean)
		}
	}

	// 3. Category filters
	if pConfig.BlockAdult && matchesAny(clean, adultKeywords) {
		return true, "Parental Filter: Adult & Pornography (18+)"
	}
	if pConfig.BlockGambling && matchesAny(clean, gamblingKeywords) {
		return true, "Parental Filter: Gambling & Judi Online"
	}
	if pConfig.BlockAds && matchesAny(clean, adKeywords) {
		return true, "Parental Filter: Advertising & Trackers"
	}
	if pConfig.BlockMalware && matchesAny(clean, malwareKeywords) {
		return true, "Parental Filter: Malware & Phishing"
	}
	if pConfig.BlockSocial && matchesAny(clean, socialKeywords) {
		return true, "Parental Filter: Social Media Platform"
	}
	if pConfig.BlockGaming && matchesAny(clean, gamingKeywords) {
		return true, "Parental Filter: Online Gaming Platform"
	}

	return false, ""
}

func matchesAny(domain string, keywords []string) bool {
	for _, kw := range keywords {
		if domain == kw || strings.HasSuffix(domain, "."+kw) || strings.Contains(domain, kw) {
			return true
		}
	}
	return false
}

// ProcessQuery processes any incoming DNS question under a user's parental policy
func (e *Engine) ProcessQuery(subdomain, clientIP string, req *dns.Msg) (*dns.Msg, error) {
	pConfig, err := e.db.GetParentalConfig(subdomain)
	if err != nil || pConfig == nil {
		pConfig = &db.ParentalConfig{
			Subdomain: subdomain,
			Enabled:   true,
			BlockMode: "0.0.0.0",
		}
	}

	if len(req.Question) == 0 {
		resp := new(dns.Msg)
		resp.SetReply(req)
		return resp, nil
	}

	q := req.Question[0]
	qName := strings.ToLower(q.Name)
	qTypeStr := dns.TypeToString[q.Qtype]

	// Check parental block
	blocked, reason := e.CheckDomain(pConfig, qName)

	if blocked {
		resp := new(dns.Msg)
		resp.SetReply(req)
		resp.Authoritative = true
		resp.RecursionAvailable = true

		sinkholeAnswers := make([]string, 0)

		if pConfig.BlockMode == "NXDOMAIN" {
			resp.SetRcode(req, dns.RcodeNameError)
		} else {
			// 0.0.0.0 sinkhole
			resp.SetRcode(req, dns.RcodeSuccess)
			if q.Qtype == dns.TypeA {
				rr := &dns.A{
					Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
					A:   net.ParseIP("0.0.0.0").To4(),
				}
				resp.Answer = append(resp.Answer, rr)
				sinkholeAnswers = append(sinkholeAnswers, fmt.Sprintf("%s 60 IN A 0.0.0.0", q.Name))
			} else if q.Qtype == dns.TypeAAAA {
				rr := &dns.AAAA{
					Hdr:  dns.RR_Header{Name: q.Name, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 60},
					AAAA: net.ParseIP("::").To16(),
				}
				resp.Answer = append(resp.Answer, rr)
				sinkholeAnswers = append(sinkholeAnswers, fmt.Sprintf("%s 60 IN AAAA ::", q.Name))
			}
		}

		// Log and broadcast blocked query
		e.logQuery(subdomain, q.Name, qTypeStr, clientIP, fmt.Sprintf("BLOCKED (%s)", reason), sinkholeAnswers)
		return resp, nil
	}

	// SafeSearch enforcement
	if pConfig.EnforceSafeSearch && (q.Qtype == dns.TypeA || q.Qtype == dns.TypeAAAA) {
		clean := strings.ToLower(strings.TrimSuffix(qName, "."))
		if strings.HasSuffix(clean, "google.com") || clean == "google.com" {
			return e.buildSafeSearchResponse(req, q, "216.239.38.120", subdomain, clientIP, "Google SafeSearch")
		}
		if strings.HasSuffix(clean, "bing.com") || clean == "bing.com" {
			return e.buildSafeSearchResponse(req, q, "204.79.197.220", subdomain, clientIP, "Bing Strict SafeSearch")
		}
		if strings.HasSuffix(clean, "youtube.com") || clean == "youtube.com" {
			return e.buildSafeSearchResponse(req, q, "216.239.38.119", subdomain, clientIP, "YouTube Restricted")
		}
	}

	// Query allowed -> forward to upstream recursive resolver
	resp, err := e.forwardUpstream(req)
	if err != nil {
		errResp := new(dns.Msg)
		errResp.SetReply(req)
		errResp.SetRcode(req, dns.RcodeServerFailure)
		e.logQuery(subdomain, q.Name, qTypeStr, clientIP, "SERVFAIL", []string{})
		return errResp, nil
	}

	answers := make([]string, 0)
	for _, a := range resp.Answer {
		answers = append(answers, a.String())
	}
	rcodeStr := dns.RcodeToString[resp.Rcode]
	e.logQuery(subdomain, q.Name, qTypeStr, clientIP, rcodeStr, answers)

	return resp, nil
}

func (e *Engine) buildSafeSearchResponse(req *dns.Msg, q dns.Question, ip string, subdomain, clientIP, service string) (*dns.Msg, error) {
	resp := new(dns.Msg)
	resp.SetReply(req)
	resp.Authoritative = false
	resp.RecursionAvailable = true
	resp.SetRcode(req, dns.RcodeSuccess)

	if q.Qtype == dns.TypeA {
		rr := &dns.A{
			Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
			A:   net.ParseIP(ip).To4(),
		}
		resp.Answer = append(resp.Answer, rr)
	}

	e.logQuery(subdomain, q.Name, "A", clientIP, fmt.Sprintf("SAFESEARCH (%s)", service), []string{fmt.Sprintf("%s 300 IN A %s", q.Name, ip)})
	return resp, nil
}

func (e *Engine) ClearCache() {
	e.cacheMu.Lock()
	e.cache = make(map[string]*cacheEntry)
	e.cacheMu.Unlock()
}

func (e *Engine) forwardUpstream(req *dns.Msg) (*dns.Msg, error) {
	q := req.Question[0]
	cacheKey := fmt.Sprintf("%s:%d", q.Name, q.Qtype)

	e.cacheMu.RLock()
	if entry, found := e.cache[cacheKey]; found {
		if time.Now().Before(entry.expires) {
			e.cacheMu.RUnlock()
			copyMsg := entry.msg.Copy()
			copyMsg.Id = req.Id
			return copyMsg, nil
		}
	}
	e.cacheMu.RUnlock()

	var resp *dns.Msg
	var err error

	upstreams := []string{e.upstream}
	for _, u := range defaultUpstreams {
		if u != e.upstream {
			upstreams = append(upstreams, u)
		}
	}

	for _, u := range upstreams {
		resp, _, err = e.dnsClient.Exchange(req, u)
		if err == nil && resp != nil {
			break
		}
	}

	if err != nil || resp == nil {
		return nil, fmt.Errorf("upstream resolution failed: %v", err)
	}

	ttl := uint32(60)
	if len(resp.Answer) > 0 {
		ttl = resp.Answer[0].Header().Ttl
		if ttl < 5 {
			ttl = 5
		} else if ttl > 300 {
			ttl = 300
		}
	}

	// Cache entry
	e.cacheMu.Lock()
	if len(e.cache) > 2000 {
		now := time.Now()
		for k, v := range e.cache {
			if now.After(v.expires) {
				delete(e.cache, k)
			}
		}
	}
	e.cache[cacheKey] = &cacheEntry{
		msg:     resp,
		expires: time.Now().Add(time.Duration(ttl) * time.Second),
	}
	e.cacheMu.Unlock()

	return resp, nil
}

func (e *Engine) logQuery(subdomain, qname, qtype, clientIP, rcode string, answers []string) {
	reqLog := &db.RequestLog{
		ID:        fmt.Sprintf("req-%d", time.Now().UnixNano()),
		Subdomain: subdomain,
		QName:     qname,
		QType:     qtype,
		ClientIP:  clientIP,
		RCode:     rcode,
		Answers:   answers,
		CreatedAt: time.Now(),
	}

	ansJSON, _ := json.Marshal(answers)
	_ = e.db.LogRequest(reqLog, string(ansJSON))

	if e.broadcaster != nil {
		e.broadcaster.BroadcastRequest(subdomain, reqLog)
	}
}

// HandleDoH handles DNS-over-HTTPS (RFC 8484) wire format requests
func (e *Engine) HandleDoH(w http.ResponseWriter, r *http.Request, subdomain string) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Subdomain")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	var dnsQueryBytes []byte
	var err error

	if r.Method == http.MethodGet {
		dnsParam := r.URL.Query().Get("dns")
		if dnsParam == "" {
			http.Error(w, "Missing dns query parameter", http.StatusBadRequest)
			return
		}
		dnsQueryBytes, err = base64.RawURLEncoding.DecodeString(dnsParam)
		if err != nil {
			dnsQueryBytes, err = base64.URLEncoding.DecodeString(dnsParam)
			if err != nil {
				http.Error(w, "Invalid base64url dns parameter", http.StatusBadRequest)
				return
			}
		}
	} else if r.Method == http.MethodPost {
		dnsQueryBytes, err = io.ReadAll(r.Body)
		if err != nil || len(dnsQueryBytes) == 0 {
			http.Error(w, "Empty DNS request body", http.StatusBadRequest)
			return
		}
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	reqMsg := new(dns.Msg)
	if err := reqMsg.Unpack(dnsQueryBytes); err != nil {
		http.Error(w, "Invalid DNS wire message", http.StatusBadRequest)
		return
	}

	clientIP := "remote"
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		clientIP = host
	}
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		clientIP = strings.TrimSpace(strings.Split(fwd, ",")[0])
	} else if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		clientIP = strings.TrimSpace(realIP)
	}

	respMsg, err := e.ProcessQuery(subdomain, clientIP, reqMsg)
	if err != nil {
		http.Error(w, "Internal DNS failure", http.StatusInternalServerError)
		return
	}

	respBytes, err := respMsg.Pack()
	if err != nil {
		http.Error(w, "Failed to pack DNS response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/dns-message")
	w.Header().Set("Cache-Control", "no-store, no-cache")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(respBytes)
}
