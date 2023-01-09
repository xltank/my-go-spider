package utils

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"my-go-spider/model"
	"net/http"
	"strings"
)

func ParseText(text string) []model.Token {
	t := fmt.Sprintf(`{"analyzer": "my_hanlp_analyzer", "text": "%s"}`, text)
	resp, err := http.Post(`http://localhost:9200/hanlp-1/_analyze`, `application/json`, strings.NewReader(t))
	if err != nil {
		fmt.Println(`ParseText Error: `, err.Error())
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(`ParseText, read resp.Body error: `, err.Error())
	}
	// bodyStr := string(body)
	r := model.AnalyzedResult{}
	err = json.Unmarshal(body, &r)
	if err != nil {
		fmt.Println(`Unmarshal analyze result Error: `, err.Error())
	}
	return r.Tokens
}
