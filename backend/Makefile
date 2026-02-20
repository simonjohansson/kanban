OPENAPI_SPEC := api/openapi.yaml
GO_CLIENT_OUT := internal/gen/client/client.gen.go

.PHONY: test openapi-sync openapi-validate openapi-gen-go-client

test:
	go test ./... -count=1

openapi-sync:
	go run ./cmd/export-openapi -out $(OPENAPI_SPEC)

openapi-validate: openapi-sync
	go run github.com/getkin/kin-openapi/cmd/validate@latest $(OPENAPI_SPEC)

openapi-gen-go-client: openapi-sync
	mkdir -p $(dir $(GO_CLIENT_OUT))
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest \
		-generate types,client \
		-package client \
		-o $(GO_CLIENT_OUT) \
		$(OPENAPI_SPEC)
