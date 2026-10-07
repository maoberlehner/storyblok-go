.PHONY: run up test fmt schema-validate seed benchmark certs skills

CERT := .certs/localhost.pem
KEY := .certs/localhost-key.pem

# Serves HTTPS because the Visual Editor only loads https preview URLs.
run: $(CERT)
	set -a && . ./.env && set +a && DEV_TOOLBAR=1 TLS_CERT_FILE=$(CERT) TLS_KEY_FILE=$(KEY) go run .

# Production-like stack with the caching proxy on https://localhost:8443.
up: $(CERT)
	docker compose up --build

test:
	go test ./...

fmt:
	gofmt -w .
	npx -y oxfmt@0

schema-validate:
	go run ./cmd/storyblok-schema validate

# Creates or updates the demo stories in STORYBLOK_SPACE. Apply the schemas first.
seed:
	set -a && . ./.env && set +a && go run ./cmd/storyblok-seed

# Compares ASSET_DELIVERY modes in Chrome against the seeded demo pages.
benchmark: $(CERT)
	cd tools/asset-benchmark && npm ci --silent
	set -a && . ./.env && set +a && node tools/asset-benchmark/bench.mjs

certs $(CERT):
	mkdir -p .certs
	mkcert -cert-file $(CERT) -key-file $(KEY) localhost 127.0.0.1 ::1

# Links the skill shipped with the globally installed CLI, so it stays on the
# installed version. The global path is machine-specific, hence not committed.
skills:
	mkdir -p .agents/skills
	ln -sfn "$$(npm root -g)/@markus/storyblok-agent/skills/storyblok-content-ops" .agents/skills/storyblok-content-ops
