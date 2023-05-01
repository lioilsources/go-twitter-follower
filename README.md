# 12/2/2023
- .env support

# 11/2/2023

## Mission
Follow users with great and stable content to be followed back.
You need to know someone, who is followed by them.

## Alghoritm
Get informations from your account
- endpoint: https://api.twitter.com/2/users/by/username/TWITTER_USERNAME
- get user id from user name
- endpoint: https://api.twitter.com/2/users/TWITTER_ID/followers
- get followers by id
- pagination
- endpoint: https://api.twitter.com/2/users/TWITTER_ID/tweets?max_results=100
- get tweets by user id

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
./oapi-codegen --generate types,client --package gen open-api-spec-all-components.json > twitter-user-by-username.gen.go
```
