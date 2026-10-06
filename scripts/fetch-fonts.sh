#!/usr/bin/env sh
# Downloads the licensed ABC Marfa fonts used by storyblok.com into static/fonts.
set -eu
cd "$(dirname "$0")/../static/fonts"
for weight in Regular Medium Bold Black; do
  curl -fsSL -o "ABCMarfa-$weight.woff2" "https://www.storyblok.com/fonts/ABCMarfa-$weight.woff2"
done
