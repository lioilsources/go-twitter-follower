# Plan: Přidat Followers tab

## Kontext
App sleduje following (koho sleduješ) a lists. Chybí followers (kdo sleduje tebe). Přidat nový Followers tab se stejným vzorem: manuální fetch, 30-denní cache, tabulka se search/sort. Generovaný klient už má `UsersIdFollowersWithResponse` — stačí ho zavolat.

## 3 změny

---

### 1. Twitter API funkce pro followers

**twitter.go** — nové funkce (kopie `GetFollowing`/`FetchAllFollowing` vzoru):

```go
func GetFollowers(client, userId, paginationToken) (*[]gen.User, *string, error)
// Volá client.UsersIdFollowersWithResponse() se stejnými UserFields
// Parametry: gen.UsersIdFollowersParams (má PaginationToken, UserFields, MaxResults)

func FetchAllFollowers(client, userId) ([]gen.User, error)
// Paginace se 3s rate limitem — identický vzor jako FetchAllFollowing
```

---

### 2. DB + App backend

**db.go** — nová tabulka v `InitDB()`:
```sql
CREATE TABLE IF NOT EXISTS followers_snapshots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_user_id TEXT NOT NULL,
    target_user_id TEXT NOT NULL,
    fetched_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_followers_source ON followers_snapshots(source_user_id, fetched_at);
CREATE INDEX IF NOT EXISTS idx_followers_target ON followers_snapshots(target_user_id);
```

**db.go** — nové funkce:
- `IsFollowersCacheFresh(db, sourceUserId) bool` — `MAX(fetched_at)` z `followers_snapshots` < 30 dní
- `SaveFollowersSnapshot(db, sourceUserId, []gen.User) error` — INSERT do `followers_snapshots` (stejný vzor jako `SaveSnapshot`)

**app.go** — nové metody:

```go
// Wails-bound: vrací followers z DB (nejnovější snapshot)
func (a *App) GetFollowersList() []FollowingUser
// Stejný SQL jako GetFollowingList() ale z followers_snapshots

// Wails-bound: vrací stats pro followers
func (a *App) GetFollowersStats() Stats
// COUNT z followers_snapshots, MAX(fetched_at) z fetch_logs pro followers endpoint

// Wails-bound: manuální fetch
func (a *App) FetchFollowersNow() string
// → IsFollowersCacheFresh() check → skip if fresh
// → FetchAllFollowers() → UpsertUser() → SaveFollowersSnapshot() → LogFetch()
```

---

### 3. Frontend — Followers tab

**frontend/index.html** — nový tab button + tab content:
- `<button class="tab" onclick="switchTab('followers')">Followers</button>` (mezi Following a Lists)
- `<div id="tab-followers">` — identická struktura jako Following tab (search, sort dropdowny, tabulka)
- Vlastní IDčka: `#followers-search`, `#followers-sort-by`, `#followers-sort-dir`, `#followers-table`, `#followers-body`

**frontend/src/main.js**:
- `let allFollowers = []`
- `loadFollowers()` → `App.GetFollowersList()` + `App.GetFollowersStats()` → `renderFollowersTable()`
- `renderFollowersTable(users)` — stejný render jako `renderTable()` ale do `#followers-body`
- `filterFollowersTable()`, `sortFollowersTable()`, `sortFollowersBy(field)` — kopie following logiky
- `switchTab('followers')` → volat `loadFollowers()`
- `fetchNow()` — rozšířit tab-aware logiku: followers tab → `FetchFollowersNow()` + `loadFollowers()`
- Stats v headeru: zobrazit followers count když je aktivní followers tab

---

## Další změny (implementováno společně)

### Sjednocení cache chování
- `GetOwnedLists()` a `GetListMembers()` už nefetchují automaticky z API — jen čtou z DB cache
- `FetchListsNow()` přidána `IsListCacheFresh()` kontrola (30-denní cache)
- Všechny taby fungují stejně: data z cache, API call jen přes Fetch Now s 30-denní cache

### Stats v hlavičce pro všechny taby
- `Stats` struct: `TotalCount`, `LastFetchAt`, `CacheExpiresAt` (datum vypršení cache)
- `GetStats()`, `GetFollowersStats()`, `GetListsStats()` — každý vrací stats pro svůj tab
- Frontend: dynamický label (following/followers/lists), datum fetche, datum příštího live callu

### Přejmenování na XBoost
- Window title, About dialog, HTML title a heading

### Odstranění HideWindowOnClose
- Aplikace se normálně zavře křížkem

## Soubory ke změně

| Soubor | Změna |
|--------|-------|
| `twitter.go` | +`GetFollowers()`, +`FetchAllFollowers()` |
| `db.go` | +`followers_snapshots` tabulka, +`IsFollowersCacheFresh`, +`SaveFollowersSnapshot` |
| `app.go` | +`GetFollowersList()`, +`GetFollowersStats()`, +`FetchFollowersNow()`, +`GetListsStats()`, sjednocení cache, rename |
| `main.go` | Rename na XBoost, odstranění HideWindowOnClose |
| `frontend/index.html` | +Followers tab, stats v hlavičce, rename |
| `frontend/src/main.js` | +followers funkce, stats pro všechny taby, rozšířený `fetchNow` a `switchTab` |
| `frontend/src/style.css` | Padding pro macOS traffic lights |
