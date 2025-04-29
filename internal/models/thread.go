package models

import (
	"database/sql"
	"strings"
	"time"
)

type Thread struct {
	ID         int       `json:"id"`
	BoardID    int       `json:"board_id"`
	Title      string    `json:"title"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	IsPinned   bool      `json:"is_pinned"`
	IsLocked   bool      `json:"is_locked"`
	LastPostAt time.Time `json:"last_post_at"`
	PostCount  int       `json:"post_count"`
	Preview    string    `json:"preview"`
}

type Pagination struct {
	CurrentPage int  `json:"current_page"`
	TotalPages  int  `json:"total_pages"`
	HasPrev     bool `json:"has_prev"`
	HasNext     bool `json:"has_next"`
	PrevPage    int  `json:"prev_page"`
	NextPage    int  `json:"next_page"`
}

func GetThreadsByBoardID(db *sql.DB, boardID, page, perPage int) ([]Thread, error) {
	offset := (page - 1) * perPage

	query := `
		SELECT t.id, t.board_id, t.title, t.created_at, t.updated_at, 
		       t.is_pinned, t.is_locked, t.last_post_at, t.post_count,
		       SUBSTRING(p.body, 1, 150) AS preview
		FROM threads t
		LEFT JOIN posts p ON t.id = p.thread_id AND p.id = (
			SELECT MIN(id) FROM posts WHERE thread_id = t.id
		)
		WHERE t.board_id = $1
		ORDER BY t.is_pinned DESC, t.last_post_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := db.Query(query, boardID, perPage, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var threads []Thread
	for rows.Next() {
		var thread Thread
		err := rows.Scan(
			&thread.ID,
			&thread.BoardID,
			&thread.Title,
			&thread.CreatedAt,
			&thread.UpdatedAt,
			&thread.IsPinned,
			&thread.IsLocked,
			&thread.LastPostAt,
			&thread.PostCount,
			&thread.Preview,
		)
		if err != nil {
			return nil, err
		}
		threads = append(threads, thread)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return threads, nil
}

func CountThreadsByBoardID(db *sql.DB, boardID int) (int, error) {
	query := `SELECT COUNT(*) FROM threads WHERE board_id = $1`

	var count int
	err := db.QueryRow(query, boardID).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func GetThreadByID(db *sql.DB, id int) (*Thread, error) {
	query := `
		SELECT id, board_id, title, created_at, updated_at,
		       is_pinned, is_locked, last_post_at, post_count
		FROM threads
		WHERE id = $1
	`

	var thread Thread
	err := db.QueryRow(query, id).Scan(
		&thread.ID,
		&thread.BoardID,
		&thread.Title,
		&thread.CreatedAt,
		&thread.UpdatedAt,
		&thread.IsPinned,
		&thread.IsLocked,
		&thread.LastPostAt,
		&thread.PostCount,
	)

	if err != nil {
		return nil, err
	}

	return &thread, nil
}

func CreateThread(db *sql.DB, boardID int, title, author, body, ipAddress string) (int, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var threadID int
	err = tx.QueryRow(`
		INSERT INTO threads (board_id, title)
		VALUES ($1, $2)
		RETURNING id
	`, boardID, title).Scan(&threadID)

	if err != nil {
		return 0, err
	}

	ipHash := generateIPHash(ipAddress)

	var tripcode string
	author, tripcode = processTripcode(author)

	_, err = tx.Exec(`
		INSERT INTO posts (thread_id, author, body, ip_hash, tripcode)
		VALUES ($1, $2, $3, $4, $5)
	`, threadID, author, body, ipHash, tripcode)

	if err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return threadID, nil
}

func generateIPHash(ipAddress string) string {
	parts := strings.Split(ipAddress, ".")
	if len(parts) >= 4 {
		return parts[0] + "." + parts[1] + "." + parts[2] + ".***"
	}

	if len(ipAddress) > 6 {
		return ipAddress[:6] + "***"
	}

	return "unknown"
}

func GetThreadForUpdate(db *sql.DB, threadID int) (*Thread, error) {
	var t Thread
	err := db.QueryRow(`
        SELECT id, board_id, title, created_at, updated_at, 
               is_pinned, is_locked, last_post_at, post_count 
        FROM threads 
        WHERE id = $1 
        FOR UPDATE NOWAIT
    `, threadID).Scan(
		&t.ID, &t.BoardID, &t.Title, &t.CreatedAt, &t.UpdatedAt,
		&t.IsPinned, &t.IsLocked, &t.LastPostAt, &t.PostCount,
	)
	return &t, err
}
