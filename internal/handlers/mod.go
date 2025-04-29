package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"net/http"
	"strconv"
	"time"

	"N0CTURNALBBS/internal/config"
	"N0CTURNALBBS/internal/models"

	"github.com/gin-gonic/gin"
	csrf "github.com/utrack/gin-csrf"
)

func ModLoginHandler(db *sql.DB, modConfig *config.ModeratorConfig, SiteConfig *config.SiteConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		GenerateCaptcha(c)
		captcha, _ := c.Get("captcha_problem")
		captchalevel, _ := c.Get("captcha_difficulty")
		if c.Request.Method == "GET" {
			c.HTML(http.StatusOK, "login.html", gin.H{
				"Site":         SiteConfig,
				"captcha":      captcha,
				"captchalevel": captchalevel,
				"csrfToken":    csrf.GetToken(c),
				"CurrentYear":  time.Now().Year(),
			})
			return
		}

		username := c.PostForm("username")
		password := c.PostForm("password")

		captchaAnswer := c.PostForm("captcha_answer")
		if username == "" || password == "" || captchaAnswer == "" {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"error":       "Content and security check are required",
				"Site":        SiteConfig,
				"CurrentYear": time.Now().Year(),
			})
			return
		}

		mod, err := models.AuthenticateModerator(db, username, password)
		if err != nil {
			c.HTML(http.StatusUnauthorized, "login.html", gin.H{
				"Site":        SiteConfig,
				"Error":       "Invalid credentials",
				"Username":    username,
				"csrfToken":   csrf.GetToken(c),
				"CurrentYear": time.Now().Year(),
			})
			return
		}

		sessionID, err := generateSessionID()
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"Site":        SiteConfig,
				"Error":       "Failed to create session",
				"csrfToken":   csrf.GetToken(c),
				"CurrentYear": time.Now().Year(),
			})
			return
		}

		duration := time.Duration(modConfig.Session.MaxAge) * time.Second

		_, err = models.CreateModSession(
			db,
			mod.ID,
			sessionID,
			c.ClientIP(),
			c.Request.UserAgent(),
			duration,
		)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"Site":        SiteConfig,
				"Error":       "Failed to create session",
				"csrfToken":   csrf.GetToken(c),
				"CurrentYear": time.Now().Year(),
			})
			return
		}

		c.SetCookie(
			modConfig.Session.CookieName,
			sessionID,
			modConfig.Session.MaxAge,
			"/",
			"",
			modConfig.Session.Secure,
			modConfig.Session.HTTPOnly,
		)

		c.Redirect(http.StatusFound, "/mod/dashboard")
	}
}

func ModLogoutHandler(db *sql.DB, modConfig *config.ModeratorConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID, err := c.Cookie(modConfig.Session.CookieName)
		if err == nil {
			models.DeleteSession(db, sessionID)
		}

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
	}
}

func ModDashboardHandler(db *sql.DB, SiteConfig *config.SiteConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		mod, exists := c.Get("moderator")
		if !exists {
			c.Redirect(http.StatusFound, "/mod/login")
			return
		}

		moderator := mod.(*models.Moderator)

		actions, err := models.GetModActions(db, 1, 10) // First page, 10 per page
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"Error":       err,
				"Site":        SiteConfig,
				"csrfToken":   csrf.GetToken(c),
				"CurrentYear": time.Now().Year(),
			})
			return
		}

		var threadCount, postCount int
		err = db.QueryRow("SELECT COUNT(*) FROM threads").Scan(&threadCount)
		if err != nil {
			threadCount = 0 // Default to 0 on error
		}

		err = db.QueryRow("SELECT COUNT(*) FROM posts").Scan(&postCount)
		if err != nil {
			postCount = 0 // Default to 0 on error
		}

		c.HTML(http.StatusOK, "dashboard.html", gin.H{
			"Site":        SiteConfig,
			"Moderator":   moderator,
			"Actions":     actions,
			"ThreadCount": threadCount,
			"PostCount":   postCount,
			"csrfToken":   csrf.GetToken(c),
			"CurrentYear": time.Now().Year(),
		})
	}
}

