package main

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
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

		CREATE TABLE IF NOT EXISTS accounts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL UNIQUE,
			username TEXT NOT NULL,
			bearer_token TEXT NOT NULL,
			is_active INTEGER DEFAULT 1,
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

// GetSnapshotTimestamps returns all distinct snapshot timestamps for a source user, newest first.
func GetSnapshotTimestamps(db *sql.DB, sourceUserId string) ([]string, error) {
	rows, err := db.Query(`
		SELECT DISTINCT fetched_at FROM following_snapshots
		WHERE source_user_id = ?
		ORDER BY fetched_at DESC
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
	return timestamps, nil
}

// GetSnapshotUserIDs returns target user IDs for a specific snapshot timestamp.
func GetSnapshotUserIDs(db *sql.DB, sourceUserId, fetchedAt string) (map[string]bool, error) {
	rows, err := db.Query(`
		SELECT target_user_id FROM following_snapshots
		WHERE source_user_id = ? AND fetched_at = ?
	`, sourceUserId, fetchedAt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make(map[string]bool)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids[id] = true
	}
	return ids, nil
}

// GetUsersByIDs returns FollowingUser records for a list of user IDs.
func GetUsersByIDs(db *sql.DB, ids []string) ([]FollowingUser, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT id, username, COALESCE(name, ''), COALESCE(description, ''),
			COALESCE(followers_count, 0), COALESCE(following_count, 0),
			COALESCE(tweet_count, 0), COALESCE(listed_count, 0),
			COALESCE(verified, 0), COALESCE(verified_type, ''),
			COALESCE(profile_image_url, ''), COALESCE(location, ''),
			COALESCE(created_at, ''), COALESCE(updated_at, '')
		FROM users WHERE id IN (%s)
		ORDER BY followers_count DESC
	`, strings.Join(placeholders, ","))

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []FollowingUser
	for rows.Next() {
		var u FollowingUser
		var verified int
		if err := rows.Scan(&u.Id, &u.Username, &u.Name, &u.Description,
			&u.FollowersCount, &u.FollowingCount, &u.TweetCount, &u.ListedCount,
			&verified, &u.VerifiedType, &u.ProfileImageUrl, &u.Location,
			&u.CreatedAt, &u.UpdatedAt); err != nil {
			continue
		}
		u.Verified = verified == 1
		users = append(users, u)
	}
	return users, nil
}

// Account represents a tracked Twitter account stored in SQLite.
type Account struct {
	ID          int    `json:"id"`
	UserID      string `json:"user_id"`
	Username    string `json:"username"`
	BearerToken string `json:"-"`
	IsActive    bool   `json:"is_active"`
	CreatedAt   string `json:"created_at"`
}

func GetAllAccounts(db *sql.DB) ([]Account, error) {
	rows, err := db.Query(`SELECT id, user_id, username, bearer_token, is_active, created_at FROM accounts ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []Account
	for rows.Next() {
		var a Account
		var isActive int
		if err := rows.Scan(&a.ID, &a.UserID, &a.Username, &a.BearerToken, &isActive, &a.CreatedAt); err != nil {
			continue
		}
		a.IsActive = isActive == 1
		accounts = append(accounts, a)
	}
	return accounts, nil
}

func GetAccountByUserID(db *sql.DB, userID string) (*Account, error) {
	var a Account
	var isActive int
	err := db.QueryRow(`SELECT id, user_id, username, bearer_token, is_active, created_at FROM accounts WHERE user_id = ?`, userID).
		Scan(&a.ID, &a.UserID, &a.Username, &a.BearerToken, &isActive, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	a.IsActive = isActive == 1
	return &a, nil
}

func GetActiveAccounts(db *sql.DB) ([]Account, error) {
	rows, err := db.Query(`SELECT id, user_id, username, bearer_token, is_active, created_at FROM accounts WHERE is_active = 1 ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []Account
	for rows.Next() {
		var a Account
		var isActive int
		if err := rows.Scan(&a.ID, &a.UserID, &a.Username, &a.BearerToken, &isActive, &a.CreatedAt); err != nil {
			continue
		}
		a.IsActive = isActive == 1
		accounts = append(accounts, a)
	}
	return accounts, nil
}

func AddAccount(db *sql.DB, userID, username, bearerToken string) error {
	_, err := db.Exec(`INSERT OR IGNORE INTO accounts (user_id, username, bearer_token) VALUES (?, ?, ?)`,
		userID, username, bearerToken)
	return err
}

func RemoveAccount(db *sql.DB, userID string) error {
	_, err := db.Exec(`DELETE FROM accounts WHERE user_id = ?`, userID)
	return err
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
