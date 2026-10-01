package main

import (
	"bytes"
	"encoding/xml"
	"slices"
	"strings"
	"time"
)

const (
	feedEntries  = 20
	feedTitle    = "cartpop.tv"
	feedSubtitle = "Cartridge game reviews, without ads"
	feedAuthor   = "cartpop.tv"
)

type atomFeed struct {
	XMLName  xml.Name    `xml:"http://www.w3.org/2005/Atom feed"`
	Title    string      `xml:"title"`
	Subtitle string      `xml:"subtitle"`
	Links    []atomLink  `xml:"link"`
	ID       string      `xml:"id"`
	Updated  string      `xml:"updated"`
	Author   atomAuthor  `xml:"author"`
	Entries  []atomEntry `xml:"entry"`
}

type atomLink struct {
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr"`
	Href string `xml:"href,attr"`
}

type atomAuthor struct {
	Name string `xml:"name"`
}

type atomEntry struct {
	Title      string         `xml:"title"`
	Link       atomLink       `xml:"link"`
	ID         string         `xml:"id"`
	Published  string         `xml:"published"`
	Updated    string         `xml:"updated"`
	Categories []atomCategory `xml:"category"`
	Summary    string         `xml:"summary"`
	Content    atomContent    `xml:"content"`
}

type atomCategory struct {
	Term string `xml:"term,attr"`
}

type atomContent struct {
	Type string `xml:"type,attr"`
	HTML string `xml:",cdata"`
}

type entryView struct {
	Root string
	*Review
}

func (t *templates) feed(site *Site) ([]byte, error) {
	feed := atomFeed{
		Title:    feedTitle,
		Subtitle: feedSubtitle,
		Links: []atomLink{
			{Rel: "self", Type: "application/atom+xml", Href: siteURL + "feed.xml"},
			{Rel: "alternate", Type: "text/html", Href: siteURL},
		},
		ID:     siteURL,
		Author: atomAuthor{Name: feedAuthor},
	}
	var updated time.Time
	if len(site.Reviews) > 0 {
		updated = site.Reviews[0].Date
	}
	feed.Updated = atomDate(updated)

	for _, r := range site.Reviews[:min(feedEntries, len(site.Reviews))] {
		var content bytes.Buffer
		if err := t.pages["feed.html"].ExecuteTemplate(&content, "entry", entryView{siteURL, r}); err != nil {
			return nil, err
		}
		entry := atomEntry{
			Title:     r.Title,
			Link:      atomLink{Rel: "alternate", Type: "text/html", Href: canonical(r.File())},
			ID:        canonical(r.File()),
			Published: atomDate(r.Date),
			Updated:   atomDate(r.Date),
			Summary:   r.Description,
			Content:   atomContent{Type: "html", HTML: content.String()},
		}
		for _, label := range r.Labels {
			entry.Categories = append(entry.Categories, atomCategory{label})
		}
		feed.Entries = append(feed.Entries, entry)
	}
	return marshalXML(feed)
}

func atomDate(day time.Time) string { return day.UTC().Format(time.RFC3339) }

type sitemap struct {
	XMLName xml.Name     `xml:"http://www.sitemaps.org/schemas/sitemap/0.9 urlset"`
	URLs    []sitemapURL `xml:"url"`
}

type sitemapURL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

func sitemapOf(site *Site, files map[string][]byte) ([]byte, error) {
	dates := map[string]string{}
	for _, r := range site.Reviews {
		dates[r.File()] = r.ISODate()
	}
	var s sitemap
	for file := range files {
		if strings.HasSuffix(file, ".html") && file != "404.html" {
			s.URLs = append(s.URLs, sitemapURL{canonical(file), dates[file]})
		}
	}
	slices.SortFunc(s.URLs, func(a, b sitemapURL) int { return strings.Compare(a.Loc, b.Loc) })
	return marshalXML(s)
}

func marshalXML(v any) ([]byte, error) {
	out, err := xml.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(append([]byte(xml.Header), out...), '\n'), nil
}
