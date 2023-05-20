generate:
	oapi-codegen --generate types,client --package gen open-api-spec-all-components.yaml > gen/twitter-client.gen.go