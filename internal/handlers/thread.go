package handlers

import (
	"N0CTURNALBBS/internal/config"
	"N0CTURNALBBS/internal/models"
	"database/sql"
	"fmt"
	"html"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	csrf "github.com/utrack/gin-csrf"
)

func ThreadHandler(db *sql.DB, SiteConfig *config.SiteConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		threadID, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"Error":       "Invalid thread ID",
				"Site":        SiteConfig,
				"CurrentYear": time.Now().Year(),
			})
			return
		}

		GenerateCaptcha(c)
		captcha, _ := c.Get("captcha_problem")
		captchalevel, _ := c.Get("captcha_difficulty")

		thread, err := models.GetThreadByID(db, threadID)
		if err != nil {
			if err == sql.ErrNoRows {
				c.HTML(http.StatusNotFound, "error.html", gin.H{
					"Error":       "Thread not found",
					"Site":        SiteConfig,
					"CurrentYear": time.Now().Year(),
				})
				return
			}

			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"Error":       "Failed to load thread",
				"Site":        SiteConfig,
				"CurrentYear": time.Now().Year(),
			})
			return
		}

		board, err := models.GetBoardByID(db, thread.BoardID)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"Error":       "Failed to load board information",
				"Site":        SiteConfig,
				"CurrentYear": time.Now().Year(),
			})
			return
		}

		posts, err := models.GetPostsByThreadID(db, threadID)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"Error":       "Failed to load posts",
				"Site":        SiteConfig,
				"CurrentYear": time.Now().Year(),
			})
			return
		}

		boards, err := models.GetAllBoards(db)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"Error":       "Failed to load boards",
				"Site":        SiteConfig,
				"CurrentYear": time.Now().Year(),
			})
			return
		}

		c.HTML(http.StatusOK, "thread.html", gin.H{
			"Site":            SiteConfig,
			"Title":           thread.Title,
			"Board":           board,
			"Boards":          boards,
			"Thread":          thread,
			"maxAuthorLength": maxAuthorLength,
			"minBodyLength":   minBodyLength,
			"maxBodyLength":   maxBodyLength,
			"captcha":         captcha,
			"captchalevel":    captchalevel,
			"Posts":           posts,
			"csrfToken":       csrf.GetToken(c),
			"CurrentYear":     time.Now().Year(),
		})
	}
}

func NewPostHandler(db *sql.DB, SiteConfig *config.SiteConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		threadID, err := strconv.Atoi(c.Param("thread_id"))
		if err != nil {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"Error": "Invalid thread ID",
				"Site":  SiteConfig,
			})
			return
		}

		thread, err := models.GetThreadForUpdate(db, threadID)
		if err != nil {
			if err == sql.ErrNoRows {
				c.HTML(http.StatusNotFound, "error.html", gin.H{
					"Error": "Thread not found",
					"Site":  SiteConfig,
				})
				return
			}

			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"Error": "Failed to load thread",
				"Site":  SiteConfig,
			})
			return
		}

		if thread.IsLocked {
			c.HTML(http.StatusForbidden, "error.html", gin.H{
				"Error": "This thread is locked. No new replies can be posted.",
				"Site":  SiteConfig,
			})
			return
		}

		honeypot1 := strings.TrimSpace(c.PostForm("contact_preference"))
		honeypot2 := strings.TrimSpace(c.PostForm("confirm_address"))

		if honeypot1 != "" || honeypot2 != "" {
			log.Printf("Bot detected - Honeypot fields filled. IP: %s", c.ClientIP())

			c.HTML(http.StatusForbidden, "error.html", gin.H{
				"Error": "Invalid request detected.",
				"Site":  SiteConfig,
			})
			return
		}

		author := html.EscapeString(c.PostForm("author"))
		body := PostProcessor(c.PostForm("body"))

		if body == "" {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"Error": "Post body is required",
				"Site":  SiteConfig,
			})
			return
		}

		var errorMessage string
		if len(author) > maxAuthorLength {
			errorMessage = fmt.Sprintf("Name cannot exceed %d characters", maxAuthorLength)
		} else if len(body) < minBodyLength {
			errorMessage = fmt.Sprintf("Message must be at least %d characters", minBodyLength)
		} else if len(body) > maxBodyLength {
			errorMessage = fmt.Sprintf("Message cannot exceed %d characters", maxBodyLength)
		}

		if errorMessage != "" {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"Error":       errorMessage,
				"Site":        SiteConfig,
				"CurrentYear": time.Now().Year(),
			})
			return
		}

		hasBanned, err := models.CheckForBannedWords(db, author, body)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"Error":       "Internal server error",
				"Site":        SiteConfig,
				"CurrentYear": time.Now().Year(),
			})
			return
		}

		if hasBanned {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"Error":       "Submission contains prohibited content",
				"Site":        SiteConfig,
				"CurrentYear": time.Now().Year(),
			})
			return
		}

		ipAddress := c.ClientIP()

		err = models.CreatePost(db, threadID, author, body, ipAddress)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"Error": "Failed to create post",
				"Site":  SiteConfig,
			})
			return
		}

		c.Redirect(http.StatusSeeOther, "/thread/"+strconv.Itoa(threadID)+"#bottom")
	}
}
