package main

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"

	"storyblok-go-website/internal/mapi"
)

const seedImageWidth, seedImageHeight = 2400, 1600

// seedImages are abstract placeholder images, generated so the repository
// holds no binary demo content. Each is a diagonal gradient between two
// colors.
var seedImages = map[string][2]color.RGBA{
	"launch-hero":   {{0x18, 0x4d, 0xb5, 0xff}, {0x7c, 0xc4, 0xf2, 0xff}},
	"partners-hero": {{0x1d, 0x6b, 0x34, 0xff}, {0xf2, 0xd0, 0x7c, 0xff}},
	"team":          {{0x91, 0x1d, 0x1b, 0xff}, {0xf4, 0xf5, 0xf8, 0xff}},
}

func renderSeedImage(colors [2]color.RGBA) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, seedImageWidth, seedImageHeight))
	span := seedImageWidth + seedImageHeight
	mix := func(a, b uint8, t float64) uint8 { return uint8(float64(a) + (float64(b)-float64(a))*t) }
	for y := range seedImageHeight {
		for x := range seedImageWidth {
			t := float64(x+y) / float64(span)
			from, to := colors[0], colors[1]
			img.SetRGBA(x, y, color.RGBA{mix(from.R, to.R, t), mix(from.G, to.G, t), mix(from.B, to.B, t), 0xff})
		}
	}
	var buf bytes.Buffer
	err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85})
	return buf.Bytes(), err
}

// seedAsset returns the asset field value for a seed image, uploading the
// image unless the space already has it.
func seedAsset(ctx context.Context, client *mapi.Client, uploaded map[string]mapi.Asset, name, alt string) (map[string]any, error) {
	asset, ok := uploaded[name]
	if !ok {
		colors, known := seedImages[name]
		if !known {
			return nil, fmt.Errorf("unknown seed image %q", name)
		}
		file := "seed-" + name + ".jpg"
		existing, err := client.FindAsset(ctx, file)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			asset = *existing
		} else {
			data, err := renderSeedImage(colors)
			if err != nil {
				return nil, err
			}
			if asset, err = client.UploadAsset(ctx, file, data, seedImageWidth, seedImageHeight, alt); err != nil {
				return nil, err
			}
		}
		uploaded[name] = asset
	}
	return map[string]any{"fieldtype": "asset", "id": asset.ID, "filename": asset.Filename, "alt": alt, "focus": "", "title": "", "name": ""}, nil
}

// resolveSeedImages replaces {"seed_image": name, "alt": text} objects in
// content with asset field values.
func resolveSeedImages(ctx context.Context, client *mapi.Client, uploaded map[string]mapi.Asset, node any) (any, error) {
	switch n := node.(type) {
	case map[string]any:
		if name, ok := n["seed_image"].(string); ok {
			alt, _ := n["alt"].(string)
			return seedAsset(ctx, client, uploaded, name, alt)
		}
		for key, child := range n {
			resolved, err := resolveSeedImages(ctx, client, uploaded, child)
			if err != nil {
				return nil, err
			}
			n[key] = resolved
		}
	case []any:
		for i, child := range n {
			resolved, err := resolveSeedImages(ctx, client, uploaded, child)
			if err != nil {
				return nil, err
			}
			n[i] = resolved
		}
	}
	return node, nil
}
