package handler

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
)

type Templates struct {
	pages    map[string]*template.Template
	partials *template.Template
	funcs    template.FuncMap
}

func NewTemplates(templatesFS embed.FS) (*Templates, error) {
	funcs := template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"mul": func(a, b int) int { return a * b },
		"min": func(a, b int) int {
			if a < b {
				return a
			}
			return b
		},
		"divFloat": func(a, b int) float64 {
			if b == 0 {
				return 0
			}
			return float64(a) / float64(b)
		},
		"deref": func(p *int) int {
			if p == nil {
				return 0
			}
			return *p
		},
	}

	subFS, err := fs.Sub(templatesFS, "templates")
	if err != nil {
		return nil, fmt.Errorf("getting templates sub fs: %w", err)
	}

	t := &Templates{
		pages: make(map[string]*template.Template),
		funcs: funcs,
	}

	baseContent, err := fs.ReadFile(subFS, "layouts/base.html")
	if err != nil {
		return nil, fmt.Errorf("reading base layout: %w", err)
	}

	var partialContents []string
	err = fs.WalkDir(subFS, "partials", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".html") {
			return err
		}
		content, err := fs.ReadFile(subFS, path)
		if err != nil {
			return err
		}
		partialContents = append(partialContents, string(content))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("reading partials: %w", err)
	}

	partialsStr := strings.Join(partialContents, "\n")
	t.partials, err = template.New("partials").Funcs(funcs).Parse(partialsStr)
	if err != nil {
		return nil, fmt.Errorf("parsing partials: %w", err)
	}

	err = fs.WalkDir(subFS, "pages", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".html") {
			return err
		}

		pageContent, err := fs.ReadFile(subFS, path)
		if err != nil {
			return fmt.Errorf("reading page %s: %w", path, err)
		}

		combined := string(baseContent) + "\n" + partialsStr + "\n" + string(pageContent)

		tmpl, err := template.New("base").Funcs(funcs).Parse(combined)
		if err != nil {
			return fmt.Errorf("parsing page %s: %w", path, err)
		}

		name := filepath.Base(path)
		t.pages[name] = tmpl
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("loading pages: %w", err)
	}

	return t, nil
}

func (t *Templates) Render(w http.ResponseWriter, name string, data any) error {
	pageName := filepath.Base(name)

	tmpl, ok := t.pages[pageName]
	if !ok {
		return fmt.Errorf("template %s not found", name)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return tmpl.ExecuteTemplate(w, "base", data)
}

func (t *Templates) RenderPartial(w http.ResponseWriter, name string, data any) error {
	partialName := strings.TrimSuffix(filepath.Base(name), ".html")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return t.partials.ExecuteTemplate(w, partialName, data)
}
