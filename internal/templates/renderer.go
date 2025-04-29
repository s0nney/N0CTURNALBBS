package templates

import (
	"N0CTURNALBBS/internal/handlers"
	"html/template"
	"time"

	"github.com/gin-gonic/gin/render"
)

func CreateRenderer(pattern string) render.HTMLRender {
	r := render.HTMLProduction{
		Template: template.Must(template.New("").Funcs(GetTemplateFuncs()).ParseGlob(pattern)),
	}
	return r
}

func GetTemplateFuncs() template.FuncMap {
	return template.FuncMap{
		"formatTime": func(t time.Time) string {
			return t.Format("Jan 02, 2006 15:04:05")
		},
		"formatBody": func(body string, threadID ...interface{}) template.HTML {
			return template.HTML(body)
		},
		"formatPreview": func(body string, maxLength int) string {
			return handlers.FormatPreview(body, maxLength)
		},
		"currentYear": func() int {
			return time.Now().Year()
		},
	}
}
