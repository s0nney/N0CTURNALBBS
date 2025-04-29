package models

import (
	"database/sql"
	"time"
)

type Board struct {
	ID          int       `json:"id"`
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	Active      bool      `json:"active"`
	ThreadCount int       `json:"thread_count"`
	PostCount   int       `json:"post_count"`
}

func GetAllBoards(db *sql.DB) ([]Board, error) {
	query := `
		SELECT b.id, b.slug, b.name, b.description, b.created_at, b.active,
			   COUNT(DISTINCT t.id) AS thread_count,
			   COUNT(p.id) AS post_count
		FROM boards b
		LEFT JOIN threads t ON b.id = t.board_id
		LEFT JOIN posts p ON t.id = p.thread_id
		WHERE b.active = true
		GROUP BY b.id
		ORDER BY b.name
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var boards []Board
	for rows.Next() {
		var board Board
		err := rows.Scan(
			&board.ID,
			&board.Slug,
			&board.Name,
			&board.Description,
			&board.CreatedAt,
			&board.Active,
			&board.ThreadCount,
			&board.PostCount,
		)
		if err != nil {
			return nil, err
		}
		boards = append(boards, board)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return boards, nil
}

func GetBoardBySlug(db *sql.DB, slug string) (*Board, error) {
	query := `
		SELECT b.id, b.slug, b.name, b.description, b.created_at, b.active,
			   COUNT(DISTINCT t.id) AS thread_count,
			   COUNT(p.id) AS post_count
		FROM boards b
		LEFT JOIN threads t ON b.id = t.board_id
		LEFT JOIN posts p ON t.id = p.thread_id
		WHERE b.slug = $1 AND b.active = true
		GROUP BY b.id
	`

	var board Board
	err := db.QueryRow(query, slug).Scan(
		&board.ID,
		&board.Slug,
		&board.Name,
		&board.Description,
		&board.CreatedAt,
		&board.Active,
		&board.ThreadCount,
		&board.PostCount,
	)

	if err != nil {
		return nil, err
	}

	return &board, nil
}

func GetBoardByID(db *sql.DB, id int) (*Board, error) {
	query := `
		SELECT b.id, b.slug, b.name, b.description, b.created_at, b.active,
			   COUNT(DISTINCT t.id) AS thread_count,
			   COUNT(p.id) AS post_count
		FROM boards b
		LEFT JOIN threads t ON b.id = t.board_id
		LEFT JOIN posts p ON t.id = p.thread_id
		WHERE b.id = $1 AND b.active = true
		GROUP BY b.id
	`

	var board Board
	err := db.QueryRow(query, id).Scan(
		&board.ID,
		&board.Slug,
		&board.Name,
		&board.Description,
		&board.CreatedAt,
		&board.Active,
		&board.ThreadCount,
		&board.PostCount,
	)

	if err != nil {
		return nil, err
	}

	return &board, nil
}