func ModThreadsHandler(db *sql.DB, SiteConfig *config.SiteConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
		if err != nil || page < 1 {
			page = 1
		}

		const threadsPerPage = 20

		boardSlug := c.Query("board")
		var boardID int
		var boardName string

		if boardSlug != "" {
			err := db.QueryRow("SELECT id, name FROM boards WHERE slug = $1", boardSlug).
				Scan(&boardID, &boardName)
			if err != nil && err != sql.ErrNoRows {
				c.HTML(http.StatusInternalServerError, "error.html", gin.H{
					"Error":       "Failed to load board information",
					"Site":        SiteConfig,
					"csrfToken":   csrf.GetToken(c),
					"CurrentYear": time.Now().Year(),
				})
				return
			}
		}

		var rows *sql.Rows
		var totalThreads int

		if boardID > 0 {
			err = db.QueryRow("SELECT COUNT(*) FROM threads WHERE board_id = $1", boardID).
				Scan(&totalThreads)
			if err != nil {
				c.HTML(http.StatusInternalServerError, "error.html", gin.H{
					"Error":       "Failed to count threads",
					"Site":        SiteConfig,
					"csrfToken":   csrf.GetToken(c),
					"CurrentYear": time.Now().Year(),
				})
				return
			}

			rows, err = db.Query(`
				SELECT t.id, t.title, t.post_count, t.created_at, t.last_post_at,
				       b.id AS board_id, b.name AS board_name, b.slug AS board_slug
				FROM threads t
				JOIN boards b ON t.board_id = b.id
				WHERE t.board_id = $1
				ORDER BY t.last_post_at DESC
				LIMIT $2 OFFSET $3
			`, boardID, threadsPerPage, (page-1)*threadsPerPage)
		} else {
			err = db.QueryRow("SELECT COUNT(*) FROM threads").Scan(&totalThreads)
			if err != nil {
				c.HTML(http.StatusInternalServerError, "error.html", gin.H{
					"Error":       "Failed to count threads",
					"Site":        SiteConfig,
					"csrfToken":   csrf.GetToken(c),
					"CurrentYear": time.Now().Year(),
				})
				return
			}

			rows, err = db.Query(`
				SELECT t.id, t.title, t.post_count, t.created_at, t.last_post_at,
				       b.id AS board_id, b.name AS board_name, b.slug AS board_slug
				FROM threads t
				JOIN boards b ON t.board_id = b.id
				ORDER BY t.last_post_at DESC
				LIMIT $1 OFFSET $2
			`, threadsPerPage, (page-1)*threadsPerPage)
		}

		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"Error":       "Failed to load threads",
				"Site":        SiteConfig,
				"csrfToken":   csrf.GetToken(c),
				"CurrentYear": time.Now().Year(),
			})
			return
		}
		defer rows.Close()

		type ThreadInfo struct {
			ID         int       `json:"id"`
			Title      string    `json:"title"`
			PostCount  int       `json:"post_count"`
			CreatedAt  time.Time `json:"created_at"`
			LastPostAt time.Time `json:"last_post_at"`
			BoardID    int       `json:"board_id"`
			BoardName  string    `json:"board_name"`
			BoardSlug  string    `json:"board_slug"`
		}

		var threads []ThreadInfo
		for rows.Next() {
			var t ThreadInfo
			err := rows.Scan(
				&t.ID, &t.Title, &t.PostCount, &t.CreatedAt, &t.LastPostAt,
				&t.BoardID, &t.BoardName, &t.BoardSlug,
			)
			if err != nil {
				c.HTML(http.StatusInternalServerError, "error.html", gin.H{
					"Error":       err,
					"Site":        SiteConfig,
					"csrfToken":   csrf.GetToken(c),
					"CurrentYear": time.Now().Year(),
				})
				return
			}
			threads = append(threads, t)
		}

		if err := rows.Err(); err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"Error":       "Error iterating threads",
				"Site":        SiteConfig,
				"csrfToken":   csrf.GetToken(c),
				"CurrentYear": time.Now().Year(),
			})
			return
		}

		rows, err = db.Query("SELECT id, name, slug FROM boards ORDER BY name")
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"Error":       "Failed to load boards",
				"Site":        SiteConfig,
				"csrfToken":   csrf.GetToken(c),
				"CurrentYear": time.Now().Year(),
			})
			return
		}
		defer rows.Close()

		type BoardInfo struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
			Slug string `json:"slug"`
		}

		var boards []BoardInfo
		for rows.Next() {
			var b BoardInfo
			if err := rows.Scan(&b.ID, &b.Name, &b.Slug); err != nil {
				c.HTML(http.StatusInternalServerError, "error.html", gin.H{
					"Error":       err,
					"Site":        SiteConfig,
					"csrfToken":   csrf.GetToken(c),
					"CurrentYear": time.Now().Year(),
				})
				return
			}
			boards = append(boards, b)
		}

		totalPages := (totalThreads + threadsPerPage - 1) / threadsPerPage
		pagination := struct {
			CurrentPage int
			TotalPages  int
			HasPrev     bool
			HasNext     bool
			PrevPage    int
			NextPage    int
			BoardSlug   string
		}{
			CurrentPage: page,
			TotalPages:  totalPages,
			HasPrev:     page > 1,
			HasNext:     page < totalPages,
			PrevPage:    page - 1,
			NextPage:    page + 1,
			BoardSlug:   boardSlug,
		}

		c.HTML(http.StatusOK, "mod_threads.html", gin.H{
			"Site":          SiteConfig,
			"Threads":       threads,
			"Boards":        boards,
			"Pagination":    pagination,
			"SelectedBoard": boardSlug,
			"BoardName":     boardName,
			"csrfToken":     csrf.GetToken(c),
			"CurrentYear":   time.Now().Year(),
		})
	}
}

