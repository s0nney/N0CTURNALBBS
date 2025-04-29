package models

import (
	"database/sql"
	"strconv"
	"strings"
	"time"
)

func CheckForBannedWords(db *sql.DB, texts ...string) (bool, error) {
	query := `
        SELECT EXISTS(
            SELECT 1 
            FROM banned_words 
            WHERE $1 ILIKE '%' || word || '%'
        )
    `

	for _, text := range texts {
		text = strings.ToLower(text)
		var exists bool
		err := db.QueryRow(query, text).Scan(&exists)
		if err != nil {
			return false, err
		}
		if exists {
			return true, nil
		}
	}
	return false, nil
}

type BannedWord struct {
	ID        int
	Word      string
	CreatedAt time.Time
}

func GetAllBannedWords(db *sql.DB) ([]BannedWord, error) {
	rows, err := db.Query("SELECT id, word, created_at FROM banned_words ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var words []BannedWord
	for rows.Next() {
		var w BannedWord
		err := rows.Scan(&w.ID, &w.Word, &w.CreatedAt)
		if err != nil {
			return nil, err
		}
		words = append(words, w)
	}
	return words, nil
}

func AddBannedWord(db *sql.DB, word string, modID int, ip string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	var wordID int
	err = tx.QueryRow(
		"INSERT INTO banned_words(word) VALUES ($1) RETURNING id",
		word,
	).Scan(&wordID)

	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Exec(`INSERT INTO mod_actions 
        (moderator_id, action_type, target_id, target_type, ip_address)
        VALUES ($1, 'add_word_filter', $2, 'word_filter', $3)`,
		modID, wordID, generateIPHash(ip),
	)

	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func DeleteBannedWord(db *sql.DB, wordID string, modID int, ip string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	id, err := strconv.Atoi(wordID)
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Exec(`INSERT INTO mod_actions 
        (moderator_id, action_type, target_id, target_type, ip_address)
        VALUES ($1, 'remove_word_filter', $2, 'word_filter', $3)`,
		modID, id, generateIPHash(ip),
	)

	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Exec("DELETE FROM banned_words WHERE id = $1", id)
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}
