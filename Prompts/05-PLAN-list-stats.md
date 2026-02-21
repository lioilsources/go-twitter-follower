# 05 — Lists Tab (zobrazení Twitter Lists a jejich členů)

## Cíl

Přidat záložku "Lists" do dashboardu, která zobrazuje Twitter Lists vlastněné uživatelem a po kliknutí na list zobrazí jeho členy v tabulce identické s Following tab. Pouze zobrazení — bez change trackingu/snapshotů.

## Co bylo implementováno

### 1. OpenAPI spec — nové List endpointy
**Soubor:** `open-api-spec-all-components.yaml`

Přidány dvě cesty z `open-api-spec-full-twitter-0223.yaml`:
- `GET /2/lists/{id}/members` (operationId: `listGetMembers`) — vrací členy listu jako `User[]`
- `GET /2/users/{id}/owned_lists` (operationId: `listUserOwnedLists`) — vrací vlastněné listy

Opravena pre-existující chyba: duplicitní `JSONDefault` pole ve vygenerovaném klientu (default response měl `Error` i `Problem` typ, sjednoceno na `Problem`).

Po změně spec: `make generate` → `gen/twitter-client.gen.go` přegenerován.

### 2. API funkce
**Soubor:** `twitter.go`

```go
func GetOwnedLists(client, userId) ([]gen.List, error)
// Fetch user's lists with list.fields (description, member_count, follower_count, private, created_at, owner_id)

func GetListMembers(client, listId, paginationToken) (*[]gen.User, *string, error)
// Single page — stejný vzor jako GetFollowing, s user.fields

func FetchAllListMembers(client, listId) ([]gen.User, error)
// Paginovaný fetch — stejný vzor jako FetchAllFollowing, s rate limiting (3s delay)
```

### 3. Wails-bound metody
**Soubor:** `app.go`

```go
type TwitterList struct {
    Id, Name, Description string
    MemberCount           int
    Private               bool
}

func (a *App) GetOwnedLists() []TwitterList
// Volá GetOwnedLists() pro vybraný účet

func (a *App) GetListMembers(listId string) []FollowingUser
// Volá FetchAllListMembers() + UpsertUser() do DB
// Reuse existujícího FollowingUser struct — žádné nové tabulky
```

### 4. Frontend

**`frontend/index.html`**
- Nový tab button "Lists" v `<nav class="tabs">`
- `#tab-lists` s dvěma views:
  - Grid view — karty listů (název, popis, počet členů, private badge)
  - Members view — tabulka členů (identická se Following tab) + back button + search

**`frontend/src/main.js`**
- `loadLists()` — fetch a render karet listů
- `viewListMembers(listId, name)` — fetch členů, render do tabulky
- `backToLists()` — navigace zpět na grid
- `filterListMembers()` — vyhledávání v členech
- `renderListMembers()` — reuse stejného row template jako Following tab
- `switchTab()` rozšířen o `lists` tab

**`frontend/src/style.css`**
- `.lists-grid` — CSS grid layout pro karty
- `.list-card` — karta listu (dark theme, hover efekt)
- `.list-private-badge` — badge pro private listy
- `.list-members-header` + `.back-btn` — navigace v member view

---

## Architektura po implementaci

```
┌──────────────────────────────────────────────┐
│  macOS Wails v2 app                          │
│  ├─ Go backend                               │
│  │  ├─ twitter.go                            │
│  │  │  ├─ NewAuthClient (bearer token)       │
│  │  │  ├─ ResolveUsername                     │
│  │  │  ├─ GetFollowing / FetchAllFollowing   │
│  │  │  ├─ GetOwnedLists           ← NOVÉ    │
│  │  │  ├─ GetListMembers          ← NOVÉ    │
│  │  │  └─ FetchAllListMembers     ← NOVÉ    │
│  │  │                                        │
│  │  ├─ app.go                                │
│  │  │  ├─ GetFollowingList() / GetDiff()     │
│  │  │  ├─ GetOwnedLists()         ← NOVÉ    │
│  │  │  ├─ GetListMembers(listId)  ← NOVÉ    │
│  │  │  └─ scheduler, notify, accounts...     │
│  │  │                                        │
│  │  └─ db.go — beze změn                     │
│  │                                           │
│  └─ Frontend                                 │
│     ├─ Tab: Following                        │
│     ├─ Tab: Changes                          │
│     └─ Tab: Lists                  ← NOVÉ   │
│        ├─ Grid view (karty listů)            │
│        └─ Members view (tabulka + search)    │
└──────────────────────────────────────────────┘
```

---

## Změněné soubory

| Soubor | Typ změny |
|--------|-----------|
| `open-api-spec-all-components.yaml` | Editován — přidány 2 paths, opraveny default responses |
| `gen/twitter-client.gen.go` | Přegenerován |
| `twitter.go` | Editován — 3 nové funkce |
| `app.go` | Editován — `TwitterList` struct + 2 nové Wails metody |
| `frontend/index.html` | Editován — Lists tab + HTML structure |
| `frontend/src/main.js` | Editován — Lists logika (load, view, search, back) |
| `frontend/src/style.css` | Editován — Lists grid + card + member view styly |

---

## Twitter API endpointy

| Endpoint | Rate limit | Poznámka |
|----------|-----------|----------|
| `GET /2/users/{id}/owned_lists` | 15 req/15min | Vrací `List[]` s metadaty |
| `GET /2/lists/{id}/members` | 900 req/15min | Vrací `User[]`, paginovaný (max 100/page) |

---

## Verifikace

- [x] `make generate` úspěšně vygeneruje klienta
- [x] `go build .` kompiluje bez chyb
- [ ] `wails dev` startuje bez chyb
- [ ] Lists tab zobrazuje vlastněné listy jako karty
- [ ] Kliknutí na kartu zobrazí členy v tabulce
- [ ] Vyhledávání filtruje členy
- [ ] Back button se vrátí na grid

---

## Možná další rozšíření

- [ ] Řazení členů listu (followers, tweets, atd.)
- [ ] Cachování členů listů do SQLite (jako following snapshots)
- [ ] Diff členů listu mezi snapshoty
- [ ] Zobrazení listů, kde je uživatel členem (list_memberships endpoint)
- [ ] Přidání/odebrání členů z listu (write operations)
