package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	"go-twitter-follower/gen"
)

type App struct {
	ctx               context.Context
	db                *sql.DB
	config            *Config
	mu                sync.Mutex
	selectedAccountID string
}

type FollowingUser struct {
	Id              string   `json:"id"`
	Username        string   `json:"username"`
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	FollowersCount  int      `json:"followers_count"`
	FollowingCount  int      `json:"following_count"`
	TweetCount      int      `json:"tweet_count"`
	ListedCount     int      `json:"listed_count"`
	Verified        bool     `json:"verified"`
	VerifiedType    string   `json:"verified_type"`
	ProfileImageUrl string   `json:"profile_image_url"`
	Location        string   `json:"location"`
	CreatedAt       string   `json:"created_at"`
	UpdatedAt       string   `json:"updated_at"`
	Lists           []string `json:"lists,omitempty"`
}

type Stats struct {
	TotalCount     int    `json:"total_count"`
	LastFetchAt    string `json:"last_fetch_at"`
	CacheExpiresAt string `json:"cache_expires_at"`
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

// --- Lists (cache-aware) ---

type TwitterList struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	MemberCount int    `json:"member_count"`
	Private     bool   `json:"private"`
}

func (a *App) GetListsStats() Stats {
	var stats Stats
	if a.selectedAccountID == "" {
		return stats
	}

	a.db.QueryRow(`
		SELECT COUNT(DISTINCT lmc.user_id)
		FROM list_member_cache lmc
		JOIN list_cache lc ON lmc.list_id = lc.list_id AND lc.owner_user_id = ?
	`, a.selectedAccountID).Scan(&stats.TotalCount)

	var fetchedAt string
	a.db.QueryRow(`
		SELECT COALESCE(MAX(fetched_at), '') FROM list_cache WHERE owner_user_id = ?
	`, a.selectedAccountID).Scan(&fetchedAt)
	if fetchedAt != "" {
		stats.LastFetchAt = fetchedAt
		if t, err := time.Parse(time.RFC3339, fetchedAt); err == nil {
			stats.CacheExpiresAt = t.Add(30 * 24 * time.Hour).Format(time.RFC3339)
		}
	}

	return stats
}

func (a *App) GetOwnedLists() []TwitterList {
	if a.selectedAccountID == "" {
		return nil
	}
	return GetCachedLists(a.db, a.selectedAccountID)
}

func (a *App) fetchAndCacheOwnedLists() []TwitterList {
	acct, err := GetAccountByUserID(a.db, a.selectedAccountID)
	if err != nil {
		log.Printf("Error getting account: %v", err)
		return nil
	}

	client, err := NewAuthClient(acct.BearerToken)
	if err != nil {
		log.Printf("Error creating client: %v", err)
		return nil
	}

	lists, err := GetOwnedLists(client, a.selectedAccountID)
	if err != nil {
		log.Printf("Error fetching owned lists: %v", err)
		return nil
	}

	var result []TwitterList
	for _, l := range lists {
		tl := TwitterList{
			Id:   string(l.Id),
			Name: l.Name,
		}
		if l.Description != nil {
			tl.Description = *l.Description
		}
		if l.MemberCount != nil {
			tl.MemberCount = *l.MemberCount
		}
		if l.Private != nil {
			tl.Private = *l.Private
		}
		result = append(result, tl)
	}

	if err := SaveListCache(a.db, a.selectedAccountID, result); err != nil {
		log.Printf("Warning: failed to save list cache: %v", err)
	}

	return result
}

func (a *App) GetListMembers(listId string) []FollowingUser {
	if a.selectedAccountID == "" {
		return nil
	}
	return a.getListMembersFromCache(listId)
}

func (a *App) getListMembersFromCache(listId string) []FollowingUser {
	ids := GetCachedListMemberIDs(a.db, listId)
	users, err := GetUsersByIDs(a.db, ids)
	if err != nil {
		log.Printf("Error getting cached list members: %v", err)
		return nil
	}
	return a.enrichWithListNames(users)
}

