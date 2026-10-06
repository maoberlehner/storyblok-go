# storyblok-go

A marketing website rendered in Go from Storyblok content, with live preview in the Visual Editor.

## Requirements

- Go 1.27+
- [mkcert](https://github.com/FiloSottile/mkcert) (the Visual Editor needs an HTTPS preview URL)

## Run

```sh
echo 'STORYBLOK_PREVIEW_TOKEN=<preview token of your space>' > .env
make run
```

`make run` downloads the fonts and creates a local certificate on first run, then serves https://localhost:8080 with the dev toolbar enabled (press `S` and click a block to open it in Storyblok).

For live preview, set `https://localhost:8080/` as the preview URL in your space settings.

## Test

```sh
make test
```
