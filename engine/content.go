package main

import (
	"bytes"
	"cmp"
	"fmt"
	"html/template"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
)

type Site struct {
	Reviews      []*Review
	Pages        []*Page
	Intro        *Page
	defaultImage shareImage
}

type Review struct {
	Slug        string
	Title       string
	Description string
	Date        time.Time
	Platform    Platform
	Labels      []string
	Rating      Rating
	Points      []Point
	Video       *Link
	OGImage     shareImage
	Text
	Feed     Text
	Zoomable bool
}

type Text struct {
	Lead       template.HTML
	Hero       template.HTML
	Body       template.HTML
	ReviewedOn template.HTML
}

type shareImage struct {
	URL           string
	Width, Height int
	Own           bool
}

type Point struct {
	Pro  bool
	Text string
}

type Link struct{ Text, URL string }

func (r *Review) File() string     { return r.Slug + ".html" }
func (r *Review) ISODate() string  { return r.Date.Format(time.DateOnly) }
func (r *Review) LongDate() string { return r.Date.Format("2 January 2006") }

type Page struct {
	Slug        string
	Title       string
	Description string
	Body        template.HTML
	Zoomable    bool
	prefix      string
}

func (p *Page) File() string { return p.Slug + ".html" }

type loader struct {
	md        goldmark.Markdown
	manifests manifests
	err       error
}

func (l *loader) fail(file string, line int, format string, args ...any) {
	if l.err == nil {
		l.err = fmt.Errorf("%s:%d: %s", file, line, fmt.Sprintf(format, args...))
	}
}

func readText(name string) ([]byte, error) {
	data, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	return bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n")), nil
}

func (l *loader) site() (*Site, error) {
	site := &Site{}
	err := l.each("reviews", func(file, slug string, text []byte) {
		site.Reviews = append(site.Reviews, l.review(file, slug, text))
	})
	if err != nil {
		return nil, err
	}
	err = l.each("pages", func(file, slug string, text []byte) {
		if page := l.page(file, slug, text); slug == "index" {
			site.Intro = page
		} else {
			site.Pages = append(site.Pages, page)
		}
	})
	if err != nil {
		return nil, err
	}
	slices.SortFunc(site.Reviews, func(a, b *Review) int {
		return cmp.Or(b.Date.Compare(a.Date), strings.Compare(a.Slug, b.Slug))
	})
	return site, l.err
}

func (l *loader) each(dir string, fn func(file, slug string, text []byte)) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		slug, ok := strings.CutSuffix(e.Name(), ".md")
		if !ok || e.IsDir() {
			continue
		}
		text, err := readText(filepath.Join(dir, e.Name()))
		if err != nil {
			return err
		}
		fn(dir+"/"+e.Name(), slug, text)
	}
	return nil
}

func (l *loader) page(file, slug string, text []byte) *Page {
	p := &Page{Slug: slug}
	if slug == "404" {
		p.prefix = "/"
	}
	doc := l.document(file, text, 1)
	doc.prefix = p.prefix
	doc.prepare()
	p.Zoomable = doc.zoomable()
	p.Title = doc.title()
	p.Description = doc.firstParagraph()
	p.Body = doc.html(doc.root)
	return p
}

func (l *loader) review(file, slug string, text []byte) *Review {
	r := &Review{Slug: slug}
	fields, body, bodyLine := l.splitHeader(file, text)
	l.header(r, file, fields)

	doc := l.document(file, body, bodyLine)
	doc.prepare()
	r.Zoomable = doc.zoomable()
	r.OGImage = l.ogImage(doc.firstImage)
	r.Title = doc.title()
	lead := doc.lead()
	hero := doc.hero()
	if lead != nil {
		r.Description = plainText(lead, doc.source)
	}
	doc.renderText(&r.Text, lead, hero)
	doc.forFeed()
	doc.renderText(&r.Feed, lead, hero)
	return r
}

type field struct {
	key, value string
	line       int
}

