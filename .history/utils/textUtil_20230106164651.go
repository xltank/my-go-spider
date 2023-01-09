package utils

import (
	"my-go-spider/model"
	"net/http"
	"strings"
)

func ParseText(text string) []model.Analyzed {
	resp, err := http.Post(`http://localhost:9200/hanlp-1/_analyze`, `application/json`, strings.NewReader(text))
	return []model.Analyzed{}
}
