package dns

import (
	"fmt"
	"strings"
	"time"

	"github.com/miekg/dns"
)

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

	// Non-A records (AAAA, MX) go straight to forwarder
	if q.Qtype != dns.TypeA {
		d.forwardOrFail(w, r, name)
		return
	}

	cacheKey := fmt.Sprintf("%s:%d", q.Name, q.Qtype)
	if cached, ok := d.getCache(cacheKey); ok {
		w.WriteMsg(cached)
		return
	}

	// Local resolution: container hostnames
	if d.Resolver != nil {
		if ip, ok := d.Resolver(name); ok {
			rr, err := dns.NewRR(fmt.Sprintf("%s IN A %s", q.Name, ip))
			if err == nil {
				m.Answer = append(m.Answer, rr)
				d.setCache(cacheKey, m)
				w.WriteMsg(m)
				return
			}
		}
	}

	d.forwardOrFail(w, r, name)
}

func (d *DNS) forwardOrFail(w dns.ResponseWriter, r *dns.Msg, name string) {
	client := &dns.Client{Timeout: 5 * time.Second}

	resp, _, err := client.Exchange(r, externalDNS)
	if err == nil && resp != nil {
		d.setCache(fmt.Sprintf("%s:%d", r.Question[0].Name, r.Question[0].Qtype), resp)
		w.WriteMsg(resp)
		return
	}

	m := new(dns.Msg)
	m.SetReply(r)
	m.Rcode = dns.RcodeServerFailure
	w.WriteMsg(m)
}
