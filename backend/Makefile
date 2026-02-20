OPENAPI_SPEC := api/openapi.yaml
GO_CLIENT_OUT := internal/gen/client/client.gen.go

.PHONY: test openapi-validate openapi-gen-go-client

test:
	go test ./... -count=1

openapi-validate:
	go run github.com/getkin/kin-openapi/cmd/validate@latest $(OPENAPI_SPEC)

openapi-gen-go-client:
	mkdir -p $(dir $(GO_CLIENT_OUT))
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest \
		-generate types,client \
		-package client \
		-o $(GO_CLIENT_OUT) \
		$(OPENAPI_SPEC)
