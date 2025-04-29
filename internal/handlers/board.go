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

const (
	maxTitleLength  = 100
	minTitleLength  = 5
	maxAuthorLength = 90 // Name maxlength
	maxBodyLength   = 5000
	minBodyLength   = 5
)

const threadsPerPage = 20 // Pagination

func HomeHandler(db *sql.DB, SiteConfig *config.SiteConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		boards, err := models.GetAllBoards(db)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"Error": "Failed to load boards",
				"Site":  SiteConfig,
			})
			return
		}

		c.HTML(http.StatusOK, "index.html", gin.H{
			"Site":        SiteConfig,
			"Boards":      boards,
			"csrfToken":   csrf.GetToken(c),
			"CurrentYear": time.Now().Year(),
		})
	}
}

func BoardHandler(db *sql.DB, SiteConfig *config.SiteConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		slug := c.Param("slug")
		GenerateCaptcha(c)
		captcha, _ := c.Get("captcha_problem")
		captchalevel, _ := c.Get("captcha_difficulty")

		page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
		if err != nil || page < 1 {
			page = 1
		}

		board, err := models.GetBoardBySlug(db, slug)
		if err != nil {
			if err == sql.ErrNoRows {
				c.HTML(http.StatusNotFound, "error.html", gin.H{
					"Error": "Board not found",
					"Site":  SiteConfig,
				})
				return
			}
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"Error": "Failed to load board",
				"Site":  SiteConfig,
			})
			return
		}

		totalThreads, err := models.CountThreadsByBoardID(db, board.ID)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"Error": "Failed to count threads",
				"Site":  SiteConfig,
			})
			return
		}

		totalPages := (totalThreads + threadsPerPage - 1) / threadsPerPage

		if page > totalPages && totalPages > 0 {
			redirectURL := fmt.Sprintf("/board/%s?page=1", slug)
			c.Redirect(http.StatusSeeOther, redirectURL)
			c.Abort()
			return
		}

		threads, err := models.GetThreadsByBoardID(db, board.ID, page, threadsPerPage)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"Error": "Failed to load threads",
				"Site":  SiteConfig,
			})
			return
		}

		pagination := models.Pagination{
			CurrentPage: page,
			TotalPages:  totalPages,
			HasPrev:     page > 1,
			HasNext:     page < totalPages,
			PrevPage:    page - 1,
			NextPage:    page + 1,
		}

		boards, err := models.GetAllBoards(db)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"Error": "Failed to load boards",
				"Site":  SiteConfig,
			})
			return
		}

		c.HTML(http.StatusOK, "board.html", gin.H{
			"Site":            SiteConfig,
			"Board":           board,
			"Boards":          boards,
			"Threads":         threads,
			"minTitleLength":  minTitleLength,
			"minBodyLength":   minBodyLength,
			"maxAuthorLength": maxAuthorLength,
			"maxTitleLength":  maxTitleLength,
			"maxBodyLength":   maxBodyLength,
			"captcha":         captcha,
			"captchalevel":    captchalevel,
			"Pagination":      pagination,
			"csrfToken":       csrf.GetToken(c),
			"CurrentYear":     time.Now().Year(),
		})
	}
}

func NewThreadHandler(db *sql.DB, SiteConfig *config.SiteConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		boardID, err := strconv.Atoi(c.Param("board_id"))
		if err != nil {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"Error":       "Invalid board ID",
				"Site":        SiteConfig,
				"CurrentYear": time.Now().Year(),
			})
			return
		}

		_, err = models.GetBoardByID(db, boardID)
		if err != nil {
			if err == sql.ErrNoRows {
				c.HTML(http.StatusNotFound, "error.html", gin.H{
					"Error":       "Board not found",
					"Site":        SiteConfig,
					"CurrentYear": time.Now().Year(),
				})
				return
			}

			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"Error":       "Failed to load board",
				"Site":        SiteConfig,
				"CurrentYear": time.Now().Year(),
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

		title := html.EscapeString(c.PostForm("title"))
		author := html.EscapeString(c.PostForm("author"))
		if author == "" || len(strings.TrimSpace(author)) == 0 {
			author = "Anonymous"
		}

		body := PostProcessor(c.PostForm("body"))

		hasBanned, err := models.CheckForBannedWords(db, title, author, body)
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

		captchaAnswer := c.PostForm("captcha_answer")
		if title == "" || body == "" || captchaAnswer == "" {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"error":       "Content and security check are required",
				"Site":        SiteConfig,
				"CurrentYear": time.Now().Year(),
			})
			return
		}

		if title == "" || body == "" {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"Error":       "Title and body are required",
				"Site":        SiteConfig,
				"CurrentYear": time.Now().Year(),
			})
			return
		}

		var errorMessage string
		if len(title) < minTitleLength {
			errorMessage = fmt.Sprintf("Title must be at least %d characters", minTitleLength)
		} else if len(title) > maxTitleLength {
			errorMessage = fmt.Sprintf("Title cannot exceed %d characters", maxTitleLength)
		} else if len(author) > maxAuthorLength {
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

		var boardIsLocked bool
		err = db.QueryRow("SELECT locked FROM boards WHERE id = $1", boardID).Scan(&boardIsLocked)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"Error":       "Error",
				"Site":        SiteConfig,
				"CurrentYear": time.Now().Year(),
			})
			return
		}

		if boardIsLocked {
			c.HTML(http.StatusForbidden, "error.html", gin.H{
				"Error":       "This board is locked. No new threads can be created.",
				"Site":        SiteConfig,
				"CurrentYear": time.Now().Year(),
			})
			return
		}

		ipAddress := c.ClientIP()

		threadID, err := models.CreateThread(db, boardID, title, author, body, ipAddress)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"Error":       "Failed to create thread",
				"Site":        SiteConfig,
				"CurrentYear": time.Now().Year(),
			})
			return
		}

		c.Redirect(http.StatusSeeOther, "/thread/"+strconv.Itoa(threadID)+"#bottom")
	}
}
