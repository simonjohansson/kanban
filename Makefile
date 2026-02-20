.PHONY: test test-backend test-cli test-cli-e2e

test: test-backend test-cli

test-backend:
	$(MAKE) -C backend test

test-cli:
	cd cli && go test ./... -count=1

test-cli-e2e:
	cd cli && go test -v ./... -count=1 -run 'TestE2EKBMultiProjectFlow|TestE2EKBWatchAllAndFiltered'
