package middleware

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"net/http"
	"time"

	"N0CTURNALBBS/internal/config"
	"N0CTURNALBBS/internal/models"

	"github.com/gin-gonic/gin"
	csrf "github.com/utrack/gin-csrf"
)

func AuthRequired(db *sql.DB, modConfig *config.ModeratorConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionCookie, err := c.Cookie(modConfig.Session.CookieName)
		if err != nil {
			c.Redirect(http.StatusFound, "/mod/login")
			c.Abort()
			return
		}

		session, err := models.GetSessionByID(db, sessionCookie)
		if err != nil {
			c.SetCookie(
				modConfig.Session.CookieName,
				"",
				-1,
				"/",
				"",
				modConfig.Session.Secure,
				modConfig.Session.HTTPOnly,
			)
			c.Redirect(http.StatusFound, "/mod/login")
			c.Abort()
			return
		}

		mod, err := models.GetModeratorByID(db, session.ModeratorID)
		if err != nil || !mod.IsActive {
			c.SetCookie(
				modConfig.Session.CookieName,
				"",
				-1,
				"/",
				"",
				modConfig.Session.Secure,
				modConfig.Session.HTTPOnly,
			)
			c.Redirect(http.StatusFound, "/mod/login")
			c.Abort()
			return
		}

		c.Set("moderator", mod)
		c.Set("session", session)
		c.Next()
	}
}

func CSRFProtection(SiteConfig *config.SiteConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == "GET" {
			c.Next()
			return
		}

		token := c.Request.Header.Get("X-CSRF-Token")
		expectedToken, exists := c.Get("csrf_token")

		if !exists || token != expectedToken.(string) {
			c.HTML(http.StatusForbidden, "error.html", gin.H{
				"Site":        SiteConfig,
				"Error":       "Invalid or missing CSRF token. Please try again.",
				"csrfToken":   csrf.GetToken(c),
				"CurrentYear": time.Now().Year(),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

func GenerateCSRFToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := generateRandomToken(32)
		c.Set("csrf_token", token)
		c.Next()
	}
}

func generateRandomToken(length int) string {
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		return "fallback_token_error_reading_random"
	}
	return base64.URLEncoding.EncodeToString(b)[:length]
}
