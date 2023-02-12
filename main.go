package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"go-twitter-follower/gen"

	"github.com/deepmap/oapi-codegen/pkg/securityprovider"
	"github.com/joho/godotenv"
)

const (
	scheme = "https"
	host   = "api.twitter.com"
)

// todo: add field annotation `env:"BEARER_TOKEN"`
// https://stackoverflow.com/questions/10858787/what-are-the-uses-for-struct-tags-in-go/30889373#30889373
type Config struct {
	BearerToken string
	Username    string
}

func GetConfig() *Config {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	return &Config{
		BearerToken: os.Getenv("BEARER_TOKEN"),
		Username:    os.Getenv("TWITTER_USERNAME"),
	}
}

func NewAuthClient(bearerToken string) (*gen.ClientWithResponses, error) {
	// Example BearerToken
	// See: https://swagger.io/docs/specification/authentication/bearer-authentication/
	bearerTokenProvider, bearerTokenProviderErr := securityprovider.NewSecurityProviderBearerToken(bearerToken)
	if bearerTokenProviderErr != nil {
		log.Fatal(bearerTokenProviderErr)
	}

	client, err := gen.NewClientWithResponses(fmt.Sprintf("%s://%s", scheme, host), gen.WithRequestEditorFn(bearerTokenProvider.Intercept))
	if err != nil {
		log.Fatal(fmt.Errorf("%w", err))
	}

	return client, nil
}

func GetUserIdFromUsername(client *gen.ClientWithResponses, username string) {
	res, err := client.FindUserByUsernameWithResponse(context.Background(), username, &gen.FindUserByUsernameParams{
		UserFields: nil,
	})
	if err != nil {
		log.Fatal(fmt.Errorf("%w", err))
	}
	if res.StatusCode() != http.StatusOK {
		log.Fatal(*res.JSONDefault.Status, ": ", *res.JSONDefault.Detail)
	}

	fmt.Printf("Id: %s\n", res.JSON200.Data.Id)
}

func main() {
	config := GetConfig()
	client, err := NewAuthClient(config.BearerToken)
	if err != nil {
		log.Fatal(fmt.Errorf("%w", err))
	}
	GetUserIdFromUsername(client, config.Username)
}