func (a *App) fetchAndCacheListMembers(listId string) []FollowingUser {
	acct, err := GetAccountByUserID(a.db, a.selectedAccountID)
	if err != nil {
		log.Printf("Error getting account: %v", err)
		return nil
	}

	client, err := NewAuthClient(acct.BearerToken)
	if err != nil {
		log.Printf("Error creating client: %v", err)
		return nil
	}

	users, err := FetchAllListMembers(client, listId)
	if err != nil {
		log.Printf("Error fetching list members: %v", err)
		return nil
	}

	var userIDs []string
	var result []FollowingUser
	for _, u := range users {
		if err := UpsertUser(a.db, u); err != nil {
			log.Printf("Warning: failed to upsert user %s: %v", u.Id, err)
		}
		userIDs = append(userIDs, u.Id)
		fu := genUserToFollowingUser(u)
		result = append(result, fu)
	}

	if err := SaveListMemberCache(a.db, listId, userIDs); err != nil {
		log.Printf("Warning: failed to save list member cache: %v", err)
	}

	return a.enrichWithListNames(result)
}

func (a *App) enrichWithListNames(users []FollowingUser) []FollowingUser {
	if len(users) == 0 {
		return users
	}
	ids := make([]string, len(users))
	for i, u := range users {
		ids[i] = u.Id
	}
	listMap := GetListNamesForUsers(a.db, a.selectedAccountID, ids)
	if listMap == nil {
		return users
	}
	for i := range users {
		if names, ok := listMap[users[i].Id]; ok {
			users[i].Lists = names
		}
	}
	return users
}

// --- Followers ---

