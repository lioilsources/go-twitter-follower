package main

import (
	"embed"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed frontend
var assets embed.FS

type Config struct {
	BearerToken string
	Username    string
	UserId      string

	// OAuth1 credentials
	ApiKey       string
	ApiKeySecret string

	// OAuth1 after PIN entered access token
	AccessToken       string
	AccessTokenSecret string
}

func GetConfig() *Config {
	// Try loading .env from current dir, then from executable's dir
	if err := godotenv.Load(); err != nil {
		// Try relative to executable location
		if exePath, exeErr := os.Executable(); exeErr == nil {
			exeDir := filepath.Dir(exePath)
			// For macOS .app bundles, go up from Contents/MacOS/
			for _, rel := range []string{".env", "../../.env", "../../../.env"} {
				if godotenv.Load(filepath.Join(exeDir, rel)) == nil {
					break
				}
			}
		}
	}

	return &Config{
		BearerToken:       os.Getenv("BEARER_TOKEN"),
		Username:          os.Getenv("TWITTER_USERNAME"),
		UserId:            os.Getenv("TWITTER_USER_ID"),
		ApiKey:            os.Getenv("API_KEY"),
		ApiKeySecret:      os.Getenv("API_KEY_SECRET"),
		AccessToken:       os.Getenv("ACCESS_TOKEN"),
		AccessTokenSecret: os.Getenv("ACCESS_TOKEN_SECRET"),
	}
}

func main() {
	app := NewApp()

	frontendFS, fsErr := fs.Sub(assets, "frontend")
	if fsErr != nil {
		log.Fatal(fsErr)
	}

	err := wails.Run(&options.App{
		Title:            "Twitter Following Tracker",
		Width:            1024,
		Height:           700,
		MinWidth:         800,
		MinHeight:        500,
		HideWindowOnClose: true,
		AssetServer: &assetserver.Options{
			Assets: frontendFS,
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
		},
		Mac: &mac.Options{
			TitleBar: mac.TitleBarHiddenInset(),
			About: &mac.AboutInfo{
				Title:   "Twitter Following Tracker",
				Message: "Follower analysis tool for X/Twitter",
			},
		},
	})

	if err != nil {
		log.Fatal(err)
	}
}
