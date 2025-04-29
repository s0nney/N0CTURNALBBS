package models

import (
	"database/sql"
	"errors"
	"log"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type Moderator struct {
	ID           int        `json:"id"`
	Username     string     `json:"username"`
	PasswordHash string     `json:"-"`
	CreatedAt    time.Time  `json:"created_at"`
	LastLogin    *time.Time `json:"last_login,omitempty"`
	IsActive     bool       `json:"is_active"`
}

type Session struct {
	ID          string    `json:"id"`
	ModeratorID int       `json:"moderator_id"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	IPAddress   string    `json:"ip_address"`
	UserAgent   string    `json:"user_agent"`
}

type ModAction struct {
	ID          int            `json:"id"`
	ModeratorID int            `json:"moderator_id"`
	ActionType  string         `json:"action_type"`
	TargetID    int            `json:"target_id"`
	TargetType  string         `json:"target_type"`
	ExecutedAt  time.Time      `json:"executed_at"`
	IPAddress   string         `json:"ip_address"`
	Reason      sql.NullString `json:"reason"`
}

func AuthenticateModerator(db *sql.DB, username, password string) (*Moderator, error) {
	var mod Moderator
	var passwordHash string
	var lastLogin sql.NullTime

	err := db.QueryRow(`
        SELECT id, username, password_hash, created_at, last_login, is_active
        FROM moderators
        WHERE username = $1 AND is_active = true`,
		username,
	).Scan(&mod.ID, &mod.Username, &passwordHash, &mod.CreatedAt, &lastLogin, &mod.IsActive)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("invalid credentials")
		}
		return nil, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password))
	if err != nil {
		return nil, errors.New("invalid credentials")
	}

	if lastLogin.Valid {
		mod.LastLogin = &lastLogin.Time
	}

	now := time.Now()
	mod.LastLogin = &now
	_, err = db.Exec("UPDATE moderators SET last_login = $1 WHERE id = $2", now, mod.ID)
	if err != nil {
		log.Printf("Failed to update last login time: %v", err)
	}

	return &mod, nil
}

func CreateModSession(db *sql.DB, modID int, sessionID, ipAddress, userAgent string, duration time.Duration) (*Session, error) {
	expiresAt := time.Now().Add(duration)

	session := Session{
		ID:          sessionID,
		ModeratorID: modID,
		CreatedAt:   time.Now(),
		ExpiresAt:   expiresAt,
		IPAddress:   ipAddress,
		UserAgent:   userAgent,
	}

	_, err := db.Exec(`
		INSERT INTO mod_sessions (id, moderator_id, created_at, expires_at, ip_address, user_agent)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		session.ID, session.ModeratorID, session.CreatedAt, session.ExpiresAt,
		generateIPHash(session.IPAddress), session.UserAgent,
	)

	if err != nil {
		return nil, err
	}

	return &session, nil
}

func GetSessionByID(db *sql.DB, sessionID string) (*Session, error) {
	var session Session

	err := db.QueryRow(`
		SELECT id, moderator_id, created_at, expires_at, ip_address, user_agent
		FROM mod_sessions
		WHERE id = $1 AND expires_at > $2`,
		sessionID, time.Now(),
	).Scan(
		&session.ID, &session.ModeratorID, &session.CreatedAt,
		&session.ExpiresAt, &session.IPAddress, &session.UserAgent,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("session not found or expired")
		}
		return nil, err
	}

	return &session, nil
}

func GetModeratorByID(db *sql.DB, id int) (*Moderator, error) {
	var mod Moderator

	err := db.QueryRow(`
		SELECT id, username, password_hash, created_at, last_login, is_active
		FROM moderators
		WHERE id = $1`,
		id,
	).Scan(
		&mod.ID, &mod.Username, &mod.PasswordHash,
		&mod.CreatedAt, &mod.LastLogin, &mod.IsActive,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("moderator not found")
		}
		return nil, err
	}

	return &mod, nil
}

func DeleteSession(db *sql.DB, sessionID string) error {
	_, err := db.Exec("DELETE FROM mod_sessions WHERE id = $1", sessionID)
	return err
}

func LogModAction(db *sql.DB, modID int, actionType string, targetID int, targetType string, ipAddress, reason string) error {
	_, err := db.Exec(`
		INSERT INTO mod_actions (moderator_id, action_type, target_id, target_type, ip_address, reason)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		modID, actionType, targetID, targetType, generateIPHash(ipAddress), reason,
	)
	return err
}

func DeleteThread(db *sql.DB, threadID, modID int, ipAddress, reason string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO mod_actions (moderator_id, action_type, target_id, target_type, ip_address, reason)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		modID, "delete_thread", threadID, "thread", generateIPHash(ipAddress), reason,
	)
	if err != nil {
		return err
	}

	_, err = tx.Exec("DELETE FROM threads WHERE id = $1", threadID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func DeletePost(db *sql.DB, postID, modID int, ipAddress, reason string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var threadID int
	var postCount int
	var isFirstPost bool

	err = tx.QueryRow(`
		SELECT thread_id, 
		       (SELECT COUNT(*) FROM posts WHERE thread_id = p.thread_id),
		       (SELECT MIN(id) FROM posts WHERE thread_id = p.thread_id) = p.id
		FROM posts p
		WHERE id = $1`,
		postID,
	).Scan(&threadID, &postCount, &isFirstPost)

	if err != nil {
		if err == sql.ErrNoRows {
			return errors.New("post not found")
		}
		return err
	}

	if isFirstPost && postCount == 1 {
		_, err = tx.Exec(`
			INSERT INTO mod_actions (moderator_id, action_type, target_id, target_type, ip_address, reason)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			modID, "delete_thread", threadID, "thread", generateIPHash(ipAddress), reason,
		)
		if err != nil {
			return err
		}

		_, err = tx.Exec("DELETE FROM threads WHERE id = $1", threadID)
		if err != nil {
			return err
		}
	} else if isFirstPost {
		return errors.New("cannot delete the first post of a thread with replies - delete the entire thread instead")
	} else {
		_, err = tx.Exec(`
			INSERT INTO mod_actions (moderator_id, action_type, target_id, target_type, ip_address, reason)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			modID, "delete_post", postID, "post", generateIPHash(ipAddress), reason,
		)
		if err != nil {
			return err
		}

		_, err = tx.Exec("DELETE FROM posts WHERE id = $1", postID)
		if err != nil {
			return err
		}

		_, err = tx.Exec(`
			UPDATE threads 
			SET post_count = post_count - 1
			WHERE id = $1`,
			threadID,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func GetModActions(db *sql.DB, page, perPage int) ([]ModAction, error) {
	offset := (page - 1) * perPage

	rows, err := db.Query(`
		SELECT id, moderator_id, action_type, target_id, target_type, executed_at, ip_address, reason
		FROM mod_actions
		ORDER BY executed_at DESC
		LIMIT $1 OFFSET $2`,
		perPage, offset,
	)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var actions []ModAction
	for rows.Next() {
		var action ModAction
		err := rows.Scan(
			&action.ID, &action.ModeratorID, &action.ActionType,
			&action.TargetID, &action.TargetType, &action.ExecutedAt,
			&action.IPAddress, &action.Reason,
		)
		if err != nil {
			return nil, err
		}
		actions = append(actions, action)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return actions, nil
}
