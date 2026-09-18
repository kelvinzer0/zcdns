package dns

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"zcdns-backend/config"
	"zcdns-backend/db"
	"zcdns-backend/parental"

	"github.com/miekg/dns"
)

type QueryBroadcaster interface {
	BroadcastRequest(subdomain string, req *db.RequestLog)
}

const FallbackHost = "2606-c700-4020-0098-1234-4321-73ab-0001.withfallback.com"

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
	// sniMap maps remoteAddr (string) -> guard subdomain extracted from TLS SNI
	sniMap sync.Map
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

// StartDoT starts a DNS-over-TLS listener on cfg.DoTPort (default 853).
// It uses the TLS certificate at cfg.TLSCertFile / cfg.TLSKeyFile to serve
// *.guard.<BaseDomain> as Private DNS hostname for Android devices.
// The guard subdomain is extracted from the TLS SNI (e.g. "panda65" from
// "panda65.guard.zcdns.id") and forwarded to the parental filtering engine.
func (s *Server) StartDoT() error {
	if s.cfg.DoTPort == 0 || s.cfg.TLSCertFile == "" || s.cfg.TLSKeyFile == "" {
		log.Printf("[DoT] DoT disabled (no port/cert configured)")
		return nil
	}

	cert, err := tls.LoadX509KeyPair(s.cfg.TLSCertFile, s.cfg.TLSKeyFile)
	if err != nil {
		return fmt.Errorf("[DoT] failed to load TLS keypair: %w", err)
	}

	guardSuffix := ".guard." + strings.ToLower(s.cfg.BaseDomain)

	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		// GetConfigForClient is called per-connection during TLS handshake.
		// We capture hello.ServerName to derive the user subdomain and store
		// it in sniMap keyed by the connection's remote address string.
		GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
			sni := strings.ToLower(hello.ServerName)
			var subdomain string
			if strings.HasSuffix(sni, guardSuffix) {
				sub := strings.TrimSuffix(sni, guardSuffix)
				if sub != "" && !strings.Contains(sub, ".") {
					subdomain = sub
				}
			}
			if subdomain == "" {
				subdomain = "default"
			}
			// RemoteAddr().String() is the canonical key for this connection.
			remoteKey := hello.Conn.RemoteAddr().String()
			s.sniMap.Store(remoteKey, subdomain)
			log.Printf("[DoT] TLS handshake from %s SNI=%q subdomain=%q", remoteKey, sni, subdomain)
			return nil, nil // nil = use the parent tlsCfg
		},
	}

	for _, dnsAddr := range s.cfg.DNSAddrs {
		host, _, err := net.SplitHostPort(dnsAddr)
		if err != nil {
			log.Printf("[DoT] invalid DNS address %q: %v", dnsAddr, err)
			continue
		}
		addr := net.JoinHostPort(host, strconv.Itoa(s.cfg.DoTPort))
		dotSrv := &dns.Server{
			Addr:      addr,
			Net:       "tcp-tls",
			TLSConfig: tlsCfg,
			Handler:   dns.HandlerFunc(s.handleDoTRequest),
		}

		s.mu.Lock()
		s.servers = append(s.servers, dotSrv)
		s.mu.Unlock()

		go func(srv *dns.Server, address string) {
			log.Printf("[DoT] Starting DNS-over-TLS listener on %s (guard.%s)", address, s.cfg.BaseDomain)
			if err := srv.ListenAndServe(); err != nil {
				log.Printf("[DoT] listener stopped on %s: %v", address, err)
			}
		}(dotSrv, addr)
	}

	// Brief pause to catch immediate listen errors (port in use, etc.)
	time.Sleep(100 * time.Millisecond)
	return nil
}

// handleDoTRequest is the DNS handler for the DoT (port 853) listener.
// It looks up the per-connection SNI subdomain from sniMap and routes the
// query through the parental engine so blocking rules are applied.
