.PHONY: build test integration probe window package

build:
	CGO_ENABLED=0 go build -trimpath -o bin/herdr-ui ./cmd/herdr-ui
	CGO_ENABLED=0 go build -trimpath -o bin/m0-probe ./cmd/m0-probe

test:
	go test ./...
	go vet ./...

integration: build
	CGO_ENABLED=0 HERDR_UI_M0_WINDOW_BINARY="$(CURDIR)/bin/herdr-ui" go test -tags integration -count=1 -v ./integration

probe:
	go run ./cmd/m0-probe

window: build
	./bin/herdr-ui -local-fixture

# The CLI resolves from the same exact mygo module revision as the application.
package:
	CGO_ENABLED=0 go run github.com/egoist/mygo/cmd/mygo build -skip-dmg
