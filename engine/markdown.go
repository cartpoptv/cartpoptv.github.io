package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"net/url"
	"path"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

func newMarkdown(scaleRating func(w io.Writer, root string, r Rating) error) goldmark.Markdown {
	return goldmark.New(goldmark.WithRendererOptions(
		renderer.WithNodeRenderers(util.Prioritized(&nodeRenderer{scaleRating}, 100)),
	))
}

type document struct {
	loader     *loader
	file       string
	firstLine  int
	source     []byte
	root       ast.Node
	prefix     string
	firstImage string
	pageLinks  []pageLink
}

type pageLink struct {
	node    *ast.Link
	address string
}

func (l *loader) document(file string, source []byte, firstLine int) *document {
	return &document{loader: l, file: file, firstLine: firstLine, source: source, root: l.md.Parser().Parse(text.NewReader(source))}
}

func (d *document) prepare() {
	d.links()
	d.figures()
	d.legend()
}

func (d *document) fail(n ast.Node, format string, args ...any) {
	line := d.firstLine + bytes.Count(d.source[:max(n.Pos(), 0)], []byte("\n"))
	d.loader.fail(d.file, line, format, args...)
}

func (d *document) html(n ast.Node) template.HTML {
	var buf bytes.Buffer
	d.render(&buf, n)
	return template.HTML(buf.String())
}

func (d *document) inline(n ast.Node) template.HTML {
	var buf bytes.Buffer
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		d.render(&buf, child)
	}
	return template.HTML(buf.String())
}

func (d *document) render(buf *bytes.Buffer, n ast.Node) {
	if err := d.loader.md.Renderer().Render(buf, d.source, n); err != nil && d.loader.err == nil {
		d.loader.err = err
	}
}

func (d *document) title() string {
	heading, ok := d.root.FirstChild().(*ast.Heading)
	if !ok || heading.Level != 1 {
		d.loader.fail(d.file, d.firstLine, "the text must start with a # title")
		return ""
	}
	d.root.RemoveChild(d.root, heading)
	return plainText(heading, d.source)
}

func (d *document) lead() ast.Node {
	paragraph, ok := d.root.FirstChild().(*ast.Paragraph)
	if !ok {
		return nil
	}
	d.root.RemoveChild(d.root, paragraph)
	return paragraph
}

func (d *document) renderText(t *Text, lead, hero ast.Node) {
	t.Body = d.html(d.root)
	if lead != nil {
		t.Lead = d.inline(lead)
	}
	if hero != nil {
		t.Hero = d.html(hero)
	}
}

func (d *document) firstParagraph() string {
	for block := d.root.FirstChild(); block != nil; block = block.NextSibling() {
		if paragraph, ok := block.(*ast.Paragraph); ok {
			return plainText(paragraph, d.source)
		}
	}
	return ""
}

func plainText(n ast.Node, source []byte) string {
	var b bytes.Buffer
	_ = ast.Walk(n, func(child ast.Node, entering bool) (ast.WalkStatus, error) {
		if t, ok := child.(*ast.Text); ok && entering {
			b.Write(t.Value(source))
			if t.SoftLineBreak() || t.HardLineBreak() {
				b.WriteByte(' ')
			}
		}
		return ast.WalkContinue, nil
	})
	return resolve(b.Bytes())
}

func resolve(raw []byte) string {
	return string(util.ResolveEntityNames(util.ResolveNumericReferences(util.UnescapePunctuations(raw))))
}

func (d *document) links() {
	_ = ast.Walk(d.root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if link, ok := n.(*ast.Link); ok && entering {
			if address := pageAddress(string(link.Destination)); address != string(link.Destination) {
				d.pageLinks = append(d.pageLinks, pageLink{link, address})
				link.Destination = []byte(d.prefix + address)
			}
		}
		return ast.WalkContinue, nil
	})
}

func (d *document) forFeed() {
	for _, link := range d.pageLinks {
		link.node.Destination = []byte(siteURL + link.address)
	}
	_ = ast.Walk(d.root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if fig, ok := n.(*figure); ok && entering {
			fig.inFeed = true
		}
		return ast.WalkContinue, nil
	})
}

func pageAddress(destination string) string {
	u, err := url.Parse(destination)
	if err != nil || u.Scheme != "" || u.Host != "" || !strings.HasSuffix(u.Path, ".md") {
		return destination
	}
	address := strings.TrimSuffix(path.Base(u.Path), ".md") + ".html"
	if u.Fragment != "" {
		address += "#" + u.EscapedFragment()
	}
	return address
}

type figure struct {
	ast.BaseBlock
	class          string
	hero           bool
	inFeed         bool
	images         []figureImage
	labelA, labelB string
}

