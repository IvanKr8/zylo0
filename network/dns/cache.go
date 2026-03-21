package dns

import (
	"time"

	"github.com/miekg/dns"
)

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
