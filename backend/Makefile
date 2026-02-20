OPENAPI_SPEC := api/openapi.yaml
GO_CLIENT_OUT := gen/client/client.gen.go

.PHONY: test test-e2e test-e2e-verbose test-no-e2e sqlc-generate openapi-sync openapi-validate openapi-gen-go-client

test:
	go test ./... -count=1

test-e2e:
	go test ./... -count=1 -run 'TestE2EBlackBoxServerProcess|TestE2EGeneratedClientFlow'

test-e2e-verbose:
	go test -v ./... -count=1 -run 'TestE2EBlackBoxServerProcess|TestE2EGeneratedClientFlow'

test-no-e2e:
	go test ./... -count=1 -skip 'TestE2EBlackBoxServerProcess|TestE2EGeneratedClientFlow'

sqlc-generate:
	go run github.com/sqlc-dev/sqlc/cmd/sqlc@latest generate -f sqlc.yaml

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
