# 04 — System Tray (Menu Bar Mode)

## Goal
Add a macOS menu bar (system tray) icon so the app runs persistently in the background. Closing the window hides it instead of quitting — the app keeps fetching hourly. Users interact via tray menu or by clicking the icon to restore the dashboard.

## Research Findings
- Wails v2 has **no built-in systray** support.
- `github.com/energye/systray` (fork of getlantern/systray) works well on macOS and supports Go embed for the icon.
- CGO is already enabled (required by Wails for macOS WebView), so no build config changes needed.
- Wails v2.11.0 provides `HideWindowOnClose: true` option — hides window on close instead of quitting.
- `github.com/wailsapp/wails/v2/pkg/runtime` exposes `WindowShow(ctx)`, `WindowHide(ctx)`, `Quit(ctx)`.

## Implementation Steps

### 1. Add dependency
```bash
go get github.com/energye/systray
```

### 2. Create tray icon — `build/trayicon.png`
- 22×22 px monochrome template PNG for macOS menu bar.
- Simple circle-with-dot (tracking/eye motif).

### 3. Create `tray.go`
- `//go:embed build/trayicon.png` for the icon bytes.
- `initSystray()` — calls `systray.Run(onTrayReady, onExit)`.
- `onTrayReady()` — sets icon, tooltip, adds menu items:
  - **Show Dashboard** → `runtime.WindowShow(a.ctx)`
  - **Fetch Now** → `go a.FetchAllAccounts()`
  - **Last fetch: --** (disabled, updated after each fetch)
  - **Quit** → `systray.Quit()` + `runtime.Quit(a.ctx)`
- `SetOnClick` → show window on tray icon click.
- `updateTrayLastFetch()` — updates the "Last fetch" menu item title.

### 4. Edit `main.go`
- Add `HideWindowOnClose: true` to `options.App{}`.

### 5. Edit `app.go`
- Add `trayLastFetchItem *systray.MenuItem` field to `App` struct.
- Call `go a.initSystray()` in `startup()`.
- Call `systray.Quit()` in `shutdown()`.
- Call `a.updateTrayLastFetch()` at end of `fetchForAccount()`.

## Lifecycle Flow
```
App starts → Wails window + systray icon appear
User closes window → window hides, tray icon stays, scheduler continues
User clicks tray icon / "Show Dashboard" → window reappears
User clicks "Quit" in tray menu → systray.Quit() + runtime.Quit() → full exit
```

## Files Changed
- `go.mod` / `go.sum` — added `github.com/energye/systray v1.0.3`
- `build/trayicon.png` — new 22×22 monochrome icon
- `tray.go` — new file, systray logic
- `main.go` — added `HideWindowOnClose: true`
- `app.go` — added tray field, init call, shutdown call, fetch update