type figureImage struct {
	src, alt      string
	credit        string
	width, height int
}

var kindFigure = ast.NewNodeKind("Figure")

func (f *figure) Kind() ast.NodeKind { return kindFigure }

func (f *figure) Dump(source []byte, level int) {
	ast.DumpHelper(f, source, level, map[string]string{"Class": f.class}, nil)
}

func (f *figure) credit() string { return f.images[0].credit }

func (f *figure) captioned() bool {
	return f.class != "compare" && (f.HasChildren() || f.credit() != "")
}

func (d *document) figures() {
	for block := d.root.FirstChild(); block != nil; {
		next := block.NextSibling()
		if p, ok := block.(*ast.Paragraph); ok && p.FirstChild() != nil && p.FirstChild().Kind() == ast.KindImage {
			d.root.ReplaceChild(d.root, p, d.figure(p))
		}
		block = next
	}
}

func (d *document) figure(paragraph *ast.Paragraph) *figure {
	fig := &figure{}
	child := paragraph.FirstChild()
	for ; child != nil; child = child.NextSibling() {
		if img, ok := child.(*ast.Image); ok {
			fig.images = append(fig.images, d.image(img))
		} else if t, ok := child.(*ast.Text); !ok || len(bytes.TrimSpace(t.Value(d.source))) > 0 {
			break
		}
	}
	var caption []ast.Node
	for ; child != nil; child = child.NextSibling() {
		caption = append(caption, child)
	}

	shots, photos := 0, 0
	for _, img := range fig.images {
		if img.width <= maxScreenshotWidth {
			shots++
		} else {
			photos++
		}
	}
	switch {
	case shots == 0:
		fig.class = "photo"
	case photos == 0 && isComparison(caption, d.source) && sameSize(fig.images):
		fig.class = "compare"
		fig.labelA, fig.labelB = plainText(caption[0], d.source), plainText(caption[2], d.source)
		caption = nil
	default:
		fig.class = "shots"
	}

	for _, node := range caption {
		paragraph.RemoveChild(paragraph, node)
		fig.AppendChild(fig, node)
	}
	return fig
}

func (d *document) image(img *ast.Image) figureImage {
	image := figureImage{src: string(img.Destination), alt: plainText(img, d.source), credit: resolve(img.Title)}
	entry, err := d.loader.manifests.find(image.src)
	switch {
	case err != nil:
		d.fail(img, "%v", err)
	case entry.Path == "":
		d.fail(img, "image not in the manifest: %s", image.src)
	}
	image.width, image.height = entry.Width, entry.Height
	if d.firstImage == "" {
		d.firstImage = image.src
	}
	return image
}

func isComparison(caption []ast.Node, source []byte) bool {
	if len(caption) != 3 || caption[0].Kind() != ast.KindEmphasis || caption[2].Kind() != ast.KindEmphasis {
		return false
	}
	vs, ok := caption[1].(*ast.Text)
	return ok && strings.TrimSpace(string(vs.Value(source))) == "vs"
}

func sameSize(images []figureImage) bool {
	return len(images) == 2 && images[0].width == images[1].width && images[0].height == images[1].height
}

func (d *document) hero() ast.Node {
	fig, ok := d.root.FirstChild().(*figure)
	if !ok || fig.class != "photo" {
		return nil
	}
	d.root.RemoveChild(d.root, fig)
	fig.hero = true
	return fig
}

func (d *document) zoomable() bool {
	for block := d.root.FirstChild(); block != nil; block = block.NextSibling() {
		if fig, ok := block.(*figure); ok && fig.class != "compare" {
			return true
		}
	}
	return false
}

type scale struct{ ast.BaseBlock }

type scaleRow struct {
	ast.BaseBlock
	rating Rating
	prefix string
}

var kindScale, kindScaleRow = ast.NewNodeKind("Scale"), ast.NewNodeKind("ScaleRow")

func (*scale) Kind() ast.NodeKind                 { return kindScale }
func (s *scale) Dump(source []byte, level int)    { ast.DumpHelper(s, source, level, nil, nil) }
func (*scaleRow) Kind() ast.NodeKind              { return kindScaleRow }
func (r *scaleRow) Dump(source []byte, level int) { ast.DumpHelper(r, source, level, nil, nil) }

func (d *document) legend() {
	for block := d.root.FirstChild(); block != nil; {
		next := block.NextSibling()
		if list, ok := block.(*ast.List); ok && !list.IsOrdered() {
			if rows := d.legendRows(list); rows != nil {
				legend := &scale{}
				for _, row := range rows {
					legend.AppendChild(legend, row)
				}
				d.root.ReplaceChild(d.root, list, legend)
			}
		}
		block = next
	}
}

