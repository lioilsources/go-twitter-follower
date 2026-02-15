# Plán: Rozšíření go-twitter-follower — SQLite, Wails menu bar app, 24/7 provoz

## Context

Aplikace aktuálně ukládá API odpovědi jako JSON soubory do `res/` a běží jednorázově z terminálu. Uživatel chce:
1. **Pouze SQLite** storage (smazat file-based)
2. **Native macOS menu bar app** přes Wails v2 (ne web dashboard)
3. **24/7 provoz** s nativními notifikacemi (nový/ztracený follower)
4. Opravit rate limiter (aktuálně 30x pomalejší než nutné)

---

## Architektura

```
┌──────────────────────────────────────────┐
│  macOS menu bar app (Wails v2)           │
│  ├─ System tray ikona (vždy viditelná)   │
│  ├─ Klik → okno s dashboardem            │
│  ├─ Nativní notifikace                   │
│  │                                       │
│  ├─ Go backend                           │
│  │  ├─ Fetcher (API calls, pagination,   │
│  │  │  rate limiting)                    │
│  │  ├─ Scheduler (1x za hodinu)          │
│  │  └─ SQLite storage (data.db)          │
│  │                                       │
│  └─ Frontend (HTML/CSS/JS ve WebView)    │
│     ├─ Tabulka following s metrikami      │
│     ├─ Diff dvou účtů                    │
│     └─ Graf růstu followerů              │
└──────────────────────────────────────────┘
```

### Proč Wails v2 (ne web dashboard)?
- Menu bar ikona — vždy viditelná, bez otevřeného browseru
- Nativní macOS notifikace (follower gained/lost)
- `.app` bundle — spustitelný jako normální macOS aplikace
- Frontend je stále HTML/JS — stejný effort jako Gin, ale lepší UX
- Background operace bez launchd (app sama běží 24/7)

### Proč SQLite?
- Relational queries (diff, sort by metrics, historie)
- Jeden soubor (`data.db`), žádný server
- `modernc.org/sqlite` — pure Go, žádný CGO

---

## Implementační kroky

### Krok 1: Opravit rate limiter + přidat user.fields
**Soubor**: `main.go`

- `main.go:22-25` — Změnit `rate_limit` z `90s` na `3s`, opravit komentář:
  ```go
  // GET /2/users/:id/following | 300 reqs/15 minutes
  rate_limit = 1000 * time.Millisecond * 3
  ```
- `main.go:136-145` — Přidat `UserFields` do `GetFollowing` params:
  - `public_metrics`, `description`, `created_at`, `verified`, `profile_image_url`, `location`
- Smazat deprecated `GetFollowers` funkci (`main.go:96-134`)
- Smazat `StoreResponse` funkci (`main.go:172-192`) — nahradí SQLite

### Krok 2: SQLite storage
**Nový soubor**: `db.go`

- Dependency: `modernc.org/sqlite` + `database/sql`
- Schéma:
  ```sql
  CREATE TABLE users (
      id TEXT PRIMARY KEY,
      username TEXT NOT NULL,
      name TEXT,
      description TEXT,
      followers_count INTEGER,
      following_count INTEGER,
      tweet_count INTEGER,
      verified INTEGER,
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

  CREATE TABLE fetch_logs (
      id INTEGER PRIMARY KEY AUTOINCREMENT,
      endpoint TEXT,
      user_id TEXT,
      status_code INTEGER,
      created_at DATETIME DEFAULT CURRENT_TIMESTAMP
  );
  ```
- Funkce: `InitDB()`, `UpsertUser()`, `SaveSnapshot()`, `GetDiff()`, `GetFollowingHistory()`
- Smazat `res/` adresář a veškerý file-based storage kód

### Krok 3: Refactor do knihovny
**Soubory**: `main.go` → rozdělit na `twitter.go` + `main.go`

- Extrahovat API logiku do exportovaných funkcí v `twitter.go`:
  - `FetchAllFollowing(client, userId) → []gen.User`
  - `ResolveUsername(client, username) → string`
