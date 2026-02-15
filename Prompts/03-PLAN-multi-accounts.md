# Plán: Multi-account podpora pro go-twitter-follower

## Context

Aplikace aktuálně podporuje pouze jeden X account (načtený z `.env`). Uživatel chce spravovat více účtů a přepínat mezi nimi v UI. DB už používá `source_user_id` ve `following_snapshots`, takže datová vrstva je připravena — chybí správa účtů, scoping dotazů a UI.

---

## Implementační kroky

### Krok 1: DB — tabulka `accounts` + CRUD funkce
**Soubor**: `db.go`

Přidat do `InitDB()`:
```sql
CREATE TABLE IF NOT EXISTS accounts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL UNIQUE,
    username TEXT NOT NULL,
    bearer_token TEXT NOT NULL,
    is_active INTEGER DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

Nové typy a funkce:
- `Account` struct (ID, UserID, Username, BearerToken, IsActive, CreatedAt)
- `AccountInfo` struct (bez BearerToken — pro frontend)
- `GetAllAccounts(db)` → `[]Account`
- `GetAccountByUserID(db, userID)` → `*Account`
- `GetActiveAccounts(db)` → `[]Account` (WHERE is_active = 1)
- `AddAccount(db, userID, username, bearerToken)` — INSERT OR IGNORE
- `RemoveAccount(db, userID)` — DELETE from accounts only (snapshoty zůstanou)

### Krok 2: Backend — account management + scoping
**Soubor**: `app.go`

**2a. App struct** — přidat `selectedAccountID string`

**2b. startup()** — auto-import .env účtu do `accounts` tabulky:
- Resolve username → user_id pokud chybí v .env
- `AddAccount(db, userId, username, bearerToken)`
- Nastavit jako `selectedAccountID`

**2c. Nové Wails-bound metody:**
- `GetAccounts() []AccountInfo` — vrátí účty BEZ bearer tokenu
- `AddNewAccount(username, bearerToken) (string, error)` — vytvoří klienta, resolve username, uloží
- `RemoveAccountByID(userID) error`
- `SelectAccount(userID)` — přepne vybraný účet
- `GetSelectedAccount() string`

**2d. Scope existujících metod na `selectedAccountID`:**

- `GetFollowingList()` — JOIN na `following_snapshots` místo přímého SELECT z `users`:
  ```sql
  FROM users u INNER JOIN following_snapshots fs ON u.id = fs.target_user_id
  WHERE fs.source_user_id = ? AND fs.fetched_at = (SELECT MAX(fetched_at) ...)
  ```
- `GetStats()` — filtrovat `fetch_logs WHERE user_id = ?`, count z posledního snapshotu
- `GetDiff()` — použít `a.selectedAccountID` místo `LIMIT 1` query

**2e. Refaktor fetchování:**
- `fetchForAccount(acct Account) string` — extrahovaná logika z `FetchNow()`
- `FetchNow()` — zavolá `fetchForAccount` pro vybraný účet
- `FetchAllAccounts() string` — iteruje všechny aktivní účty
- `scheduler()` — volá `FetchAllAccounts()` místo `FetchNow()`

**2f. Notifikace** — zahrnout `@username` účtu do titulku notifikace

### Krok 3: Frontend — account selector + management modal
**Soubory**: `frontend/index.html`, `frontend/src/main.js`, `frontend/src/style.css`

**HTML:**
- Account selector dropdown v headeru (mezi `<h1>` a `#stats`)
- Tlačítko "+" pro otevření správy účtů
- Modal pro přidání/odebrání účtů (username + bearer token input)

**JS:**
- `loadAccounts()` — naplní dropdown z `GetAccounts()`
- `switchAccount(userID)` — `SelectAccount()` + reload dat
- `addAccount()` — `AddNewAccount()` + reload
- `removeAccount(userID)` — `RemoveAccountByID()` + reload
- `toggleAccountManager()` — toggle modalu
- Update `DOMContentLoaded` — volat `loadAccounts()` před `loadData()`

**CSS:**
- Styly pro `#account-select` (dropdown v dark theme)
- Tlačítko "+" (kulaté, blue accent)
- Modal overlay + content (dark card, 400px)
- Account list items s remove tlačítkem
- Add account form (inputs + button)

---

## Klíčové soubory

| Soubor | Změny |
|--------|-------|
| `db.go` | +accounts tabulka, +Account/AccountInfo typy, +CRUD funkce |
| `app.go` | +selectedAccountID, +account management metody, scope queries, refactor fetch |
| `frontend/index.html` | +account selector, +management modal |
| `frontend/src/main.js` | +account switching/management funkce |
| `frontend/src/style.css` | +account selector, modal styly |

---

## Bezpečnost

- `AccountInfo` struct (bez `BearerToken`) pro frontend — token se nikdy neposílá do JS
- Bearer tokeny uložené v SQLite (přijatelné pro lokální desktop app)
- `AddNewAccount` validuje token přes API call (resolve username) před uložením

## Migrace

- `CREATE TABLE IF NOT EXISTS` — bezpečné pro existující DB
- `.env` účet se auto-importuje při startu (idempotentní přes INSERT OR IGNORE)
- Existující snapshot data zůstanou netknutá

## Verifikace

1. `go build .` — kompilace bez chyb
2. Spuštění s existující `.env` → účet se automaticky objeví v dropdown
3. Přidání druhého účtu přes modal → resolve username, uloží se
4. Přepnutí účtu → Following tabulka, Stats, Changes se přepnou
5. Scheduler fetchuje pro všechny účty
6. Odebrání účtu → zmizí z dropdown, data v DB zůstanou
