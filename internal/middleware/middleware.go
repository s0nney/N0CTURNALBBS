package middleware

import (
	"N0CTURNALBBS/internal/config"
	"fmt"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		latency := time.Since(start)

		statusCode := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method

		fmt.Printf("[%s] %s %s %d %s %s\n",
			time.Now().Format("2006-01-02 15:04:05"),
			method,
			path,
			statusCode,
			latency,
			clientIP,
		)
	}
}

func RateLimiter(rateConfig *config.RateLimitConfig, SiteConfig *config.SiteConfig) gin.HandlerFunc {
	type ipData struct {
		count    int
		lastSeen time.Time
	}

	var (
		ipMap = make(map[string]*ipData)
		mu    sync.Mutex
	)

	windowLength := time.Duration(rateConfig.WindowSeconds) * time.Second
	cleanupEvery := time.Duration(rateConfig.CleanupSeconds) * time.Second

	// Cleanup goroutine
	go func() {
		ticker := time.NewTicker(cleanupEvery)
		for range ticker.C {
			mu.Lock()
			now := time.Now()
			for ip, data := range ipMap {
				if now.Sub(data.lastSeen) > windowLength {
					delete(ipMap, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return func(c *gin.Context) {
		ip := c.ClientIP()

		mu.Lock()
		data, exists := ipMap[ip]
		now := time.Now()

		if !exists {
			ipMap[ip] = &ipData{count: 1, lastSeen: now}
			mu.Unlock()
			c.Next()
			return
		}

		if now.Sub(data.lastSeen) > windowLength {
			data.count = 1
			data.lastSeen = now
			mu.Unlock()
			c.Next()
			return
		}

		data.count++
		data.lastSeen = now

		if data.count > rateConfig.MaxRequests {
			mu.Unlock()

			if rateConfig.EnableJSONResponse {
				for k, v := range rateConfig.ErrorResponse.Headers {
					c.Header(k, v)
				}

				retryAfter := int(windowLength.Seconds() - now.Sub(data.lastSeen).Seconds())
				if retryAfter < 0 {
					retryAfter = 0
				}

				c.JSON(rateConfig.ErrorResponse.Code, gin.H{
					"error":         "RATE_LIMIT_EXCEEDED",
					"message":       rateConfig.ErrorMessage,
					"retry_after":   retryAfter,
					"rate_limit":    rateConfig.MaxRequests,
					"window_length": rateConfig.WindowSeconds,
				})
			} else {
				// Fallback to HTML
				c.HTML(rateConfig.ErrorResponse.Code, "error.html", gin.H{
					"Error": rateConfig.ErrorMessage,
					"Site":  SiteConfig,
				})
			}

			c.Abort()
			return
		}

		mu.Unlock()
		c.Next()
	}
}

func TemplateHelpers() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("formatTime", func(t time.Time) string {
			return t.Format("Jan 02, 2006 15:04:05")
		})

		c.Set("formatBody", func(body string) string {
			return body
		})

		c.Next()
	}
}
