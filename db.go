package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"go-twitter-follower/gen"

	_ "modernc.org/sqlite"
)

const dbPath = "data.db"

func InitDB() *sql.DB {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatal(fmt.Errorf("opening database: %w", err))
	}

	// Enable WAL mode for better concurrent read/write performance
	_, err = db.Exec("PRAGMA journal_mode=WAL")
	if err != nil {
		log.Fatal(fmt.Errorf("setting WAL mode: %w", err))
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			username TEXT NOT NULL,
			name TEXT,
			description TEXT,
			followers_count INTEGER,
			following_count INTEGER,
			tweet_count INTEGER,
			listed_count INTEGER,
			verified INTEGER DEFAULT 0,
			verified_type TEXT,
			profile_image_url TEXT,
			created_at TEXT,
			location TEXT,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS following_snapshots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			source_user_id TEXT NOT NULL,
			target_user_id TEXT NOT NULL,
			fetched_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_snapshots_source ON following_snapshots(source_user_id, fetched_at);
		CREATE INDEX IF NOT EXISTS idx_snapshots_target ON following_snapshots(target_user_id);

		CREATE TABLE IF NOT EXISTS fetch_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			endpoint TEXT,
			user_id TEXT,
			status_code INTEGER,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		log.Fatal(fmt.Errorf("creating tables: %w", err))
	}

	return db
}

func UpsertUser(db *sql.DB, user gen.User) error {
	var followersCount, followingCount, tweetCount, listedCount int
	if user.PublicMetrics != nil {
		followersCount = user.PublicMetrics.FollowersCount
		followingCount = user.PublicMetrics.FollowingCount
		tweetCount = user.PublicMetrics.TweetCount
		listedCount = user.PublicMetrics.ListedCount
	}

	var verified int
	if user.Verified != nil && *user.Verified {
		verified = 1
	}

	var createdAt *string
	if user.CreatedAt != nil {
		s := user.CreatedAt.Format(time.RFC3339)
		createdAt = &s
	}

	_, err := db.Exec(`
		INSERT INTO users (id, username, name, description, followers_count, following_count,
			tweet_count, listed_count, verified, verified_type, profile_image_url, created_at, location, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			username = excluded.username,
			name = excluded.name,
			description = excluded.description,
			followers_count = excluded.followers_count,
			following_count = excluded.following_count,
			tweet_count = excluded.tweet_count,
			listed_count = excluded.listed_count,
			verified = excluded.verified,
			verified_type = excluded.verified_type,
			profile_image_url = excluded.profile_image_url,
			created_at = excluded.created_at,
			location = excluded.location,
			updated_at = CURRENT_TIMESTAMP
	`, user.Id, user.Username, user.Name, user.Description,
		followersCount, followingCount, tweetCount, listedCount,
		verified, user.VerifiedType, user.ProfileImageUrl, createdAt, user.Location)

	return err
}

func SaveSnapshot(db *sql.DB, sourceUserId string, users []gen.User) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	fetchedAt := time.Now().UTC().Format(time.RFC3339)

	stmt, err := tx.Prepare(`
		INSERT INTO following_snapshots (source_user_id, target_user_id, fetched_at)
		VALUES (?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("preparing statement: %w", err)
	}
	defer stmt.Close()

	for _, user := range users {
		if _, err := stmt.Exec(sourceUserId, user.Id, fetchedAt); err != nil {
			return fmt.Errorf("inserting snapshot for user %s: %w", user.Id, err)
		}
	}

	return tx.Commit()
}

// GetPreviousFollowingIDs returns the set of target user IDs from the most recent
// snapshot before the current one for a given source user.
func GetPreviousFollowingIDs(db *sql.DB, sourceUserId string) (map[string]bool, error) {
	// Get the two most recent distinct fetched_at timestamps
	rows, err := db.Query(`
		SELECT DISTINCT fetched_at FROM following_snapshots
		WHERE source_user_id = ?
		ORDER BY fetched_at DESC
		LIMIT 2
	`, sourceUserId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var timestamps []string
	for rows.Next() {
		var ts string
		if err := rows.Scan(&ts); err != nil {
			return nil, err
		}
		timestamps = append(timestamps, ts)
	}

	// Need at least 2 snapshots to compare
	if len(timestamps) < 2 {
		return nil, nil
	}

	previousTS := timestamps[1]
	idRows, err := db.Query(`
		SELECT target_user_id FROM following_snapshots
		WHERE source_user_id = ? AND fetched_at = ?
	`, sourceUserId, previousTS)
	if err != nil {
		return nil, err
	}
	defer idRows.Close()

	ids := make(map[string]bool)
	for idRows.Next() {
		var id string
		if err := idRows.Scan(&id); err != nil {
			return nil, err
		}
		ids[id] = true
	}
	return ids, nil
}

func LogFetch(db *sql.DB, endpoint, userId string, statusCode int) {
	_, err := db.Exec(`
		INSERT INTO fetch_logs (endpoint, user_id, status_code)
		VALUES (?, ?, ?)
	`, endpoint, userId, statusCode)
	if err != nil {
		log.Printf("Warning: failed to log fetch: %v", err)
	}
}
