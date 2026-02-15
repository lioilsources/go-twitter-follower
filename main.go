package main

import (
	"embed"
	"io/fs"
	"log"
	"os"

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
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
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
		Title:     "Twitter Following Tracker",
		Width:     1024,
		Height:    700,
		MinWidth:  800,
		MinHeight: 500,
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
