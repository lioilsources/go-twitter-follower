# 20/5/2023
- store json responses to file system

# 1/5/2023
- get followers by id
- pagination
- rate limiting

# 12/2/2023
- .env support

# 11/2/2023

## Mission
Follow users with great and stable content to be followed back.
You need to know someone, who is followed by them.

## Alghoritm
Get informations from your account
- get user id from user name
- endpoint: https://api.twitter.com/2/users/by/username/TWITTER_USERNAME
- get followers by id
- endpoint: https://api.twitter.com/2/users/TWITTER_ID/followers
- pagination
- rate limit
- get tweets by user id
- endpoint: https://api.twitter.com/2/users/TWITTER_ID/tweets?max_results=100

Do the same for the username you wanna use
- filter out same followers
- out: number of follower to follow
- loop: all followers tweets sorted by number
- endpoint: POST Follow user; top 5-10 per day

## GoLang
```sh
go mod init follower
```

```sh
go get github.com/deepmap/oapi-codegen/pkg/securityprovider
```

```sh
go get github.com/Netflix/go-env
```

## Twitter Developer Portal
- Bearer Token: https://developer.twitter.com/en/portal/dashboard

## OpenAPI
- twitter json OpenApi spec: https://api.twitter.com/2/openapi.json
- json to jaym converter: https://editor.swagger.io/#/

```sh
go install github.com/deepmap/oapi-codegen/cmd/oapi-codegen@latest
```

```sh
./oapi-codegen --generate types,client --package gen open-api-spec-all-components.yaml > gen/twitter-client.gen.go
```
