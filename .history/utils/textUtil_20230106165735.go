package utils

import (
	"html/template"
	"my-go-spider/model"
	"net/http"
	"strings"
)

func ParseText(text string) []model.Analyzed {
	t := template.New("t")
	t.Parse(`{"analyzer": "my_hanlp_analyzer", "text": {{.}}`)
	t.Execute()
	resp, err := http.Post(`http://localhost:9200/hanlp-1/_analyze`, `application/json`, strings.NewReader(text))
	return []model.Analyzed{}
}