func (a *App) GetFollowersList() []FollowingUser {
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
		INNER JOIN followers_snapshots fs ON u.id = fs.target_user_id
		WHERE fs.source_user_id = ?
		  AND fs.fetched_at = (
			  SELECT MAX(fetched_at) FROM followers_snapshots
			  WHERE source_user_id = ?
		  )
		ORDER BY u.followers_count DESC
	`, a.selectedAccountID, a.selectedAccountID)
	if err != nil {
		log.Printf("Error querying followers: %v", err)
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
			log.Printf("Error scanning follower: %v", err)
			continue
		}
		u.Verified = verified == 1
		users = append(users, u)
	}
	return users
}

func (a *App) GetFollowersStats() Stats {
	var stats Stats
	if a.selectedAccountID == "" {
		return stats
	}

	a.db.QueryRow(`
		SELECT COUNT(*) FROM followers_snapshots
		WHERE source_user_id = ?
		  AND fetched_at = (SELECT MAX(fetched_at) FROM followers_snapshots WHERE source_user_id = ?)
	`, a.selectedAccountID, a.selectedAccountID).Scan(&stats.TotalCount)

	var fetchedAt string
	a.db.QueryRow(`
		SELECT COALESCE(MAX(fetched_at), '') FROM followers_snapshots WHERE source_user_id = ?
	`, a.selectedAccountID).Scan(&fetchedAt)
	if fetchedAt != "" {
		stats.LastFetchAt = fetchedAt
		if t, err := time.Parse(time.RFC3339, fetchedAt); err == nil {
			stats.CacheExpiresAt = t.Add(30 * 24 * time.Hour).Format(time.RFC3339)
		}
	}

	return stats
}

func (a *App) FetchFollowersNow() string {
	if a.selectedAccountID == "" {
		return "No account selected. Add an account first."
	}

	acct, err := GetAccountByUserID(a.db, a.selectedAccountID)
	if err != nil {
		return "Account not found."
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if IsFollowersCacheFresh(a.db, acct.UserID) {
		msg := fmt.Sprintf("Cache fresh for @%s followers, skipping API call", acct.Username)
		log.Println(msg)
		return msg
	}

	log.Printf("[fetch] Fetching followers for @%s (user_id=%s)", acct.Username, acct.UserID)
	client, err := NewAuthClient(acct.BearerToken)
	if err != nil {
		msg := fmt.Sprintf("Error for @%s: %v", acct.Username, err)
		log.Println(msg)
		return msg
	}

	all, err := FetchAllFollowers(client, acct.UserID)
	if err != nil {
		msg := fmt.Sprintf("Fetch error for @%s: %v", acct.Username, err)
		log.Println(msg)
		return msg
	}

	for _, user := range all {
		if err := UpsertUser(a.db, user); err != nil {
			log.Printf("Warning: failed to upsert user %s: %v", user.Id, err)
		}
	}
	if err := SaveFollowersSnapshot(a.db, acct.UserID, all); err != nil {
		log.Printf("Warning: failed to save followers snapshot: %v", err)
	}
	LogFetch(a.db, "GET /2/users/:id/followers", acct.UserID, 200)

	msg := fmt.Sprintf("Fetched %d followers for @%s at %s", len(all), acct.Username, time.Now().Format("15:04:05"))
	log.Println(msg)
	return msg
}

// --- Fetching (manual only, no scheduler) ---

// FetchNow fetches following for the currently selected account (manual trigger from UI).
func (a *App) FetchNow() string {
	if a.selectedAccountID == "" {
		return "No account selected. Add an account first."
	}

	acct, err := GetAccountByUserID(a.db, a.selectedAccountID)
	if err != nil {
		return "Account not found."
	}

	return a.fetchFollowingForAccount(*acct)
}

// FetchListsNow fetches owned lists + all members (with 30-day cache check).
func (a *App) FetchListsNow() string {
	if a.selectedAccountID == "" {
		return "No account selected. Add an account first."
	}

	if IsListCacheFresh(a.db, a.selectedAccountID) {
		msg := fmt.Sprintf("Cache fresh for lists, skipping API call")
		log.Println(msg)
		return msg
	}

	lists := a.fetchAndCacheOwnedLists()
	if lists == nil {
		return "Error fetching lists."
	}

	for _, l := range lists {
		a.fetchAndCacheListMembers(l.Id)
	}

	return fmt.Sprintf("Fetched %d lists at %s", len(lists), time.Now().Format("15:04:05"))
}

func (a *App) fetchFollowingForAccount(acct Account) string {
	a.mu.Lock()
	defer a.mu.Unlock()

	if IsFollowingCacheFresh(a.db, acct.UserID) {
		msg := fmt.Sprintf("Cache fresh for @%s, skipping API call", acct.Username)
		log.Println(msg)
		return msg
	}

	log.Printf("[fetch] Fetching for @%s (user_id=%s, token=%s...)", acct.Username, acct.UserID, acct.BearerToken[:min(8, len(acct.BearerToken))])
	client, err := NewAuthClient(acct.BearerToken)
	if err != nil {
		msg := fmt.Sprintf("Error for @%s: %v", acct.Username, err)
		log.Println(msg)
		return msg
	}

	all, err := FetchAllFollowing(client, acct.UserID)
	if err != nil {
		msg := fmt.Sprintf("Fetch error for @%s: %v", acct.Username, err)
		log.Println(msg)
		return msg
	}

	for _, user := range all {
		if err := UpsertUser(a.db, user); err != nil {
			log.Printf("Warning: failed to upsert user %s: %v", user.Id, err)
		}
	}
	if err := SaveSnapshot(a.db, acct.UserID, all); err != nil {
		log.Printf("Warning: failed to save snapshot: %v", err)
	}
	LogFetch(a.db, "GET /2/users/:id/following", acct.UserID, 200)

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
	`, a.selectedAccountID, a.selectedAccountID).Scan(&stats.TotalCount)

	var fetchedAt string
	a.db.QueryRow(`
		SELECT COALESCE(MAX(fetched_at), '') FROM following_snapshots WHERE source_user_id = ?
	`, a.selectedAccountID).Scan(&fetchedAt)
	if fetchedAt != "" {
		stats.LastFetchAt = fetchedAt
		if t, err := time.Parse(time.RFC3339, fetchedAt); err == nil {
			stats.CacheExpiresAt = t.Add(30 * 24 * time.Hour).Format(time.RFC3339)
		}
	}

	return stats
}

// --- Helpers ---

func genUserToFollowingUser(u gen.User) FollowingUser {
	fu := FollowingUser{
		Id:       u.Id,
		Username: u.Username,
		Name:     u.Name,
	}
	if u.Description != nil {
		fu.Description = *u.Description
	}
	if u.PublicMetrics != nil {
		fu.FollowersCount = u.PublicMetrics.FollowersCount
		fu.FollowingCount = u.PublicMetrics.FollowingCount
		fu.TweetCount = u.PublicMetrics.TweetCount
		fu.ListedCount = u.PublicMetrics.ListedCount
	}
	if u.Verified != nil {
		fu.Verified = *u.Verified
	}
	if u.VerifiedType != nil {
		fu.VerifiedType = string(*u.VerifiedType)
	}
	if u.ProfileImageUrl != nil {
		fu.ProfileImageUrl = *u.ProfileImageUrl
	}
	if u.Location != nil {
		fu.Location = *u.Location
	}
	if u.CreatedAt != nil {
		fu.CreatedAt = u.CreatedAt.Format("2006-01-02T15:04:05Z")
	}
	return fu
}
