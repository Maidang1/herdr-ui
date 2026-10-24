.PHONY: build test integration probe run window package dmg

build:
	CGO_ENABLED=0 ./scripts/go.sh build -trimpath -o bin/herdr-ui ./cmd/herdr-ui
	CGO_ENABLED=0 ./scripts/go.sh build -trimpath -o bin/m0-probe ./cmd/m0-probe

test:
	./scripts/go.sh test ./...
	./scripts/go.sh vet ./...

integration: build
	CGO_ENABLED=0 HERDR_UI_M0_WINDOW_BINARY="$(CURDIR)/bin/herdr-ui" ./scripts/go.sh test -tags integration -count=1 -v ./integration

probe:
	./scripts/go.sh run ./cmd/m0-probe

run: build
	./bin/herdr-ui

window: build
	./bin/herdr-ui -local-fixture

# The CLI resolves from the same exact mygo module revision as the application.
package:
	CGO_ENABLED=0 ./scripts/go.sh run github.com/egoist/mygo/cmd/mygo build -skip-dmg

# Universal (arm64 + x86_64) app and disk image, as the release workflow builds.
dmg:
	CGO_ENABLED=0 ./scripts/go.sh run github.com/egoist/mygo/cmd/mygo build -platform darwin/universal
