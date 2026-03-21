package dns

import (
	"sync"
	"time"

	"github.com/miekg/dns"
)

type cacheEntry struct {
	msg       *dns.Msg
	expiresAt time.Time
}

type DNS struct {
	ListenAddr string
	Resolver   func(name string) (string, bool)

	udpServer *dns.Server
	tcpServer *dns.Server
	running   bool
	cache     map[string]cacheEntry
	cacheMu   sync.RWMutex
	cacheTTL  time.Duration
}

const (
	defaultCacheTTL = 30 * time.Second
	startTimeout    = 5 * time.Second
	externalDNS     = "8.8.8.8:53"
)
