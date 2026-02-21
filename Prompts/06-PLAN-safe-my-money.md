# Plan: Úsporný režim + vylepšení Lists

## Kontext
API volání stojí peníze. Aktuálně app scheduluje fetch každou hodinu a Lists tab vždy volá API. Cíl: všechny drahé batch requesty (following, owned_lists, list_members) provádět max 1× za měsíc, jinak servírovat z SQLite cache. Scheduler pryč, vše manuální. Changes tab pryč (pro following nedává smysl). List members dostanou sloupec s příslušností k listům.

## 4 změny (v pořadí implementace)

---

### 1. Měsíční cache pro batch API volání

**db.go** — nové tabulky v `InitDB()` (za `accounts`):
```sql
CREATE TABLE IF NOT EXISTS list_cache (
    list_id TEXT NOT NULL,
    owner_user_id TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT DEFAULT '',
    member_count INTEGER DEFAULT 0,
    private INTEGER DEFAULT 0,
    fetched_at TEXT NOT NULL,
    PRIMARY KEY (list_id, owner_user_id)
);
CREATE TABLE IF NOT EXISTS list_member_cache (
    list_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    fetched_at TEXT NOT NULL,
    PRIMARY KEY (list_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_list_member_user ON list_member_cache(user_id);
```

**db.go** — nové funkce:
- `IsFollowingCacheFresh(db, sourceUserId) bool` — `MAX(fetched_at)` z `following_snapshots` < 30 dní
- `IsListCacheFresh(db, ownerUserId) bool` — `MAX(fetched_at)` z `list_cache` < 30 dní
- `IsListMemberCacheFresh(db, listId) bool` — `MAX(fetched_at)` z `list_member_cache` < 30 dní
- `SaveListCache(db, ownerUserId, []TwitterList)` — DELETE + INSERT do `list_cache`
- `GetCachedLists(db, ownerUserId) []TwitterList`
- `SaveListMemberCache(db, listId, []string{userIDs})` — DELETE + INSERT do `list_member_cache`
- `GetCachedListMemberIDs(db, listId) []string`

Všechny `fetched_at` použijí `time.Now().UTC().Format(time.RFC3339)` (konzistentní s `SaveSnapshot`).

**app.go** — cache-aware metody:
- `GetOwnedLists()` → check `IsListCacheFresh` → cache hit: `GetCachedLists` / miss: `fetchAndCacheOwnedLists()` (nová privátní metoda)
- `GetListMembers(listId)` → check `IsListMemberCacheFresh` → cache hit: `GetUsersByIDs(GetCachedListMemberIDs)` / miss: `fetchAndCacheListMembers(listId)` (nová privátní metoda)
- `fetchForAccount()` → přidat check `IsFollowingCacheFresh`, pokud fresh → skip API, return "Cache fresh"

---

### 2. Odstranit scheduler, vše manuální

**app.go**:
- Smazat `scheduler()` metodu a `go a.scheduler()` volání ve `startup()`
- Smazat `FetchAllAccounts()` (volá jen scheduler)
- Přejmenovat `fetchForAccount()` → `fetchFollowingForAccount()`, odebrat `notifyChanges()` volání
- Přidat `FetchListsNow() string` — force-fetch owned lists + všichni members (bypass cache)
- `FetchNow()` zůstává pro following (s cache check)

**frontend/src/main.js**:
- Smazat `setInterval(loadData, 5 * 60 * 1000)` (řádek 321)
- `fetchNow()` — tab-aware: pokud je aktivní Lists tab → volat `FetchListsNow()` + `loadLists()`, jinak `FetchNow()` + `loadData()`

---

### 3. Odstranit Changes tab

**app.go** — smazat:
- `DiffResult` struct (řádky 48-52)
- `GetDiff()` metodu (řádky 424-468)
- `notifyChanges()` metodu (řádky 472-526)
- `beeep` import

**db.go** — smazat:
- `GetPreviousFollowingIDs()` (řádky 155-201)
- `GetSnapshotTimestamps()` (řádky 204-224)
- `GetSnapshotUserIDs()` (řádky 227-246)
- Ponechat `GetUsersByIDs()` — používá se pro list members cache

**frontend/index.html**:
- Smazat Changes tab button (řádek 30)
- Smazat `#tab-changes` div (řádky 72-86)

**frontend/src/main.js**:
- Smazat `loadDiff()`, `renderDiffCard()`
- Smazat `loadDiff()` volání ze `switchAccount()` a `switchTab()`

**frontend/src/style.css**:
- Smazat diff CSS (`#diff-container` až `.diff-card-stats`)

Po změnách: `go mod tidy` odstraní `beeep` a tranzitivní deps.

---

### 4. Sloupec "Lists" v tabulce list members

**app.go** — rozšířit `FollowingUser`:
```go
Lists []string `json:"lists,omitempty"`
```

**db.go** — nová funkce:
```go
func GetListNamesForUsers(db, ownerUserId, []userIDs) map[string][]string
// JOIN list_member_cache + list_cache → user_id → []list_name
```

**app.go** — nová privátní metoda `enrichWithListNames([]FollowingUser) []FollowingUser`:
- Volá `GetListNamesForUsers()`, přiřadí `Lists` pole
- Volat v `getListMembersFromCache()` i `fetchAndCacheListMembers()` před return

**frontend/index.html**:
- Přidat `<th class="col-lists">Lists</th>` do list-members-table header
- Colspan 7 → 8 v loading states

**frontend/src/main.js**:
- `renderListMembers()` — přidat `<td class="lists-cell">${renderListBadges(u.lists)}</td>`
- Nová funkce `renderListBadges(lists)` — render jako `<span class="list-badge">` tagy

**frontend/src/style.css**:
- `.col-lists`, `.lists-cell`, `.list-badge` styly (badge: `background: #1d9bf0`, bílý text, rounded)

---

## Soubory ke změně

| Soubor | Změna |
|--------|-------|
| `db.go` | +2 tabulky, +8 cache funkcí, −3 diff funkce |
| `app.go` | Cache-aware metody, −scheduler, −diff, −notify, +FetchListsNow, +enrichWithListNames |
| `frontend/index.html` | −Changes tab, +Lists column header |
| `frontend/src/main.js` | −diff kód, −setInterval, tab-aware fetchNow, +renderListBadges |
| `frontend/src/style.css` | −diff CSS, +list-badge CSS |

## Verifikace

1. `go build .` kompiluje
2. `go mod tidy` odstraní beeep
3. `wails dev` startuje bez chyb
4. Following tab: první Fetch Now → API call, druhý → "Cache fresh" (do 30 dní)
5. Lists tab: první load → API call + cache, druhý → z cache
6. Lists tab → Fetch Now → force refresh ze API
7. List members tabulka zobrazuje "Lists" sloupec s badge tagy
8. Changes tab neexistuje
9. Žádný automatický scheduler/interval
