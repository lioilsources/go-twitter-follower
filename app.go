package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"go-twitter-follower/gen"

	"github.com/gen2brain/beeep"
)

type App struct {
	ctx               context.Context
	db                *sql.DB
	config            *Config
	mu                sync.Mutex
	selectedAccountID string
}

type FollowingUser struct {
	Id              string `json:"id"`
	Username        string `json:"username"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	FollowersCount  int    `json:"followers_count"`
	FollowingCount  int    `json:"following_count"`
	TweetCount      int    `json:"tweet_count"`
	ListedCount     int    `json:"listed_count"`
	Verified        bool   `json:"verified"`
	VerifiedType    string `json:"verified_type"`
	ProfileImageUrl string `json:"profile_image_url"`
	Location        string `json:"location"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

type Stats struct {
	TotalFollowing int    `json:"total_following"`
	LastFetchAt    string `json:"last_fetch_at"`
	TotalFetches   int    `json:"total_fetches"`
}

type DiffResult struct {
	NewUsers  []FollowingUser `json:"new_users"`
	LostUsers []FollowingUser `json:"lost_users"`
	FetchedAt string          `json:"fetched_at"`
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.config = GetConfig()
	a.db = InitDB()

	// Auto-import .env account if configured
	if a.config.BearerToken != "" && a.config.Username != "" {
		userId := a.config.UserId
		if userId == "" {
			client, err := NewAuthClient(a.config.BearerToken)
			if err == nil {
				resolved, err := ResolveUsername(client, a.config.Username)
				if err == nil {
					userId = resolved
				}
			}
		}
		if userId != "" {
			AddAccount(a.db, userId, a.config.Username, a.config.BearerToken)
			a.selectedAccountID = userId
		}
	}

	// If no account selected, pick the first one
	if a.selectedAccountID == "" {
		accounts, _ := GetAllAccounts(a.db)
		if len(accounts) > 0 {
			a.selectedAccountID = accounts[0].UserID
		}
	}

	// Start background scheduler
	go a.scheduler()
}

func (a *App) shutdown(ctx context.Context) {
	if a.db != nil {
		a.db.Close()
	}
}

// --- Account management (Wails-bound) ---

func (a *App) GetAccounts() []Account {
	accounts, err := GetAllAccounts(a.db)
	if err != nil {
		log.Printf("Error getting accounts: %v", err)
		return nil
	}
	return accounts
}

func (a *App) GetSelectedAccount() string {
	return a.selectedAccountID
}

func (a *App) SelectAccount(userID string) {
	a.selectedAccountID = userID
}

func (a *App) AddNewAccount(username, bearerToken string) (string, error) {
	client, err := NewAuthClient(bearerToken)
	if err != nil {
		return "", fmt.Errorf("invalid bearer token: %w", err)
	}

	userId, err := ResolveUsername(client, username)
	if err != nil {
		return "", fmt.Errorf("could not resolve @%s: %w", username, err)
	}

	if err := AddAccount(a.db, userId, username, bearerToken); err != nil {
		return "", fmt.Errorf("failed to save account: %w", err)
	}

	// Auto-select if first account
	if a.selectedAccountID == "" {
		a.selectedAccountID = userId
	}

	return userId, nil
}

func (a *App) RemoveAccountByID(userID string) error {
	if err := RemoveAccount(a.db, userID); err != nil {
		return err
	}

	// If we removed the selected account, switch to another
	if a.selectedAccountID == userID {
		a.selectedAccountID = ""
		accounts, _ := GetAllAccounts(a.db)
		if len(accounts) > 0 {
			a.selectedAccountID = accounts[0].UserID
		}
	}
	return nil
}

// --- Scheduler & Fetching ---

func (a *App) scheduler() {
	a.FetchAllAccounts()

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			a.FetchAllAccounts()
		case <-a.ctx.Done():
			return
		}
	}
}

func (a *App) FetchAllAccounts() string {
	accounts, err := GetActiveAccounts(a.db)
	if err != nil || len(accounts) == 0 {
		return "No accounts configured."
	}

	var results []string
	for _, acct := range accounts {
		result := a.fetchForAccount(acct)
		results = append(results, result)
	}
	return strings.Join(results, "\n")
}

// FetchNow fetches for the currently selected account (manual trigger from UI).
func (a *App) FetchNow() string {
	if a.selectedAccountID == "" {
		return "No account selected. Add an account first."
	}

	acct, err := GetAccountByUserID(a.db, a.selectedAccountID)
	if err != nil {
		return "Account not found."
	}

	return a.fetchForAccount(*acct)
}

func (a *App) fetchForAccount(acct Account) string {
	a.mu.Lock()
	defer a.mu.Unlock()

	client, err := NewAuthClient(acct.BearerToken)
	if err != nil {
		msg := fmt.Sprintf("Error for @%s: %v", acct.Username, err)
		log.Println(msg)
		return msg
	}

	all := FetchAllFollowing(client, acct.UserID)

	for _, user := range all {
		if err := UpsertUser(a.db, user); err != nil {
			log.Printf("Warning: failed to upsert user %s: %v", user.Id, err)
		}
	}
	if err := SaveSnapshot(a.db, acct.UserID, all); err != nil {
		log.Printf("Warning: failed to save snapshot: %v", err)
	}
	LogFetch(a.db, "GET /2/users/:id/following", acct.UserID, 200)

	a.notifyChanges(acct.UserID, acct.Username, all)

	msg := fmt.Sprintf("Fetched %d for @%s at %s", len(all), acct.Username, time.Now().Format("15:04:05"))
	log.Println(msg)
	return msg
}

