.PHONY: run up generate test fmt schema-validate seed certs skills

CERT := .certs/localhost.pem
KEY := .certs/localhost-key.pem

# Serves HTTPS because the Visual Editor only loads https preview URLs.
run: generate $(CERT)
	set -a && . ./.env && set +a && SITE_URL="$${SITE_URL:-https://localhost:8080}" FORM_SECRET="$${FORM_SECRET:-dev-form-secret}" DEV_TOOLBAR=1 TLS_CERT_FILE=$(CERT) TLS_KEY_FILE=$(KEY) go run .

# Production-like stack with the caching proxy on https://localhost:8443.
up: $(CERT)
	docker compose up --build

# The generated *_templ.go files are committed, so plain go commands work too.
generate:
	go tool templ generate -path internal

test: generate
	go test ./...

fmt:
	go tool templ fmt internal
	gofmt -w .
	npx -y oxfmt@0

schema-validate: generate
	go run ./cmd/storyblok-schema validate

# Creates or updates the demo stories in STORYBLOK_SPACE. Apply the schemas first.
seed:
	set -a && . ./.env && set +a && go run ./cmd/storyblok-seed

certs $(CERT):
	mkdir -p .certs
	mkcert -cert-file $(CERT) -key-file $(KEY) localhost 127.0.0.1 ::1

# Links the skill shipped with the globally installed CLI, so it stays on the
# installed version. The global path is machine-specific, hence not committed.
skills:
	mkdir -p .agents/skills
	ln -sfn "$$(npm root -g)/@markus/storyblok-agent/skills/storyblok-content-ops" .agents/skills/storyblok-content-ops
