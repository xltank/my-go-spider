package utils

import (
	"my-go-spider/model"
	"net/http"
)

func ParseText(text string) []model.Analyzed {
	var buff = 
	resp, err := http.Post(`http://localhost:9200/hanlp-1/_analyze`, `application/json`)
	return []model.Analyzed{}
}
