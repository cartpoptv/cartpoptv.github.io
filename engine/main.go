package main

import (
	"fmt"
	"os"
)

func main() {
	n, err := build()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("reviews: %d, written to public/\n", n)
}

func build() (int, error) {
	t, err := loadTemplates()
	if err != nil {
		return 0, err
	}
	l := &loader{md: newMarkdown(t.scaleRating), manifests: manifests{}}
	site, err := l.site()
	if err != nil {
		return 0, err
	}
	if site.defaultImage, err = defaultImage(); err != nil {
		return 0, err
	}
	files, err := t.render(site)
	if err != nil {
		return 0, err
	}
	return len(site.Reviews), write(files)
}
