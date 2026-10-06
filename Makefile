.PHONY: run test fonts

run: static/fonts/ABCMarfa-Regular.woff2
	set -a && . ./.env && set +a && go run .

test:
	go test ./...

fonts static/fonts/ABCMarfa-Regular.woff2:
	./scripts/fetch-fonts.sh
