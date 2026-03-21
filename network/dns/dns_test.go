package dns

import (
	"net"
	"testing"
	"time"

	"github.com/miekg/dns"
)

func TestDNSStartStop(t *testing.T) {
	d := NewDNS("127.0.0.1:1055", nil)

	if err := d.Start(); err != nil {
		t.Fatalf("failed to start DNS: %v", err)
	}
	defer d.Stop()

	if !d.IsRunning() {
		t.Fatal("DNS server not running after Start()")
	}

	conn, err := net.Dial("udp", "127.0.0.1:1055")
	if err != nil {
		t.Fatalf("cannot connect to DNS server: %v", err)
	}
	conn.Close()
}

func TestDNSLocalResolution(t *testing.T) {
	resolver := func(name string) (string, bool) {
		switch name {
		case "postgres":
			return "10.20.0.2", true
		case "redis":
			return "10.20.0.3", true
		default:
			return "", false
		}
	}

	d := NewDNS("127.0.0.1:1056", resolver)
	if err := d.Start(); err != nil {
		t.Fatalf("failed to start DNS: %v", err)
	}
	defer d.Stop()

	waitForServer(t, d)

	tests := []struct {
		name     string
		expected string
	}{
		{"postgres", "10.20.0.2"},
		{"redis", "10.20.0.3"},
	}

	c := new(dns.Client)
	for _, tt := range tests {
		m := new(dns.Msg)
		m.SetQuestion(tt.name+".", dns.TypeA)

		r, _, err := c.Exchange(m, "127.0.0.1:1056")
		if err != nil {
			t.Errorf("query %s failed: %v", tt.name, err)
			continue
		}

		if len(r.Answer) == 0 {
			t.Errorf("no answer for %s", tt.name)
			continue
		}

		a, ok := r.Answer[0].(*dns.A)
		if !ok {
			t.Errorf("answer for %s is not A record", tt.name)
			continue
		}

		if a.A.String() != tt.expected {
			t.Errorf("%s: expected %s, got %s", tt.name, tt.expected, a.A.String())
		}
	}
}

func TestDNSExternalForwarding(t *testing.T) {
	d := NewDNS("127.0.0.1:1057", func(name string) (string, bool) {
		return "", false
	})
	if err := d.Start(); err != nil {
		t.Fatalf("failed to start DNS: %v", err)
	}
	defer d.Stop()

	waitForServer(t, d)

	m := new(dns.Msg)
	m.SetQuestion("google.com.", dns.TypeA)

	c := new(dns.Client)
	r, _, err := c.Exchange(m, "127.0.0.1:1057")
	if err != nil {
		t.Fatalf("external query failed: %v", err)
	}

	if len(r.Answer) == 0 {
		t.Fatal("no answer from external DNS")
	}
}

func TestDNSCache(t *testing.T) {
	called := 0
	resolver := func(name string) (string, bool) {
		called++
		if name == "cached" {
			return "10.20.0.100", true
		}
		return "", false
	}

	d := NewDNS("127.0.0.1:1058", resolver)
	d.cacheTTL = 2 * time.Second
	if err := d.Start(); err != nil {
		t.Fatalf("failed to start DNS: %v", err)
	}
	defer d.Stop()

	waitForServer(t, d)

	c := new(dns.Client)
	m := new(dns.Msg)
	m.SetQuestion("cached.", dns.TypeA)

	// First query should hit resolver
	if _, _, err := c.Exchange(m, "127.0.0.1:1058"); err != nil {
		t.Fatalf("first query failed: %v", err)
	}
	if called != 1 {
		t.Errorf("expected resolver called once, got %d", called)
	}

	// Second query should hit cache
	if _, _, err := c.Exchange(m, "127.0.0.1:1058"); err != nil {
		t.Fatalf("second query failed: %v", err)
	}
	if called != 1 {
		t.Errorf("resolver called again, cache not working")
	}

	// Wait for cache expiration
	time.Sleep(3 * time.Second)

	if _, _, err := c.Exchange(m, "127.0.0.1:1058"); err != nil {
		t.Fatalf("query after expiration failed: %v", err)
	}
	if called != 2 {
		t.Errorf("expected resolver called after expiration, got %d", called)
	}
}

func TestDNSUnknownQuery(t *testing.T) {
	d := NewDNS("127.0.0.1:1059", func(name string) (string, bool) {
		return "", false
	})
	if err := d.Start(); err != nil {
		t.Fatalf("failed to start DNS: %v", err)
	}
	defer d.Stop()

	waitForServer(t, d)

	m := new(dns.Msg)
	m.SetQuestion("nonexistent.domain.that.does.not.exist.", dns.TypeA)

	c := new(dns.Client)
	r, _, err := c.Exchange(m, "127.0.0.1:1059")
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if r.Rcode != dns.RcodeNameError && r.Rcode != dns.RcodeServerFailure {
		t.Errorf("expected NXDOMAIN or SERVFAIL, got %d", r.Rcode)
	}
}

func TestDNSNonARecord(t *testing.T) {
	d := NewDNS("127.0.0.1:1060", nil)
	if err := d.Start(); err != nil {
		t.Fatalf("failed to start DNS: %v", err)
	}
	defer d.Stop()

	waitForServer(t, d)

	m := new(dns.Msg)
	m.SetQuestion("google.com.", dns.TypeMX)

	c := new(dns.Client)
	r, _, err := c.Exchange(m, "127.0.0.1:1060")
	if err != nil {
		t.Fatalf("MX query failed: %v", err)
	}

	// Should be forwarded to external DNS
	if len(r.Answer) == 0 && r.Rcode != dns.RcodeSuccess {
		t.Error("MX query should return answer or success")
	}
}

func waitForServer(t *testing.T, d *DNS) {
	start := time.Now()
	for {
		if d.IsRunning() {
			return
		}
		if time.Since(start) > 5*time.Second {
			t.Fatal("DNS server failed to start within timeout")
		}
		time.Sleep(50 * time.Millisecond)
	}
}
