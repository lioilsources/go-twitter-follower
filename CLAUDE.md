# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

macOS desktop app (Wails v2) for Twitter/X following analysis. Fetches a user's following list via Twitter API v2, stores data in SQLite, displays a dashboard with metrics, and sends native macOS notifications on following changes. Runs 24/7 with hourly background fetching.

## Build & Run Commands

```bash
# Development mode (hot reload)
wails dev

# Build macOS .app bundle
wails build

# Generate Twitter API client from OpenAPI spec
make generate

# Plain Go build (without Wails runtime — for CI/testing)
go build .
```

No test suite exists in this project.

## Architecture

Single-package (`main`) Wails v2 application:

- **main.go** — Wails app entry point, config loading (`GetConfig`), embedded frontend assets.
- **app.go** — Wails backend: `App` struct with methods callable from JS (`GetFollowingList`, `GetStats`, `FetchNow`), background scheduler (1x/hour), and notification logic for following changes.
- **twitter.go** — Twitter API logic: `NewAuthClient` (bearer token auth), `ResolveUsername`, `GetFollowing` (single page with user.fields), `FetchAllFollowing` (full pagination with 3s rate limiting for 300 req/15min).
- **db.go** — SQLite storage via `modernc.org/sqlite`: schema init, `UpsertUser`, `SaveSnapshot`, `GetPreviousFollowingIDs` (for diff/notifications), `LogFetch`.
- **auth.go** — OAuth1 PIN-based authorization flow. Experimental; main flow uses OAuth2 bearer token.
- **frontend/** — HTML/CSS/JS dashboard embedded via `//go:embed`. Table with search, sort, and filtering. Calls Go methods via Wails bindings.
- **gen/twitter-client.gen.go** — Auto-generated types and client from `open-api-spec-all-components.yaml` via `oapi-codegen`. Do not edit manually; regenerate with `make generate`.

## Configuration

All credentials loaded from `.env` file via `godotenv`:

- `BEARER_TOKEN` — Required for main OAuth2 flow
- `TWITTER_USERNAME`, `TWITTER_USER_ID` — Target account
- `API_KEY`, `API_KEY_SECRET`, `ACCESS_TOKEN`, `ACCESS_TOKEN_SECRET` — OAuth1 credentials

## Key Twitter API Endpoints

- `GET /2/users/by/username/{username}` — Resolve username to user ID
- `GET /2/users/{id}/following` — Get following list (paginated, 300 req/15min)

## Dependencies

- `github.com/wailsapp/wails/v2` — macOS desktop app framework
- `modernc.org/sqlite` — Pure Go SQLite driver (no CGO)
- `github.com/gen2brain/beeep` — Native desktop notifications
- `github.com/deepmap/oapi-codegen` — OpenAPI client/types generation
- `github.com/dghubble/oauth1` — OAuth1 authentication
- `github.com/joho/godotenv` — .env file loading
