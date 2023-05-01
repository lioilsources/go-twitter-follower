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

func GetUserIdFromUsername(client *gen.ClientWithResponses, username string) string {
	res, err := client.FindUserByUsernameWithResponse(context.Background(), username, &gen.FindUserByUsernameParams{
		UserFields: nil,
	})
	if err != nil {
		log.Fatal(fmt.Errorf("%w", err))
	}
	if res.StatusCode() != http.StatusOK {
		log.Fatal(*res.JSONDefault.Status, ": ", *res.JSONDefault.Detail)
	}

	userId := res.JSON200.Data.Id
	fmt.Printf("Id: %s\n", userId)
	return userId
}

func GetFollowers(client *gen.ClientWithResponses, userId string, pagination_token *string) (*[]gen.User, string) {
	params := &gen.UsersIdFollowersParams{
		Expansions:      nil,
		MaxResults:      nil,
		PaginationToken: nil,
		TweetFields:     nil,
		UserFields:      nil,
	}
	if pagination_token != nil {
		params.PaginationToken = pagination_token
	}

	res, err := client.UsersIdFollowersWithResponse(context.Background(), userId, params)
	if err != nil {
		log.Fatal(fmt.Errorf("%w", err))
	}
	if res.StatusCode() != http.StatusOK {
		log.Fatal(*res.JSONDefault.Status, ": ", *res.JSONDefault.Detail)
	}

	// for _, user := range *res.JSON200.Data {
	// 	fmt.Printf("Username: %s\n", user.Username)
	// }

	next_token := *res.JSON200.Meta.NextToken
	return res.JSON200.Data, next_token
}

func main() {
	config := GetConfig()
	client, err := NewAuthClient(config.BearerToken)
	if err != nil {
		log.Fatal(fmt.Errorf("%w", err))
	}

	// get user id from user name
	userId := GetUserIdFromUsername(client, config.Username)

	// pagination without rate limiting
	// get followers by id
	all_followers := make([]gen.User, 0)
	var pagination_token *string = nil

	rate_limit := 10 // 10 per 15 min
	for {
		rate_limit -= 1

		folowers, pagination_token := GetFollowers(client, userId, pagination_token)

		all_followers = append(all_followers, *folowers...)

		if pagination_token == "" {
			break
		}
		if rate_limit == 0 {
			break
		}
	}

	num_followers := len(all_followers)
	if rate_limit == 0 {
		fmt.Printf("Partial followers: %d\n", num_followers)
	} else {
		fmt.Printf("Total followers: %d\n", num_followers)
	}
}
