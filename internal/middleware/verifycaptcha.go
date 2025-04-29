package middleware

import (
	"N0CTURNALBBS/internal/config"
	"N0CTURNALBBS/internal/handlers"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func CaptchaRequired(SiteConfig *config.SiteConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == "GET" {
			handlers.GenerateCaptcha(c)
			c.Next()
			return
		}

		session := sessions.Default(c)

		expectedAnswer := session.Get("captcha_answer")
		if expectedAnswer == nil {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"Error":       "Captcha session expired. Please try again.",
				"Site":        SiteConfig,
				"CurrentYear": time.Now().Year(),
			})
			c.Abort()
			return
		}

		userAnswerStr := c.PostForm("captcha_answer")
		if userAnswerStr == "" {
			handlers.IncrementFailedAttempts(session)
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"Error":       "Captcha answer required",
				"Site":        SiteConfig,
				"CurrentYear": time.Now().Year(),
			})
			c.Abort()
			return
		}

		userAnswer, err := strconv.Atoi(userAnswerStr)
		if err != nil || userAnswer != expectedAnswer.(int) {
			handlers.IncrementFailedAttempts(session)
			handlers.GenerateCaptcha(c)
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"Error":       "Incorrect captcha answer. Please try again.",
				"Site":        SiteConfig,
				"CurrentYear": time.Now().Year(),
			})
			c.Abort()
			return
		}

		handlers.ResetFailedAttempts(session)
		session.Delete("captcha_answer")
		session.Save()

		c.Next()
	}
}
