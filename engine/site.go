package main

import (
	"fmt"
	"strings"
)

const (
	siteURL            = "https://cartpoptv.github.io/"
	defaultShareImage  = "img/og-default.png"
	maxScreenshotWidth = 320
)

type Platform struct {
	ID    string
	Label string
}

var platforms = []Platform{
	{"gb", "Game Boy"},
	{"gbc", "Game Boy Color"},
	{"n64", "Nintendo 64"},
	{"hw", "Hardware"},
}

var pillLabels = []string{"Cartridge"}

func platformOf(label string) (Platform, bool) {
	for _, p := range platforms {
		if p.Label == label {
			return p, true
		}
	}
	return Platform{}, false
}

type Rating int

var materials = [...]string{1: "stone", 2: "copper", 3: "silver", 4: "gold", 5: "diamond"}

func (r Rating) Material() string   { return materials[r] }
func (r Rating) String() string     { return fmt.Sprintf("%d out of 5 (%s)", int(r), r.Material()) }
func (r Rating) Tokens() []struct{} { return make([]struct{}, r) }

func ratingOf(material string) (Rating, bool) {
	for n, m := range materials {
		if n > 0 && strings.EqualFold(m, material) {
			return Rating(n), true
		}
	}
	return 0, false
}

func canonical(file string) string {
	if file == "index.html" {
		return siteURL
	}
	return siteURL + file
}

func labelSlug(label string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(label) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(r)
			dash = false
		} else {
			dash = true
		}
	}
	return b.String()
}

func labelFile(label string) string { return "label-" + labelSlug(label) + ".html" }
