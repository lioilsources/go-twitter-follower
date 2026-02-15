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
	ctx    context.Context
	db     *sql.DB
	config *Config
	client *gen.ClientWithResponses
	mu     sync.Mutex
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

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	a.config = GetConfig()

	client, err := NewAuthClient(a.config.BearerToken)
	if err != nil {
		log.Fatal(err)
	}
	a.client = client

	a.db = InitDB()

	// Start background scheduler
	go a.scheduler()
}

func (a *App) shutdown(ctx context.Context) {
	if a.db != nil {
		a.db.Close()
	}
}

func (a *App) scheduler() {
	// Fetch immediately on startup
	a.FetchNow()

	// Then every hour
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			a.FetchNow()
		case <-a.ctx.Done():
			return
		}
	}
}

func (a *App) FetchNow() string {
	a.mu.Lock()
	defer a.mu.Unlock()

	userId, err := ResolveUsername(a.client, a.config.Username)
	if err != nil {
		msg := fmt.Sprintf("Error resolving username: %v", err)
		log.Println(msg)
		return msg
	}

	all := FetchAllFollowing(a.client, userId)

	for _, user := range all {
		if err := UpsertUser(a.db, user); err != nil {
			log.Printf("Warning: failed to upsert user %s: %v", user.Id, err)
		}
	}
	if err := SaveSnapshot(a.db, userId, all); err != nil {
		log.Printf("Warning: failed to save snapshot: %v", err)
	}
	LogFetch(a.db, "GET /2/users/:id/following", userId, 200)

	// Check for changes and notify
	a.notifyChanges(userId, all)

	msg := fmt.Sprintf("Fetched %d followings at %s", len(all), time.Now().Format("15:04:05"))
	log.Println(msg)
	return msg
}

func (a *App) GetFollowingList() []FollowingUser {
	rows, err := a.db.Query(`
		SELECT id, username, COALESCE(name, ''), COALESCE(description, ''),
			COALESCE(followers_count, 0), COALESCE(following_count, 0),
			COALESCE(tweet_count, 0), COALESCE(listed_count, 0),
			COALESCE(verified, 0), COALESCE(verified_type, ''),
			COALESCE(profile_image_url, ''), COALESCE(location, ''),
			COALESCE(created_at, ''), COALESCE(updated_at, '')
		FROM users
		ORDER BY followers_count DESC
	`)
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

func (a *App) notifyChanges(sourceUserId string, currentUsers []gen.User) {
	previousIDs, err := GetPreviousFollowingIDs(a.db, sourceUserId)
	if err != nil {
		log.Printf("Warning: failed to get previous snapshot: %v", err)
		return
	}
	if previousIDs == nil {
		return // First fetch, nothing to compare
	}

	currentIDs := make(map[string]bool)
	currentMap := make(map[string]gen.User)
	for _, u := range currentUsers {
		currentIDs[u.Id] = true
		currentMap[u.Id] = u
	}

	// New followings
	var newUsers []string
	for _, u := range currentUsers {
		if !previousIDs[u.Id] {
			newUsers = append(newUsers, "@"+u.Username)
		}
	}

	// Lost followings
	var lostIDs []string
	for id := range previousIDs {
		if !currentIDs[id] {
			lostIDs = append(lostIDs, id)
		}
	}

	if len(newUsers) > 0 {
		msg := fmt.Sprintf("New: %s", strings.Join(newUsers, ", "))
		if err := beeep.Notify("Following Tracker", msg, ""); err != nil {
			log.Printf("Notification error: %v", err)
		}
	}

	if len(lostIDs) > 0 {
		// Look up usernames from DB
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
		if err := beeep.Notify("Following Tracker", msg, ""); err != nil {
			log.Printf("Notification error: %v", err)
		}
	}
}

func (a *App) GetStats() Stats {
	var stats Stats

	a.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&stats.TotalFollowing)
	a.db.QueryRow("SELECT COALESCE(MAX(created_at), '') FROM fetch_logs").Scan(&stats.LastFetchAt)
	a.db.QueryRow("SELECT COUNT(*) FROM fetch_logs").Scan(&stats.TotalFetches)

	return stats
}
