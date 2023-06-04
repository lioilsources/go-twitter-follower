package main

import (
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/dghubble/oauth1"
	"github.com/dghubble/oauth1/twitter"
)

func GetAccessTokenFromConfig(config Config) (string, string) {
	// if PIN valid
	requestAccessToken := config.AccessToken
	requestAccessSecret := config.AccessTokenSecret

	return requestAccessToken, requestAccessSecret
}

func FetchAccessToken(consumerKey, consumerSecret string) (string, string) {
	//oauth1 PIN based authentication
	callbackUrl := "oob" // out of band

	loginConfig := oauth1.Config{
		ConsumerKey:    consumerKey,
		ConsumerSecret: consumerSecret,
		CallbackURL:    callbackUrl,
		Endpoint:       twitter.AuthenticateEndpoint,
	}

	// login
	loginToken, _, err := loginConfig.RequestToken()
	if err != nil {
		log.Fatal(fmt.Errorf("%w", err))
	}
	authorizationURL, err := loginConfig.AuthorizationURL(loginToken)
	if err != nil {
		log.Fatal(fmt.Errorf("%w", err))
	}
	fmt.Printf("Open this URL in your browser:\n%s\n", authorizationURL.String())

	fmt.Println("Login token")
	fmt.Println(loginToken)

	// recive PIN
	fmt.Printf("Paste your PIN here: ")
	var verifier string
	_, err = fmt.Scanf("%s", &verifier)
	if err != nil {
		log.Fatal(fmt.Errorf("%w", err))
	}
	// Twitter ignores the oauth_signature on the access token request. The user
	// to which the request (temporary) token corresponds is already known on the
	// server. The request for a request token earlier was validated signed by
	// the consumer. Consumer applications can avoid keeping request token state
	// between authorization granting and callback handling.
	accessToken, accessSecret, err := loginConfig.AccessToken(loginToken, "secret does not matter", verifier)
	if err != nil {
		log.Fatal(fmt.Errorf("%w", err))
	}

	return accessToken, accessSecret
}

func GetAuthenticatedClient(consumerKey, consumerSecret, accessToken, accessSecret string) *http.Client {
	authorizeRequestToken := oauth1.NewToken(accessToken, accessSecret)
	fmt.Println("Consumer was granted an access token to act on behalf of a user.")
	fmt.Printf("token: %s\nsecret: %s\n", authorizeRequestToken.Token, authorizeRequestToken.TokenSecret)

	// oauth1 authorize request
	requestAccessToken := authorizeRequestToken.Token
	requestAccessSecret := authorizeRequestToken.TokenSecret

	requestConfig := oauth1.NewConfig(consumerKey, consumerSecret)
	requestToken := oauth1.NewToken(requestAccessToken, requestAccessSecret)

	// httpClient will automatically authorize http.Request's
	httpClient := requestConfig.Client(oauth1.NoContext, requestToken)

	return httpClient
}

func AuthenticatedOAuth1Request(httpClient *http.Client, path string) {
	resp, err := httpClient.Get(path)
	if err != nil {
		log.Fatal(fmt.Errorf("%w", err))
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(fmt.Errorf("%w", err))
	}
	fmt.Println(len(body))
	fmt.Printf("Raw Response Body:\n%v\n", string(body))
}

func Flow() {
	config := GetConfig()

	consumerKey := config.ApiKey
	consumerSecret := config.ApiKeySecret

	// fetch access token
	accessToken, accessSecret := FetchAccessToken(consumerKey, consumerSecret)

	// get authenticated client
	httpClient := GetAuthenticatedClient(consumerKey, consumerSecret, accessToken, accessSecret)

	// make authenticated request
	path := fmt.Sprintf("https://api.twitter.com/2/users/%s/timelines/reverse_chronological", config.UserId)
	AuthenticatedOAuth1Request(httpClient, path)
}
