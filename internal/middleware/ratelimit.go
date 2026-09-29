package middleware

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-redis/redis_rate/v10"
	"github.com/redis/go-redis/v9"
)

type RateLimiter struct {
	limiter *redis_rate.Limiter
	rate    redis_rate.Limit
	nome    string
}

func NovoRateLimiter(redisClient *redis.Client, nome string, rpm int, burst int) *RateLimiter {
	limiter := redis_rate.NewLimiter(redisClient)

	return &RateLimiter{
		limiter: limiter,
		nome:    nome,
		rate: redis_rate.Limit{
			Rate:   rpm,
			Burst:  burst,
			Period: time.Minute,
		},
	}
}

func (rl *RateLimiter) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := obterIP(r)
		chave := rl.nome + ":" + ip

		res, err := rl.limiter.Allow(r.Context(), chave, rl.rate)
		if err != nil {
			log.Printf("⚠️  Rate limiter (Redis) falhou: %v", err)
			next(w, r)
			return
		}

		if res.Allowed == 0 {
			retryAfter := int(res.RetryAfter.Seconds())
			if retryAfter < 1 {
				retryAfter = 1
			}

			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", itoa(retryAfter))
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"erro":        "demasiadas tentativas",
				"retry_after": retryAfter,
				"mensagem":    "Tenta novamente em " + itoa(retryAfter) + " segundos",
			})
			return
		}

		w.Header().Set("X-RateLimit-Remaining", itoa(res.Remaining))
		next(w, r)
	}
}

func obterIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return strings.TrimSpace(xff[:i])
			}
		}
		return strings.TrimSpace(xff)
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

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func NovoClienteRedis() (*redis.Client, error) {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379/0"
	}

	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(opt)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return client, nil
}
