# Aktuální stav: go-twitter-follower — po implementaci plánu 01

## Co bylo implementováno

Všech 5 kroků z `01-PLAN-native-app.md` je hotovo + přidán diff view.

---

## Aktuální architektura

```
┌──────────────────────────────────────────────┐
│  macOS Wails v2 app                          │
│  ├─ Okno s dashboardem (1024×700)            │
│  ├─ Nativní notifikace (beeep)               │
│  │                                           │
│  ├─ Go backend                               │
│  │  ├─ twitter.go — API logika               │
│  │  │  ├─ NewAuthClient (bearer token)       │
│  │  │  ├─ ResolveUsername                     │
│  │  │  ├─ GetFollowing (single page)         │
│  │  │  └─ FetchAllFollowing (pagination, 3s) │
│  │  │                                        │
│  │  ├─ app.go — Wails backend                │
│  │  │  ├─ GetFollowingList()                 │
│  │  │  ├─ GetDiff() — new/lost users         │
│  │  │  ├─ GetStats()                         │
│  │  │  ├─ FetchNow()                         │
│  │  │  ├─ scheduler() — 1x/hod              │
│  │  │  └─ notifyChanges() — beeep notify     │
│  │  │                                        │
│  │  └─ db.go — SQLite storage (data.db)      │
│  │     ├─ InitDB() — WAL mode, schema        │
│  │     ├─ UpsertUser()                        │
│  │     ├─ SaveSnapshot()                      │
│  │     ├─ GetPreviousFollowingIDs()           │
│  │     ├─ GetSnapshotTimestamps()             │
│  │     ├─ GetSnapshotUserIDs()                │
│  │     ├─ GetUsersByIDs()                     │
│  │     └─ LogFetch()                          │
│  │                                           │
│  └─ Frontend (HTML/CSS/JS ve WebView)        │
│     ├─ Tab: Following — tabulka s metrikami   │
│     ├─ Tab: Changes — diff new/lost users    │
│     └─ Dark theme (X/Twitter style)          │
└──────────────────────────────────────────────┘
```

---

## Struktura souborů

```
├── main.go              # Wails entry point, GetConfig(), embedded frontend
├── app.go               # Wails backend (Go ↔ JS bridge)
├── twitter.go           # Twitter API v2 logika
├── db.go                # SQLite init, CRUD, diff queries
├── auth.go              # OAuth1 PIN-based flow (experimentální)
├── frontend/
│   ├── index.html       # Dashboard s tab navigací
│   └── src/
│       ├── main.js      # Tab switching, diff loading, table rendering
│       └── style.css    # Dark theme, tabs, diff cards
├── gen/
│   └── twitter-client.gen.go  # Auto-generated (oapi-codegen)
├── build/
│   └── appicon.png
├── wails.json
├── .env                 # BEARER_TOKEN, TWITTER_USERNAME, ...
└── Prompts/
    ├── 01-PLAN-native-app.md
    └── 02-PLAN-sqlite-dashboard-diff-followers.md
```

---

## SQLite schéma

```sql
CREATE TABLE users (
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

CREATE TABLE following_snapshots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_user_id TEXT NOT NULL,
    target_user_id TEXT NOT NULL,
    fetched_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_snapshots_source ON following_snapshots(source_user_id, fetched_at);
CREATE INDEX idx_snapshots_target ON following_snapshots(target_user_id);

CREATE TABLE fetch_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    endpoint TEXT,
    user_id TEXT,
    status_code INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

---

## Frontend — dvě záložky

### Tab: Following
- Tabulka všech sledovaných uživatelů
- Sloupce: avatar, user (name + handle + verified badge), description, followers, following, tweets, location
- Vyhledávání (username, name, description)
- Řazení (followers, following, tweets, username, name) + asc/desc
- Kliknutelné hlavičky tabulky pro rychlé řazení

### Tab: Changes (Diff view)
- Porovnání dvou posledních snapshotů
- Sekce "New Following" (zelená) — nově sledovaní
- Sekce "Unfollowed" (červená) — přestali sledovat
- Card layout s avatarem, jménem, handle, followers count, tweets count
- Empty state pokud nejsou změny nebo < 2 fetche

---

## Diff logika

1. `GetDiff()` v `app.go` najde `source_user_id` z DB
2. `GetSnapshotTimestamps()` vrátí timestamps všech snapshotů (newest first)
3. Porovná `timestamps[0]` (current) vs `timestamps[1]` (previous)
4. `GetSnapshotUserIDs()` vrátí sety user IDs pro oba snapshoty
5. Set difference → new IDs (v current ale ne v previous), lost IDs (naopak)
6. `GetUsersByIDs()` načte plné profily z `users` tabulky

---

## Notifikace

Po každém fetch (`notifyChanges()`):
- Porovná aktuální following list s předchozím snapshotem
- Nový following → macOS notifikace "New: @user1, @user2"
- Ztracený following → "Lost: @user1, @user2"
- Používá `github.com/gen2brain/beeep`

---

## Graceful degradace

- Pokud chybí `.env` nebo credentials → app se spustí, zobrazí warning v logu
- `FetchNow()` vrátí "Not configured" místo crash
- `GetConfig()` hledá `.env` v CWD, executable dir, a parent dirs (pro .app bundle)

---

## Příkazy

```bash
# Development
wails dev                    # Hot reload (wails CLI v ~/go/bin/)

# Build
wails build                  # Vytvoří .app bundle
go build .                   # Plain Go build (bez Wails runtime)

# Regenerace API klienta
make generate
```

---

## Dependencies

```
github.com/wailsapp/wails/v2    # Desktop app framework
modernc.org/sqlite               # Pure Go SQLite (no CGO)
github.com/gen2brain/beeep       # Nativní notifikace
github.com/deepmap/oapi-codegen  # OpenAPI codegen
github.com/dghubble/oauth1       # OAuth1 auth
github.com/joho/godotenv         # .env loading
```

---

## Možná další rozšíření

- [ ] Historie změn (ne jen poslední diff, ale všechny snapshoty)
- [ ] Graf růstu followerů v čase
- [ ] System tray ikona (menu bar mode)
- [ ] Diff dvou libovolných snapshotů (dropdown výběr)
- [ ] Export dat (CSV/JSON)
- [ ] Multi-account tracking
- [ ] Auto-follow/unfollow (OAuth1 write operations)
