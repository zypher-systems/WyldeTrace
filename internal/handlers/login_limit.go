package handlers

import (
	"net"
	"net/http"
	"sync"
	"time"
)

type loginAttempt struct {
	fails []time.Time
}

var loginLimiter = struct {
	mu   sync.Mutex
	byIP map[string]*loginAttempt
}{byIP: map[string]*loginAttempt{}}

const (
	loginMaxFails = 8
	loginWindow   = 15 * time.Minute
)

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func loginAllowed(ip string) bool {
	loginLimiter.mu.Lock()
	defer loginLimiter.mu.Unlock()
	a := loginLimiter.byIP[ip]
	if a == nil {
		return true
	}
	cutoff := time.Now().Add(-loginWindow)
	kept := a.fails[:0]
	for _, t := range a.fails {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	a.fails = kept
	return len(a.fails) < loginMaxFails
}

func loginFail(ip string) {
	loginLimiter.mu.Lock()
	defer loginLimiter.mu.Unlock()
	a := loginLimiter.byIP[ip]
	if a == nil {
		a = &loginAttempt{}
		loginLimiter.byIP[ip] = a
	}
	a.fails = append(a.fails, time.Now())
}

func loginSuccess(ip string) {
	loginLimiter.mu.Lock()
	defer loginLimiter.mu.Unlock()
	delete(loginLimiter.byIP, ip)
}
