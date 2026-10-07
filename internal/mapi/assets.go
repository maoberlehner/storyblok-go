package mapi

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"time"
)

type Asset struct {
	ID       int64  `json:"id"`
	Filename string `json:"filename"`
	Alt      string `json:"alt"`
}

// FindAsset returns the asset whose file is named name, or nil.
func (c *Client) FindAsset(ctx context.Context, name string) (*Asset, error) {
	var envelope struct {
		Assets []Asset `json:"assets"`
	}
	query := url.Values{"search": {name}}
	if _, err := c.request(ctx, http.MethodGet, "/assets?"+query.Encode(), nil, &envelope); err != nil {
		return nil, err
	}
	for _, asset := range envelope.Assets {
		if path.Base(asset.Filename) == name {
			return &asset, nil
		}
	}
	return nil, nil
}

// UploadAsset uploads an image. Sending its pixel size makes Storyblok encode
// the dimensions in the asset URL, which pages use to reserve image space.
func (c *Client) UploadAsset(ctx context.Context, name string, data []byte, width, height int, alt string) (Asset, error) {
	var signed struct {
		ID        int64             `json:"id"`
		PostURL   string            `json:"post_url"`
		PrettyURL string            `json:"pretty_url"`
		Fields    map[string]string `json:"fields"`
	}
	body := map[string]any{"filename": name, "size": fmt.Sprintf("%dx%d", width, height), "alt": alt, "validate_upload": 1}
	if _, err := c.request(ctx, http.MethodPost, "/assets", body, &signed); err != nil {
		return Asset{}, err
	}
	if err := upload(ctx, signed.PostURL, signed.Fields, name, data); err != nil {
		return Asset{}, fmt.Errorf("uploading %s: %w", name, err)
	}
	var finished map[string]any
	if _, err := c.request(ctx, http.MethodGet, "/assets/"+strconv.FormatInt(signed.ID, 10)+"/finish_upload", nil, &finished); err != nil {
		return Asset{}, err
	}
	return Asset{ID: signed.ID, Filename: "https:" + signed.PrettyURL, Alt: alt}, nil
}

// upload posts the file to the signed storage URL. It carries no Management
// API token: the signed fields authorize it.
func upload(ctx context.Context, postURL string, fields map[string]string, name string, data []byte) error {
	var buf bytes.Buffer
	form := multipart.NewWriter(&buf)
	for key, value := range fields {
		if err := form.WriteField(key, value); err != nil {
			return err
		}
	}
	file, err := form.CreateFormFile("file", name)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		return err
	}
	if err := form.Close(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, postURL, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", form.FormDataContentType())
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("storage returned HTTP %d", res.StatusCode)
	}
	return nil
}
