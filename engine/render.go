package main

import (
	"bytes"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"slices"
)

type templates struct {
	pages    map[string]*template.Template
	partials *template.Template
}

type score struct {
	Root string
	Rating
}

func loadTemplates() (*templates, error) {
	parse := func(set *template.Template, name string) error {
		text, err := readText(filepath.Join("templates", name))
		if err == nil {
			_, err = set.New(name).Parse(string(text))
		}
		return err
	}
	shared := template.New("shared").Funcs(template.FuncMap{
		"score":     func(root string, r Rating) score { return score{root, r} },
		"labelFile": labelFile,
	})
	for _, name := range []string{"base.html", "partials.html"} {
		if err := parse(shared, name); err != nil {
			return nil, err
		}
	}
	t := &templates{pages: map[string]*template.Template{}, partials: shared}
	for _, name := range []string{"review.html", "list.html", "page.html", "feed.html"} {
		set, err := shared.Clone()
		if err == nil {
			err = parse(set, name)
		}
		if err != nil {
			return nil, err
		}
		t.pages[name] = set
	}
	return t, nil
}

func (t *templates) scaleRating(w io.Writer, root string, r Rating) error {
	return t.partials.ExecuteTemplate(w, "scale-rating", score{root, r})
}

type shell struct {
	Root    string
	Current string
	Head    head
}

type head struct {
	Description string
	URL         string
	Type        string
	Image       shareImage
	Published   string
}

func (h head) Card() string {
	if h.Image.Own {
		return "summary_large_image"
	}
	return "summary"
}

type reviewView struct {
	shell
	*Review
	Previous, Next *Review
}

type listView struct {
	shell
	Title   string
	Intro   *Page
	Filters []Filter
	Rows    []Row
}

type Row struct {
	*Review
	Hidden bool
}

type pageView struct {
	shell
	*Page
}

type Filter struct {
	Name     string
	File     string
	Title    string
	Platform *Platform
	Count    int
	Active   bool
}

func filters(reviews []*Review, active string) []Filter {
	counts := map[string]int{}
	for _, r := range reviews {
		for _, label := range r.Labels {
			counts[label]++
		}
	}
	list := []Filter{{Name: "All", File: "index.html", Title: listTitle(""), Count: len(reviews), Active: active == ""}}
	add := func(label string, p *Platform) {
		if n := counts[label]; n > 0 {
			list = append(list, Filter{Name: label, File: labelFile(label), Title: listTitle(label), Platform: p, Count: n, Active: label == active})
		}
	}
	for i := range platforms {
		add(platforms[i].Label, &platforms[i])
	}
	for _, label := range pillLabels {
		add(label, nil)
	}
	return list
}

func listTitle(label string) string {
	if label == "" {
		return "cartpop.tv - cartridge game reviews"
	}
	return label + " reviews - cartpop.tv"
}

func list(site *Site, label string) listView {
	file := "index.html"
	if label != "" {
		file = labelFile(label)
	}
	h := head{Description: site.Intro.Description, URL: canonical(file), Type: "website", Image: site.defaultImage}
	view := listView{Current: file, Head: h, Title: listTitle(label), Intro: site.Intro, Filters: filters(site.Reviews, label)}
	for _, r := range site.Reviews {
		view.Rows = append(view.Rows, Row{r, label != "" && !slices.Contains(r.Labels, label)})
	}
	return view
}

func siteLabels(reviews []*Review) []string {
	var labels []string
	for _, r := range reviews {
		labels = append(labels, r.Labels...)
	}
	slices.Sort(labels)
	return slices.Compact(labels)
}

func (t *templates) render(site *Site) (map[string][]byte, error) {
	files := map[string][]byte{}
	add := func(name, file string, view any) error {
		var buf bytes.Buffer
		if err := t.pages[name].ExecuteTemplate(&buf, "base.html", view); err != nil {
			return err
		}
		files[file] = buf.Bytes()
		return nil
	}

	for _, label := range append([]string{""}, siteLabels(site.Reviews)...) {
		view := list(site, label)
		if err := add("list.html", view.Current, view); err != nil {
			return nil, err
		}
	}
	for i, r := range site.Reviews {
		image := r.OGImage
		if !image.Own {
			image = site.defaultImage
		}
		h := head{Description: r.Description, URL: canonical(r.File()), Type: "article", Image: image, Published: r.ISODate()}
		view := reviewView{Current: r.File(), Head: h, Review: r}
		if i+1 < len(site.Reviews) {
			view.Previous = site.Reviews[i+1]
		}
		if i > 0 {
			view.Next = site.Reviews[i-1]
		}
		if err := add("review.html", r.File(), view); err != nil {
			return nil, err
		}
	}
	for _, p := range site.Pages {
		h := head{Description: p.Description, Type: "website", Image: site.defaultImage}
		if p.Slug != "404" {
			h.URL = canonical(p.File())
		}
		if err := add("page.html", p.File(), pageView{shell{Root: p.prefix, Current: p.File(), Head: h}, p}); err != nil {
			return nil, err
		}
	}

	feed, err := t.feed(site)
	if err != nil {
		return nil, err
	}
	sitemap, err := sitemapOf(site, files)
	if err != nil {
		return nil, err
	}
	files["feed.xml"], files["sitemap.xml"] = feed, sitemap
	return files, nil
}

func write(files map[string][]byte) error {
	entries, err := os.ReadDir("public")
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, e := range entries {
		if err := os.RemoveAll(filepath.Join("public", e.Name())); err != nil {
			return err
		}
	}
	if err := os.MkdirAll("public", 0o755); err != nil {
		return err
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join("public", name), data, 0o644); err != nil {
			return err
		}
	}
	return os.CopyFS("public", os.DirFS("static"))
}
