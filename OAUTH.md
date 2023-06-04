1. OAuth 1.0a User Context
- PIN based, you don't need to expose server
2. OAuth 2.0 App Only
- Bearer Token
3. OAuth 2.0 Authorization Code with PKCE
- need server to folow flow
- scopes
- unable to call GET Bookmarks endpoint

# Credentials
## OAUTH1
CONSUMER_KEY=""
CONSUMER_SECRET=""
CALLBACK_URL="oob"
### Temporary
ACCESS_TOKEN="oauth_token"
ACCESS_TOKEN_SECRET="oauth_token_secret"

## OAUTH2
oauth_client_key
oauth_client_secret
oauth_callback

# OAUTH1 3-legged OAuth Flow
"github.com/dghubble/oauth1" handle this

Step 1: POST oauth/request_token
req {
    auth-header: oauth_consumer_key/secret
    ?oauth_callback=oob
}
res {
    oauth_token
    oauth_token_secret
    oauth_callback_confirmed=true
}

Step 2: GET oauth/authorize
req {
    ?oauth_token
}
shows 7digit pin

Step 3: POST oauth/access_token
req {    
    ?oauth_token
    &oauth_verifier=PIN
}
res {
    oauth_token
    oauth_token_secret
}

Step 4: Verification  GET account/verify_credentials
res {
    oauth_token
    oauth_token_secret
}

# OAUTH2 PKCE
Authorization Url: https://api.twitter.com/oauth/authorize
Access Token Url:  https://api.twitter.com/oauth/token
Callback Url:  https://insomnia.rest (does not matter)

# References
Twitter Endpoint's Authorization mapping: https://developer.twitter.com/en/docs/authentication/guides/v2-authentication-mapping
Twitter Doc: https://developer.twitter.com/en/docs/authentication/overview
Insomnia PKCE: https://insomnia.rest/blog/oauth2-github-api
PIN-based OAuth1: https://developer.twitter.com/en/docs/authentication/oauth-1-0a/pin-based-oauth
Twitter oauth urls: https://developer.twitter.com/en/docs/authentication/api-reference/request_token
Blog: https://dev.to/namick/the-missing-guide-to-twitter-oauth-user-authorization-9lh
OAuth1 Examples: https://github.com/dghubble/oauth1/tree/v0.7.2/examples
Bearer Provider: https://github.com/deepmap/oapi-codegen/blob/master/pkg/securityprovider/securityprovider.go
