.PHONY: run test fmt fonts certs

CERT := .certs/localhost.pem
KEY := .certs/localhost-key.pem

# Serves HTTPS because the Visual Editor only loads https preview URLs.
run: static/fonts/ABCMarfa-Regular.woff2 $(CERT)
	set -a && . ./.env && set +a && DEV_TOOLBAR=1 TLS_CERT_FILE=$(CERT) TLS_KEY_FILE=$(KEY) go run .

test:
	go test ./...

fmt:
	gofmt -w .
	npx -y oxfmt@0

fonts static/fonts/ABCMarfa-Regular.woff2:
	./scripts/fetch-fonts.sh

certs $(CERT):
	mkdir -p .certs
	mkcert -cert-file $(CERT) -key-file $(KEY) localhost 127.0.0.1 ::1