- `main.go` zůstane jen jako entry point (CLI nebo Wails)
- Toto umožní Wails backendu volat API funkce přímo

### Krok 4: Wails v2 menu bar app
**Nové soubory**: Wails projekt struktura

```
├── main.go              # Wails app entry point
├── app.go               # Wails backend (Go methods volatelné z JS)
├── twitter.go           # API logika (z kroku 3)
├── db.go                # SQLite (z kroku 2)
├── frontend/            # Wails frontend
│   ├── index.html
│   ├── src/
│   │   ├── main.js      # Dashboard logika
│   │   └── style.css
│   └── wailsjs/         # Auto-generated bindings
├── build/               # Wails build output
│   └── appicon.png
└── wails.json           # Wails config
```

- `app.go` — Wails backend struct s metodami:
  - `GetFollowing() []User` — vrátí data z SQLite
  - `GetDiff(user1, user2 string) []User` — diff dvou účtů
  - `GetStats() Stats` — statistiky
  - `FetchNow()` — manuální trigger fetche
  - `StartScheduler()` — spustí background fetching (1x/hod)
- Frontend — HTML/JS dashboard:
  - Tabulka following s řazením a filtry
  - Diff view
  - Notifikace při změnách (nový/ztracený follower)
- System tray ikona s menu:
  - "Show Dashboard" — otevře okno
  - "Fetch Now" — manuální fetch
  - "Last fetch: 14:30" — info
  - "Quit"

### Krok 5: Nativní notifikace
**Soubor**: `app.go`

- Po každém fetch porovnat s předchozím snapshotem
- Pokud nový follower → macOS notifikace "New follower: @username"
- Pokud ztracený → "Lost follower: @username"
- Wails v2: použít `beeep` nebo `go-toast` knihovnu (Wails v2 nemá built-in notifications, to je až v3)

---

## Priorita a pořadí

| Krok | Co | Závislosti |
|------|----|-----------|
| 1 | Rate limiter + user.fields | Žádné |
| 2 | SQLite storage | Krok 1 |
| 3 | Refactor do knihovny | Krok 2 |
| 4 | Wails menu bar app | Krok 3 |
| 5 | Notifikace | Krok 4 |

---

## Klíčové soubory

| Soubor | Akce | Popis |
|--------|------|-------|
| `main.go` | Edit → refactor | Rate limiter, user.fields, pak Wails entry point |
| `twitter.go` | Nový | Extrahovaná API logika |
| `db.go` | Nový | SQLite init, CRUD, dotazy |
| `app.go` | Nový | Wails backend (Go ↔ JS bridge) |
| `frontend/` | Nový | Dashboard HTML/JS/CSS |
| `wails.json` | Nový | Wails konfigurace |
| `go.mod` | Edit | +wails, +modernc.org/sqlite |

Smazat:
- `StoreResponse()` z `main.go`
- `GetFollowers()` (deprecated) z `main.go`
- `res/` adresář usage

## Existující kód k znovupoužití

- `GetConfig()` (`main.go:43`) — načítání `.env`
- `NewAuthClient()` (`main.go:60`) — autentizace bearer tokenem
- `GetUserIdFromUsername()` (`main.go:76`) — resolve username → ID
- `GetFollowing()` (`main.go:136`) — API volání s paginací (rozšířit o user.fields)
- `gen.User` struct — obsahuje `PublicMetrics` field pro metriky
- OAuth1 flow (`auth.go`) — pro budoucí write operace (follow, like)

---

## Nové dependencies

```
github.com/wailsapp/wails/v2    # Desktop app framework
modernc.org/sqlite               # Pure Go SQLite driver
github.com/gen2brain/beeep       # Nativní notifikace (cross-platform)
```

## Verifikace

1. `go build .` — kompilace bez chyb
2. Jednorázový fetch — data se uloží do `data.db`, ne do `res/`
3. `wails dev` — spustí dev mode, menu bar ikona viditelná
4. Klik na ikonu → dashboard s tabulkou following
5. Scheduler — automatický fetch každou hodinu, notifikace při změně
6. `wails build` — vytvoří `.app` bundle v `build/bin/`
