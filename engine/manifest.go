package main

import (
	"encoding/json/v2"
	"fmt"
	"net/http"
	"regexp"
	"time"
)

type ManifestImage struct {
	Path   string `json:"path"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

var imageAddress = regexp.MustCompile(`^` + regexp.QuoteMeta(siteURL) + `(images-\d{4})/(.+)$`)

var client = &http.Client{Timeout: 30 * time.Second}

type manifests map[string]map[string]ManifestImage

func (m manifests) find(address string) (ManifestImage, error) {
	match := imageAddress.FindStringSubmatch(address)
	if match == nil {
		return ManifestImage{}, nil
	}
	repo, path := match[1], match[2]
	images, ok := m[repo]
	if !ok {
		var err error
		images, err = fetchManifest(repo)
		m[repo] = images
		if err != nil {
			return ManifestImage{}, err
		}
	}
	return images[path], nil
}

func fetchManifest(repo string) (map[string]ManifestImage, error) {
	url := siteURL + repo + "/manifest.json"
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	var manifest struct {
		Images []ManifestImage `json:"images"`
	}
	if err := json.UnmarshalRead(resp.Body, &manifest); err != nil {
		return nil, fmt.Errorf("%s: %w", url, err)
	}
	images := make(map[string]ManifestImage, len(manifest.Images))
	for _, img := range manifest.Images {
		images[img.Path] = img
	}
	return images, nil
}
