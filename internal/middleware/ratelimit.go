package middleware

import (
	"encoding/json"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type cliente struct {
	limiter *rate.Limiter
	ultimo  time.Time
}

type RateLimiter struct {
	mu       sync.Mutex
	clientes map[string]*cliente
	rpm      int
	burst    int
	janela   time.Duration
}

func NovoRateLimiter(rpm int, burst int) *RateLimiter {
	rl := &RateLimiter{
		clientes: make(map[string]*cliente),
		rpm:      rpm,
		burst:    burst,
		janela:   10 * time.Minute,
	}
	go rl.limparInativos()
	return rl
}

func (rl *RateLimiter) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := obterIP(r)

		rl.mu.Lock()
		c, existe := rl.clientes[ip]
		if !existe {
			c = &cliente{
				limiter: rate.NewLimiter(rate.Limit(float64(rl.rpm)/60.0), rl.burst),
				ultimo:  time.Now(),
			}
			rl.clientes[ip] = c
		}
		c.ultimo = time.Now()
		rl.mu.Unlock()

		if !c.limiter.Allow() {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]string{
				"erro": "demasiadas tentativas. Tenta novamente em 1 minuto.",
			})
			return
		}

		next(w, r)
	}
}

func (rl *RateLimiter) limparInativos() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		for ip, c := range rl.clientes {
			if time.Since(c.ultimo) > rl.janela {
				delete(rl.clientes, ip)
			}
		}
		rl.mu.Unlock()
	}
}

func obterIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return xff[:i]
			}
		}
		return xff
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

func (rl *RateLimiter) Stats() int {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	return len(rl.clientes)
}
