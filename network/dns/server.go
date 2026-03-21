package dns

import (
	"fmt"
	"time"

	"github.com/miekg/dns"
)

func NewDNS(listenAddr string, resolver func(string) (string, bool)) *DNS {
	return &DNS{
		ListenAddr: listenAddr,
		Resolver:   resolver,
		cache:      make(map[string]cacheEntry),
		cacheTTL:   defaultCacheTTL,
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

	// Run servers in background
	go func() {
		if err := d.udpServer.ListenAndServe(); err != nil {
			fmt.Printf("DNS UDP error on %s: %v\n", d.ListenAddr, err)
		}
	}()

	go func() {
		if err := d.tcpServer.ListenAndServe(); err != nil {
			fmt.Printf("DNS TCP error on %s: %v\n", d.ListenAddr, err)
		}
	}()

	start := time.Now()
	for {
		if time.Since(start) > startTimeout {
			return fmt.Errorf("DNS server failed to start on %s", d.ListenAddr)
		}

		if d.udpServer != nil && d.udpServer.PacketConn != nil {
			d.running = true
			return nil
		}

		time.Sleep(10 * time.Millisecond)
	}
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