func (l *loader) splitHeader(file string, src []byte) (fields []field, body []byte, bodyLine int) {
	lines := strings.SplitAfter(string(src), "\n")
	if strings.TrimSpace(lines[0]) != "---" {
		l.fail(file, 1, `the first line must be "---"`)
		return nil, src, 1
	}
	offset := len(lines[0])
	for i, raw := range lines[1:] {
		n := i + 2
		offset += len(raw)
		line := strings.TrimSpace(raw)
		switch {
		case line == "---":
			return fields, src[offset:], n + 1
		case line == "":
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			l.fail(file, n, `expected "key: value"`)
			continue
		}
		fields = append(fields, field{strings.TrimSpace(key), strings.TrimSpace(value), n})
	}
	l.fail(file, 1, `the header has no closing "---"`)
	return fields, nil, len(lines)
}

func (l *loader) header(r *Review, file string, fields []field) {
	for _, f := range fields {
		switch f.key {
		case "date":
			date, err := time.Parse(time.DateOnly, f.value)
			if err != nil {
				l.fail(file, f.line, "bad date %q, expected YYYY-MM-DD", f.value)
			}
			r.Date = date
		case "labels":
			for label := range strings.SplitSeq(f.value, ",") {
				if label = strings.TrimSpace(label); label != "" {
					r.Labels = append(r.Labels, label)
				}
			}
			if len(r.Labels) > 0 {
				platform, ok := platformOf(r.Labels[0])
				if !ok {
					l.fail(file, f.line, "unknown platform %q", r.Labels[0])
				}
				r.Platform = platform
			}
		case "rating":
			n, err := strconv.Atoi(f.value)
			if err != nil || n < 1 || n > 5 {
				l.fail(file, f.line, "bad rating %q, expected 1 to 5", f.value)
				continue
			}
			r.Rating = Rating(n)
		case "reviewed-on":
			if doc := l.headerText(file, f); doc != nil {
				r.ReviewedOn = doc.inline(doc.root.FirstChild())
				doc.forFeed()
				r.Feed.ReviewedOn = doc.inline(doc.root.FirstChild())
			}
		case "pro", "con":
			r.Points = append(r.Points, Point{Pro: f.key == "pro", Text: f.value})
		case "video":
			r.Video = l.video(file, f)
		default:
			l.fail(file, f.line, "unknown key %q", f.key)
		}
	}
	if r.Date.IsZero() || r.Labels == nil || r.Rating == 0 {
		l.fail(file, 1, "date, labels and rating are required")
	}
}

func (l *loader) headerText(file string, f field) *document {
	doc := l.document(file, []byte(f.value), f.line)
	if _, ok := doc.root.FirstChild().(*ast.Paragraph); !ok {
		return nil
	}
	doc.links()
	return doc
}

func (l *loader) video(file string, f field) *Link {
	var link *ast.Link
	doc := l.headerText(file, f)
	if doc != nil {
		link, _ = doc.root.FirstChild().FirstChild().(*ast.Link)
	}
	if link == nil {
		l.fail(file, f.line, "video must be a link: [text](url)")
		return nil
	}
	return &Link{Text: plainText(link, doc.source), URL: string(link.Destination)}
}

func (l *loader) ogImage(firstImage string) shareImage {
	folder, _, ok := strings.CutLast(firstImage, "/")
	if !ok {
		return shareImage{}
	}
	for _, name := range []string{"og-image.jpg", "og-image.png"} {
		address := folder + "/" + name
		if img, _ := l.manifests.find(address); img.Path != "" {
			return shareImage{URL: address, Width: img.Width, Height: img.Height, Own: true}
		}
	}
	return shareImage{}
}

func defaultImage() (shareImage, error) {
	f, err := os.Open(filepath.Join("static", defaultShareImage))
	if err != nil {
		return shareImage{}, err
	}
	defer f.Close()
	config, _, err := image.DecodeConfig(f)
	if err != nil {
		return shareImage{}, err
	}
	return shareImage{URL: siteURL + defaultShareImage, Width: config.Width, Height: config.Height}, nil
}
