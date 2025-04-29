package handlers

import (
	"N0CTURNALBBS/internal/models"
	"database/sql"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func LockThreadHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		mod, _ := c.Get("moderator")
		moderator := mod.(*models.Moderator)

		threadID, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid thread ID"})
			return
		}

		var thread models.Thread
		if err := db.QueryRow("SELECT is_locked FROM threads WHERE id = $1", threadID).
			Scan(&thread.IsLocked); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Thread not found"})
			return
		}

		newStatus := !thread.IsLocked
		_, err = db.Exec("UPDATE threads SET is_locked = $1 WHERE id = $2", newStatus, threadID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update thread"})
			return
		}

		action := "unlock_thread"
		if newStatus {
			action = "lock_thread"
		}

		models.LogModAction(db, moderator.ID, action, threadID, "thread", c.ClientIP(), "")

		c.Redirect(http.StatusSeeOther, fmt.Sprintf("/mod/threads/%d/posts", threadID))
	}
}
