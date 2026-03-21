package dns

import (
	"fmt"
	"log"
	"strings"
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

func NewDNS(listenAddr string, resolver func(string) (string, bool)) *DNS {
	return &DNS{
		ListenAddr: listenAddr,
		Resolver:   resolver,
		cache:      make(map[string]cacheEntry),
		cacheTTL:   30 * time.Second,
	}
}

func (d *DNS) Start() error {
	mux := dns.NewServeMux()
	mux.HandleFunc(".", d.handleDNS)

	d.udpServer = &dns.Server{
		Addr:    d.ListenAddr,
		Net:     "udp",
		Handler: mux,
		UDPSize: 65535,
	}
	d.tcpServer = &dns.Server{
		Addr:    d.ListenAddr,
		Net:     "tcp",
		Handler: mux,
	}

	errCh := make(chan error, 1)

	go func() {
		if err := d.udpServer.ListenAndServe(); err != nil {
			errCh <- fmt.Errorf("UDP DNS server error: %v", err)
		}
	}()

	go func() {
		if err := d.tcpServer.ListenAndServe(); err != nil {
			log.Printf("TCP DNS server error: %v", err)
		}
	}()

	start := time.Now()
	for {
		if time.Since(start) > 5*time.Second {
			return fmt.Errorf("DNS server did not start within 5 seconds")
		}

		if d.udpServer != nil && d.udpServer.PacketConn != nil {
			d.running = true
			log.Printf("DNS server started on %s", d.ListenAddr)
			return nil
		}

		time.Sleep(10 * time.Millisecond)
	}
}

func (d *DNS) handleDNS(w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(r)

	if len(r.Question) == 0 {
		m.Rcode = dns.RcodeFormatError
		w.WriteMsg(m)
		return
	}

	q := r.Question[0]
	name := strings.TrimSuffix(q.Name, ".")

	if q.Qtype != dns.TypeA {
		d.forwardOrFail(w, r, name)
		return
	}

	cacheKey := fmt.Sprintf("%s:%d", q.Name, q.Qtype)
	if cached, ok := d.getCache(cacheKey); ok {
		w.WriteMsg(cached)
		return
	}

	log.Printf("DNS query: %s", name)

	if d.Resolver != nil {
		if ip, ok := d.Resolver(name); ok {
			rr, err := dns.NewRR(fmt.Sprintf("%s IN A %s", q.Name, ip))
			if err == nil {
				m.Answer = append(m.Answer, rr)
				log.Printf("DNS resolved: %s -> %s", name, ip)
				d.setCache(cacheKey, m)
				w.WriteMsg(m)
				return
			}
		}
	}

	d.forwardOrFail(w, r, name)
}

func (d *DNS) forwardOrFail(w dns.ResponseWriter, r *dns.Msg, name string) {
	client := new(dns.Client)
	client.Timeout = 5 * time.Second

	resp, _, err := client.Exchange(r, "8.8.8.8:53")
	if err == nil && resp != nil {
		log.Printf("DNS forwarded: %s -> external resolver", name)
		d.setCache(fmt.Sprintf("%s:%d", r.Question[0].Name, r.Question[0].Qtype), resp)
		w.WriteMsg(resp)
		return
	}

	log.Printf("DNS failed: %s - %v", name, err)
	m := new(dns.Msg)
	m.SetReply(r)
	m.Rcode = dns.RcodeServerFailure
	w.WriteMsg(m)
}

func (d *DNS) getCache(key string) (*dns.Msg, bool) {
	d.cacheMu.RLock()
	defer d.cacheMu.RUnlock()

	entry, ok := d.cache[key]
	if !ok {
		return nil, false
	}

	if time.Now().After(entry.expiresAt) {
		go d.deleteCache(key)
		return nil, false
	}

	return entry.msg, true
}

func (d *DNS) setCache(key string, msg *dns.Msg) {
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()

	d.cache[key] = cacheEntry{
		msg:       msg.Copy(),
		expiresAt: time.Now().Add(d.cacheTTL),
	}
}

func (d *DNS) deleteCache(key string) {
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	delete(d.cache, key)
}

func (d *DNS) Stop() error {
	d.running = false
	if d.udpServer != nil {
		_ = d.udpServer.Shutdown()
	}
	if d.tcpServer != nil {
		_ = d.tcpServer.Shutdown()
	}
	return nil
}

func (d *DNS) IsRunning() bool {
	return d.running
}