func (d *document) legendRows(list *ast.List) []*scaleRow {
	var rows []*scaleRow
	var contents []ast.Node
	seen := map[Rating]bool{}
	for item := list.FirstChild(); item != nil; item = item.NextSibling() {
		content := item.FirstChild()
		if content == nil || content.NextSibling() != nil {
			return nil
		}
		name, ok := content.FirstChild().(*ast.Emphasis)
		if !ok || name.Level != 2 {
			return nil
		}
		rating, ok := ratingOf(plainText(name, d.source))
		if !ok || seen[rating] {
			return nil
		}
		seen[rating] = true
		rows = append(rows, &scaleRow{rating: rating, prefix: d.prefix})
		contents = append(contents, content)
	}
	if len(rows) != 5 {
		return nil
	}
	for i, row := range rows {
		for child := contents[i].FirstChild(); child != nil; {
			next := child.NextSibling()
			contents[i].RemoveChild(contents[i], child)
			row.AppendChild(row, child)
			child = next
		}
	}
	return rows
}

type nodeRenderer struct {
	scaleRating func(w io.Writer, root string, r Rating) error
}

func (r *nodeRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(kindFigure, r.renderFigure)
	reg.Register(kindScale, r.renderScale)
	reg.Register(kindScaleRow, r.renderScaleRow)
}

func (r *nodeRenderer) renderFigure(w util.BufWriter, _ []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	fig := n.(*figure)
	if !entering {
		if fig.captioned() {
			if credit := fig.credit(); credit != "" {
				if fig.HasChildren() {
					_ = w.WriteByte(' ')
				}
				fmt.Fprintf(w, `<span class="photo-credit">%s</span>`, escape(credit))
			}
			_, _ = w.WriteString("</figcaption>\n")
		}
		_, _ = w.WriteString("</figure>\n")
		return ast.WalkContinue, nil
	}

	class := fig.class
	if fig.hero {
		class += " photo-hero"
	}
	fmt.Fprintf(w, "<figure class=\"%s\">\n", class)
	if fig.class == "compare" && fig.inFeed {
		writeImage(w, "", fig.images[0])
		writeImage(w, "", fig.images[1])
		fmt.Fprintf(w, "<figcaption>%s vs %s</figcaption>\n", escape(fig.labelA), escape(fig.labelB))
		return ast.WalkContinue, nil
	}
	if fig.class == "compare" {
		_, _ = w.WriteString("<div class=\"compare-frame\">\n")
		writeImage(w, "compare-b", fig.images[1])
		writeImage(w, "compare-a", fig.images[0])
		_, _ = w.WriteString(`<input class="compare-range" type="range" min="0" max="100" value="50" aria-label="Drag to compare the two images">` + "\n</div>\n")
		fmt.Fprintf(w, `<figcaption><span class="compare-label-a">%s</span> <span class="compare-vs">vs</span> <span class="compare-label-b">%s</span></figcaption>`+"\n",
			escape(fig.labelA), escape(fig.labelB))
		return ast.WalkContinue, nil
	}
	for _, image := range fig.images {
		writeImage(w, "", image)
	}
	if fig.captioned() {
		_, _ = w.WriteString("<figcaption>")
	}
	return ast.WalkContinue, nil
}

func writeImage(w util.BufWriter, class string, image figureImage) {
	if class != "" {
		class = ` class="` + class + `"`
	}
	src := util.EscapeHTML(util.URLEscape([]byte(image.src), true))
	fmt.Fprintf(w, `<img%s src="%s" width="%d" height="%d" alt="%s">`+"\n", class, src, image.width, image.height, escape(image.alt))
}

func escape(s string) []byte { return util.EscapeHTML([]byte(s)) }

func (r *nodeRenderer) renderScale(w util.BufWriter, _ []byte, _ ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		_, _ = w.WriteString("<dl class=\"scale\">\n")
	} else {
		_, _ = w.WriteString("</dl>\n")
	}
	return ast.WalkContinue, nil
}

func (r *nodeRenderer) renderScaleRow(w util.BufWriter, _ []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		_, _ = w.WriteString("</dd>\n</div>\n")
		return ast.WalkContinue, nil
	}
	_, _ = w.WriteString("<div class=\"scale-row\">\n<dt>")
	row := n.(*scaleRow)
	if err := r.scaleRating(w, row.prefix, row.rating); err != nil {
		return ast.WalkStop, err
	}
	_, _ = w.WriteString("</dt>\n<dd>")
	return ast.WalkContinue, nil
}
