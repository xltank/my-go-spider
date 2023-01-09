package utils

import (
	"fmt"
	"html/template"
	"my-go-spider/model"
	"net/http"
	"strings"
)

func ParseText(text string) []model.Analyzed {
	t := fmt.Sprintf(`{"analyzer": "my_hanlp_analyzer", "text": %s`, text)
	resp, err := http.Post(`http://localhost:9200/hanlp-1/_analyze`, `application/json`, strings.NewReader(t))
	if err != nil {
		fmt.Println(`ParseText Error: `, err.Error())
	}

	resp.
	return []model.Analyzed{}
}
