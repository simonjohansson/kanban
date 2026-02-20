module github.com/simonjohansson/kanban/cli

go 1.26

require (
	github.com/gorilla/websocket v1.5.3
	github.com/simonjohansson/kanban/backend v0.0.0
	github.com/spf13/cobra v1.10.1
	github.com/stretchr/testify v1.11.1
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/apapsch/go-jsonmerge/v2 v2.0.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/oapi-codegen/runtime v1.1.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
)

replace github.com/simonjohansson/kanban/backend => ../backend
