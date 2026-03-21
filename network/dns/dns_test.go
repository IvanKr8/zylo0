package dns

import (
	"testing"
	"time"

	"github.com/miekg/dns"
)

func TestDNSFullCycle(t *testing.T) {
	d := NewDNS("127.0.0.1:1055", func(name string) (string, bool) {
		if name == "testcontainer" {
			return "10.50.0.2", true
		}
		return "", false
	})
	defer d.Stop()

	go func() {
		if err := d.Start(); err != nil {
			t.Fatalf("failed to start DNS server: %v", err)
		}
	}()

	// Ждём пока сервер реально стартует, максимум 5 секунд
	start := time.Now()
	for {
		if d.IsRunning() {
			break
		}
		if time.Since(start) > 5*time.Second {
			t.Fatal("DNS server failed to start within 5 seconds")
		}
		time.Sleep(50 * time.Millisecond)
	}

	c := new(dns.Client)
	m := new(dns.Msg)
	m.SetQuestion("testcontainer.", dns.TypeA)

	r, _, err := c.Exchange(m, "127.0.0.1:1055")
	if err != nil {
		t.Fatalf("DNS query failed: %v", err)
	}

	if len(r.Answer) == 0 {
		t.Fatal("no answer received")
	}

	a, ok := r.Answer[0].(*dns.A)
	if !ok {
		t.Fatal("answer is not an A record")
	}

	if a.A.String() != "10.50.0.2" {
		t.Fatalf("unexpected IP: %v", a.A.String())
	}
}
