package handlers

import (
	"N0CTURNALBBS/internal/config"
	"N0CTURNALBBS/internal/models"
	"database/sql"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	csrf "github.com/utrack/gin-csrf"
)

func ModWordFiltersHandler(db *sql.DB, SiteConfig *config.SiteConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		filters, err := models.GetAllBannedWords(db)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"Error": "Failed to load filters",
				"Site":  SiteConfig,
			})
			return
		}

		c.HTML(http.StatusOK, "mod_filters.html", gin.H{
			"Site":        SiteConfig,
			"Filters":     filters,
			"csrfToken":   csrf.GetToken(c),
			"CurrentYear": time.Now().Year(),
		})
	}
}

func ModAddBannedWordHandler(db *sql.DB, SiteConfig *config.SiteConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		mod, _ := c.Get("moderator")
		word := strings.TrimSpace(c.PostForm("word"))

		if word == "" {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{
				"Error": "Word cannot be empty",
				"Site":  SiteConfig,
			})
			return
		}

		err := models.AddBannedWord(db, word, mod.(*models.Moderator).ID, c.ClientIP())
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"Error": "Error",
				"Site":  SiteConfig,
			})
			return
		}

		c.Redirect(http.StatusSeeOther, "/mod/filters")
	}
}

func ModDeleteBannedWordHandler(db *sql.DB, SiteConfig *config.SiteConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		mod, _ := c.Get("moderator")
		wordID := c.Param("id")

		err := models.DeleteBannedWord(db, wordID, mod.(*models.Moderator).ID, c.ClientIP())
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"Error": "Failed to delete word filter",
				"Site":  SiteConfig,
			})
			return
		}

		c.Redirect(http.StatusSeeOther, "/mod/filters")
	}
}