func ModActionsHandler(db *sql.DB, SiteConfig *config.SiteConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
		if err != nil || page < 1 {
			page = 1
		}

		const actionsPerPage = 25

		var totalActions int
		err = db.QueryRow("SELECT COUNT(*) FROM mod_actions").Scan(&totalActions)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"Error":       "Failed to count moderation actions",
				"Site":        SiteConfig,
				"csrfToken":   csrf.GetToken(c),
				"CurrentYear": time.Now().Year(),
			})
			return
		}

		rows, err := db.Query(`
			SELECT ma.id, ma.moderator_id, ma.action_type, ma.target_id, ma.target_type, 
				   ma.executed_at, ma.ip_address, ma.reason, m.username
			FROM mod_actions ma
			JOIN moderators m ON ma.moderator_id = m.id
			ORDER BY ma.executed_at DESC
			LIMIT $1 OFFSET $2
		`, actionsPerPage, (page-1)*actionsPerPage)

		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"Error":       "Failed to load moderation actions",
				"Site":        SiteConfig,
				"csrfToken":   csrf.GetToken(c),
				"CurrentYear": time.Now().Year(),
			})
			return
		}
		defer rows.Close()

		type ActionInfo struct {
			ID          int            `json:"id"`
			ModeratorID int            `json:"moderator_id"`
			Username    string         `json:"username"`
			ActionType  string         `json:"action_type"`
			TargetID    int            `json:"target_id"`
			TargetType  string         `json:"target_type"`
			ExecutedAt  time.Time      `json:"executed_at"`
			IPAddress   string         `json:"ip_address"`
			Reason      sql.NullString `json:"reason"`
		}

		var actions []ActionInfo
		for rows.Next() {
			var action ActionInfo
			err := rows.Scan(
				&action.ID, &action.ModeratorID, &action.ActionType,
				&action.TargetID, &action.TargetType, &action.ExecutedAt,
				&action.IPAddress, &action.Reason, &action.Username,
			)
			if err != nil {
				c.HTML(http.StatusInternalServerError, "error.html", gin.H{
					"Error":       err,
					"Site":        SiteConfig,
					"csrfToken":   csrf.GetToken(c),
					"CurrentYear": time.Now().Year(),
				})
				return
			}
			actions = append(actions, action)
		}

		if err := rows.Err(); err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"Error":       "Error iterating actions",
				"Site":        SiteConfig,
				"csrfToken":   csrf.GetToken(c),
				"CurrentYear": time.Now().Year(),
			})
			return
		}

		totalPages := (totalActions + actionsPerPage - 1) / actionsPerPage
		pagination := struct {
			CurrentPage int
			TotalPages  int
			HasPrev     bool
			HasNext     bool
			PrevPage    int
			NextPage    int
		}{
			CurrentPage: page,
			TotalPages:  totalPages,
			HasPrev:     page > 1,
			HasNext:     page < totalPages,
			PrevPage:    page - 1,
			NextPage:    page + 1,
		}

		c.HTML(http.StatusOK, "mod_actions.html", gin.H{
			"Site":        SiteConfig,
			"Actions":     actions,
			"Pagination":  pagination,
			"csrfToken":   csrf.GetToken(c),
			"CurrentYear": time.Now().Year(),
		})
	}
}

func DeleteThreadHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		mod, exists := c.Get("moderator")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authorized"})
			return
		}
		moderator := mod.(*models.Moderator)

		threadID, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid thread ID"})
			return
		}

		reason := c.PostForm("reason")
		if reason == "" {
			reason = "No reason provided"
		}

		err = models.DeleteThread(db, threadID, moderator.ID, c.ClientIP(), reason)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "message": "Thread deleted successfully"})
	}
}

func ModPostsHandler(db *sql.DB, SiteConfig *config.SiteConfig) gin.HandlerFunc {
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

		var thread struct {
			ID        int
			Title     string
			BoardID   int
			BoardName string
			BoardSlug string
			CreatedAt time.Time
			PostCount int
			IsLocked  bool
		}

		err = db.QueryRow(`
    SELECT t.id, t.title, t.board_id, b.name, b.slug, 
           t.created_at, t.post_count, t.is_locked
    FROM threads t
    JOIN boards b ON t.board_id = b.id
    WHERE t.id = $1
`, threadID).Scan(
			&thread.ID, &thread.Title, &thread.BoardID, &thread.BoardName,
			&thread.BoardSlug, &thread.CreatedAt, &thread.PostCount, &thread.IsLocked,
		)

		if err != nil {
			if err == sql.ErrNoRows {
				c.HTML(http.StatusNotFound, "error.html", gin.H{
					"Error":       "Thread not found",
					"Site":        SiteConfig,
					"csrfToken":   csrf.GetToken(c),
					"CurrentYear": time.Now().Year(),
				})
				return
			}
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"Error":       "Failed to load thread information",
				"Site":        SiteConfig,
				"csrfToken":   csrf.GetToken(c),
				"CurrentYear": time.Now().Year(),
			})
			return
		}

		// Get posts
		rows, err := db.Query(`
			SELECT id, author, body, created_at, ip_hash
			FROM posts
			WHERE thread_id = $1
			ORDER BY id
		`, threadID)

		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"Error":       "Failed to load posts",
				"Site":        SiteConfig,
				"csrfToken":   csrf.GetToken(c),
				"CurrentYear": time.Now().Year(),
			})
			return
		}
		defer rows.Close()

		type PostInfo struct {
			ID        int
			Author    sql.NullString
			Body      string
			CreatedAt time.Time
			IPHash    sql.NullString
		}

		var posts []PostInfo
		for rows.Next() {
			var p PostInfo
			if err := rows.Scan(&p.ID, &p.Author, &p.Body, &p.CreatedAt, &p.IPHash); err != nil {
				c.HTML(http.StatusInternalServerError, "error.html", gin.H{
					"Error":       err,
					"Site":        SiteConfig,
					"csrfToken":   csrf.GetToken(c),
					"CurrentYear": time.Now().Year(),
				})
				return
			}
			posts = append(posts, p)
		}

		if err := rows.Err(); err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"Error":       "Error iterating posts",
				"Site":        SiteConfig,
				"csrfToken":   csrf.GetToken(c),
				"CurrentYear": time.Now().Year(),
			})
			return
		}

		c.HTML(http.StatusOK, "mod_posts.html", gin.H{
			"Site":        SiteConfig,
			"Thread":      thread,
			"Posts":       posts,
			"csrfToken":   csrf.GetToken(c),
			"CurrentYear": time.Now().Year(),
		})
	}
}

func DeletePostHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		mod, exists := c.Get("moderator")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authorized"})
			return
		}
		moderator := mod.(*models.Moderator)

		postID, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid post ID"})
			return
		}

		reason := c.PostForm("reason")
		if reason == "" {
			reason = "No reason provided"
		}

		err = models.DeletePost(db, postID, moderator.ID, c.ClientIP(), reason)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "message": "Post deleted successfully"})
	}
}

func generateSessionID() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
