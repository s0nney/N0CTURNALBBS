package models

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"database/sql"
	"encoding/base64"
	"strings"
	"time"
)

type Post struct {
	ID        int       `json:"id"`
	ThreadID  int       `json:"thread_id"`
	Author    string    `json:"author"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	IPHash    string    `json:"-"` // Not exposed to templates for privacy
	Tripcode  string    `json:"tripcode"`
}

func GetPostsByThreadID(db *sql.DB, threadID int) ([]Post, error) {
	query := `
                SELECT id, thread_id, author, body, created_at, tripcode
                FROM posts
                WHERE thread_id = $1
                ORDER BY id
        `

	rows, err := db.Query(query, threadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var post Post
		var author sql.NullString
		var tripcode sql.NullString

		err := rows.Scan(
			&post.ID,
			&post.ThreadID,
			&author,
			&post.Body,
			&post.CreatedAt,
			&tripcode,
		)
		if err != nil {
			return nil, err
		}

		if author.Valid {
			post.Author = author.String
		}
		if tripcode.Valid {
			post.Tripcode = tripcode.String
		}

		posts = append(posts, post)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return posts, nil
}

func CreatePost(db *sql.DB, threadID int, author, body, ipAddress string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	ipHash := generateIPHash(ipAddress)

	var tripcode string
	author, tripcode = processTripcode(author)

	// Insert the post
	_, err = tx.Exec(`
                INSERT INTO posts (thread_id, author, body, ip_hash, tripcode)
                VALUES ($1, $2, $3, $4, $5)
        `, threadID, author, body, ipHash, tripcode)

	if err != nil {
		return err
	}

	_, err = tx.Exec(`
                UPDATE threads
                SET last_post_at = CURRENT_TIMESTAMP,
                    post_count = post_count + 1,
                    updated_at = CURRENT_TIMESTAMP
                WHERE id = $1
        `, threadID)

	if err != nil {
		return err
	}

	return tx.Commit()
}

func processTripcode(author string) (string, string) {
	secureTrip := strings.Split(author, "##")
	if len(secureTrip) > 1 {
		name := secureTrip[0]
		password := secureTrip[1]
		tripcode := generateSecureTripcode(password)
		return name, tripcode
	}

	parts := strings.Split(author, "#")
	if len(parts) == 1 {
		return author, ""
	}
	name := parts[0]
	password := parts[1]
	tripcode := generateTripcode(password)
	return name, tripcode
}

func generateSecureTripcode(password string) string {
	if len(password) > 32 {
		password = password[:32]
	}

	salt := "salt_here" // Generate with: head -c 64 /dev/urandom | base64
	salted := password + salt

	h1 := sha512.New()
	h1.Write([]byte(salted))
	intermediate := h1.Sum(nil)

	h2 := hmac.New(sha512.New, []byte(salt))
	h2.Write(intermediate)
	hash := h2.Sum(nil)

	tripcode := base64.URLEncoding.EncodeToString(hash)
	if len(tripcode) > 10 {
		tripcode = tripcode[:10]
	}

	return "!!" + tripcode
}

func generateTripcode(password string) string {
	if len(password) > 32 {
		password = password[:32]
	}
	h := sha256.New()
	h.Write([]byte(password))
	hash := h.Sum(nil)
	tripcode := base64.URLEncoding.EncodeToString(hash)
	if len(tripcode) > 10 {
		tripcode = tripcode[:10]
	}
	return "!" + tripcode
}
