package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"go-twitter-follower/gen"

	"github.com/deepmap/oapi-codegen/pkg/securityprovider"
)

const (
	scheme = "https"
	host   = "api.twitter.com"

	// https://docs.x.com/x-api/fundamentals/rate-limits
	// GET /2/users/:id/following | 300 reqs/15 minutes (per app & per user)
	rate_limit = 1000 * time.Millisecond * 3 // 300 per 15 min
)

func NewAuthClient(bearerToken string) (*gen.ClientWithResponses, error) {
	bearerTokenProvider, bearerTokenProviderErr := securityprovider.NewSecurityProviderBearerToken(bearerToken)
	if bearerTokenProviderErr != nil {
		return nil, fmt.Errorf("creating bearer token provider: %w", bearerTokenProviderErr)
	}

	client, err := gen.NewClientWithResponses(fmt.Sprintf("%s://%s", scheme, host), gen.WithRequestEditorFn(bearerTokenProvider.Intercept))
	if err != nil {
		return nil, fmt.Errorf("creating client: %w", err)
	}

	return client, nil
}

func ResolveUsername(client *gen.ClientWithResponses, username string) (string, error) {
	res, err := client.FindUserByUsernameWithResponse(context.Background(), username, &gen.FindUserByUsernameParams{
		UserFields: nil,
	})
	if err != nil {
		return "", fmt.Errorf("finding user by username: %w", err)
	}
	if res.StatusCode() != http.StatusOK {
		return "", fmt.Errorf("API error: %s: %s", *res.JSONDefault.Status, *res.JSONDefault.Detail)
	}

	return res.JSON200.Data.Id, nil
}

func GetFollowing(client *gen.ClientWithResponses, userId string, pagination_token *string) (*[]gen.User, *string, error) {
	userFields := gen.UserFieldsParameter{
		"public_metrics",
		"description",
		"created_at",
		"verified",
		"verified_type",
		"profile_image_url",
		"location",
	}
	params := &gen.UsersIdFollowingParams{
		Expansions:      nil,
		MaxResults:      nil,
		PaginationToken: nil,
		TweetFields:     nil,
		UserFields:      &userFields,
	}
	if pagination_token != nil {
		params.PaginationToken = pagination_token
	}

	log.Printf("[fetch] GET /2/users/%s/following (pagination: %v)", userId, pagination_token != nil)
	res, err := client.UsersIdFollowingWithResponse(context.Background(), userId, params)
	if err != nil {
		return nil, nil, fmt.Errorf("API request failed: %w", err)
	}
	log.Printf("[fetch] Response: HTTP %d (%d bytes)", res.StatusCode(), len(res.Body))
	if res.StatusCode() != http.StatusOK {
		log.Printf("[fetch] Error body: %s", string(res.Body))
		if res.JSONDefault != nil && res.JSONDefault.Status != nil && res.JSONDefault.Detail != nil {
			return nil, nil, fmt.Errorf("API error %d: %s: %s", res.StatusCode(), *res.JSONDefault.Status, *res.JSONDefault.Detail)
		}
		return nil, nil, fmt.Errorf("API error %d: %s", res.StatusCode(), string(res.Body))
	}
	if res.JSON200 == nil || res.JSON200.Data == nil {
		return nil, nil, fmt.Errorf("API returned empty response")
	}

	next_token := res.JSON200.Meta.NextToken
	return res.JSON200.Data, next_token, nil
}

// FetchAllFollowing fetches the complete following list with pagination and rate limiting.
func FetchAllFollowing(client *gen.ClientWithResponses, userId string) ([]gen.User, error) {
	all := make([]gen.User, 0)

	// first call without rate limit delay
	f, pagination_token, err := GetFollowing(client, userId, nil)
	if err != nil {
		return nil, err
	}
	all = append(all, *f...)
	log.Printf("Counting followings: %d", len(all))

	if pagination_token == nil {
		return all, nil
	}

	rate_limiter := time.Tick(rate_limit)
	for range rate_limiter {
		f, pagination_token, err = GetFollowing(client, userId, pagination_token)
		if err != nil {
			return nil, err
		}
		all = append(all, *f...)
		log.Printf("Counting followings: %d", len(all))

		if pagination_token == nil {
			break
		}
	}

	return all, nil
}