// --- Data queries (scoped to selectedAccountID) ---

func (a *App) GetFollowingList() []FollowingUser {
	if a.selectedAccountID == "" {
		return nil
	}

	rows, err := a.db.Query(`
		SELECT u.id, u.username, COALESCE(u.name, ''), COALESCE(u.description, ''),
			COALESCE(u.followers_count, 0), COALESCE(u.following_count, 0),
			COALESCE(u.tweet_count, 0), COALESCE(u.listed_count, 0),
			COALESCE(u.verified, 0), COALESCE(u.verified_type, ''),
			COALESCE(u.profile_image_url, ''), COALESCE(u.location, ''),
			COALESCE(u.created_at, ''), COALESCE(u.updated_at, '')
		FROM users u
		INNER JOIN following_snapshots fs ON u.id = fs.target_user_id
		WHERE fs.source_user_id = ?
		  AND fs.fetched_at = (
			  SELECT MAX(fetched_at) FROM following_snapshots
			  WHERE source_user_id = ?
		  )
		ORDER BY u.followers_count DESC
	`, a.selectedAccountID, a.selectedAccountID)
	if err != nil {
		log.Printf("Error querying users: %v", err)
		return nil
	}
	defer rows.Close()

	var users []FollowingUser
	for rows.Next() {
		var u FollowingUser
		var verified int
		err := rows.Scan(&u.Id, &u.Username, &u.Name, &u.Description,
			&u.FollowersCount, &u.FollowingCount, &u.TweetCount, &u.ListedCount,
			&verified, &u.VerifiedType, &u.ProfileImageUrl, &u.Location,
			&u.CreatedAt, &u.UpdatedAt)
		if err != nil {
			log.Printf("Error scanning user: %v", err)
			continue
		}
		u.Verified = verified == 1
		users = append(users, u)
	}
	return users
}

func (a *App) GetStats() Stats {
	var stats Stats
	if a.selectedAccountID == "" {
		return stats
	}

	a.db.QueryRow(`
		SELECT COUNT(*) FROM following_snapshots
		WHERE source_user_id = ?
		  AND fetched_at = (SELECT MAX(fetched_at) FROM following_snapshots WHERE source_user_id = ?)
	`, a.selectedAccountID, a.selectedAccountID).Scan(&stats.TotalFollowing)

	a.db.QueryRow(`
		SELECT COALESCE(MAX(created_at), '') FROM fetch_logs WHERE user_id = ?
	`, a.selectedAccountID).Scan(&stats.LastFetchAt)

	a.db.QueryRow(`
		SELECT COUNT(*) FROM fetch_logs WHERE user_id = ?
	`, a.selectedAccountID).Scan(&stats.TotalFetches)

	return stats
}

func (a *App) GetDiff() DiffResult {
	result := DiffResult{}
	if a.selectedAccountID == "" {
		return result
	}

	timestamps, err := GetSnapshotTimestamps(a.db, a.selectedAccountID)
	if err != nil || len(timestamps) < 2 {
		return result
	}

	result.FetchedAt = timestamps[0]

	currentIDs, err := GetSnapshotUserIDs(a.db, a.selectedAccountID, timestamps[0])
	if err != nil {
		return result
	}
	previousIDs, err := GetSnapshotUserIDs(a.db, a.selectedAccountID, timestamps[1])
	if err != nil {
		return result
	}

	var newIDs []string
	for id := range currentIDs {
		if !previousIDs[id] {
			newIDs = append(newIDs, id)
		}
	}

	var lostIDs []string
	for id := range previousIDs {
		if !currentIDs[id] {
			lostIDs = append(lostIDs, id)
		}
	}

	if len(newIDs) > 0 {
		result.NewUsers, _ = GetUsersByIDs(a.db, newIDs)
	}
	if len(lostIDs) > 0 {
		result.LostUsers, _ = GetUsersByIDs(a.db, lostIDs)
	}

	return result
}

// --- Notifications ---

func (a *App) notifyChanges(sourceUserId, accountUsername string, currentUsers []gen.User) {
	previousIDs, err := GetPreviousFollowingIDs(a.db, sourceUserId)
	if err != nil {
		log.Printf("Warning: failed to get previous snapshot: %v", err)
		return
	}
	if previousIDs == nil {
		return
	}

	currentIDs := make(map[string]bool)
	for _, u := range currentUsers {
		currentIDs[u.Id] = true
	}

	var newUsers []string
	for _, u := range currentUsers {
		if !previousIDs[u.Id] {
			newUsers = append(newUsers, "@"+u.Username)
		}
	}

	var lostIDs []string
	for id := range previousIDs {
		if !currentIDs[id] {
			lostIDs = append(lostIDs, id)
		}
	}

	title := fmt.Sprintf("@%s Following Tracker", accountUsername)

	if len(newUsers) > 0 {
		msg := fmt.Sprintf("New: %s", strings.Join(newUsers, ", "))
		if err := beeep.Notify(title, msg, ""); err != nil {
			log.Printf("Notification error: %v", err)
		}
	}

	if len(lostIDs) > 0 {
		var lostNames []string
		for _, id := range lostIDs {
			var username string
			err := a.db.QueryRow("SELECT username FROM users WHERE id = ?", id).Scan(&username)
			if err == nil {
				lostNames = append(lostNames, "@"+username)
			} else {
				lostNames = append(lostNames, id)
			}
		}
		msg := fmt.Sprintf("Lost: %s", strings.Join(lostNames, ", "))
		if err := beeep.Notify(title, msg, ""); err != nil {
			log.Printf("Notification error: %v", err)
		}
	}
}
