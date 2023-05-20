package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"go-twitter-follower/gen"

	"github.com/deepmap/oapi-codegen/pkg/securityprovider"
	"github.com/joho/godotenv"
)

const (
	scheme = "https"
	host   = "api.twitter.com"

	// https://developer.twitter.com/en/docs/twitter-api/rate-limits#v2-limits
	// GET_2_lists_id_followers | 10 reqs/15 minutes | 28,800/30days
	rate_limit = 1000 * time.Millisecond * 90 // 10 per 15 min
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

// deprecated
// I don't work with followers but followings to make diff against popular account.
// Number of followers is important metric of popularity but b/c of rate limiting, it is time consuming
// to get total number fast.
func GetFollowers(client *gen.ClientWithResponses, userId string, pagination_token *string) (*[]gen.User, *string) {
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

	// store json response
	json, err := json.MarshalIndent(res.JSON200.Data, "", "\t")
	if err != nil {
		log.Fatal(fmt.Errorf("%w", err))
	}
	pagination := "0"
	if pagination_token != nil {
		pagination = *pagination_token
	}
	path := fmt.Sprintf("res/GetFollowers/%s", userId)
	StoreResponse(path, pagination, string(json))

	next_token := res.JSON200.Meta.NextToken
	return res.JSON200.Data, next_token
}

func GetFollowing(client *gen.ClientWithResponses, userId string, pagination_token *string) (*[]gen.User, *string) {
	params := &gen.UsersIdFollowingParams{
		Expansions:      nil,
		MaxResults:      nil,
		PaginationToken: nil,
		TweetFields:     nil,
		UserFields:      nil,
	}
	if pagination_token != nil {
		params.PaginationToken = pagination_token
	}

	res, err := client.UsersIdFollowingWithResponse(context.Background(), userId, params)
	if err != nil {
		log.Fatal(fmt.Errorf("%w", err))
	}
	if res.StatusCode() != http.StatusOK {
		log.Fatal(*res.JSONDefault.Status, ": ", *res.JSONDefault.Detail)
	}

	// store json response
	json, err := json.MarshalIndent(res.JSON200.Data, "", "\t")
	if err != nil {
		log.Fatal(fmt.Errorf("%w", err))
	}
	pagination := "0"
	if pagination_token != nil {
		pagination = *pagination_token
	}
	path := fmt.Sprintf("res/GetFollowing/%s", userId)
	StoreResponse(path, pagination, string(json))

	next_token := res.JSON200.Meta.NextToken
	return res.JSON200.Data, next_token
}

func StoreResponse(path, filename string, data string) {
	err := os.MkdirAll(path, os.ModePerm)
	if err != nil {
		log.Fatal(fmt.Errorf("%w", err))
	}

	f, err := os.Create(fmt.Sprintf("%s/%s.json", path, filename))
	if err != nil {
		log.Fatal(fmt.Errorf("%w", err))
	}

	defer f.Close()

	n3, err := f.WriteString(data)
	if err != nil {
		log.Fatal(fmt.Errorf("%w", err))
	}

	fmt.Printf("wrote %d bytes\n", n3)
	f.Sync()
}

func main() {
	config := GetConfig()
	client, err := NewAuthClient(config.BearerToken)
	if err != nil {
		log.Fatal(fmt.Errorf("%w", err))
	}

	// get user id from user name
	userId := GetUserIdFromUsername(client, config.Username)

	// pagination with rate limiting
	// get followers by id
	all := make([]gen.User, 0)

	// first call without limit
	f, pagination_token := GetFollowing(client, userId, nil)
	all = append(all, *f...)
	fmt.Printf("[%s] Counting followings: %d\n", time.Now().Format("2006-01-02 15:04:05"), len(all))

	rate_limiter := time.Tick(rate_limit)
	for range rate_limiter {
		f, pagination_token = GetFollowing(client, userId, pagination_token)
		all = append(all, *f...)
		fmt.Printf("[%s] Counting followings: %d\n", time.Now().Format("2006-01-02 15:04:05"), len(all))

		if pagination_token == nil {
			break
		}
	}
	num := len(all)
	fmt.Printf("Total followings: %d\n", num)
}
