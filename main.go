package main

import (
	"context"
	"fmt"
	"log"
	"my-go-spider/db"
	"my-go-spider/model"
	"my-go-spider/utils"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// var siteEntry = "https://www.sina.com.cn"
var siteEntry = "https://finance.sina.com.cn/"

func main() {

	db.Connect("mongodb://localhost:27017")

	c := colly.NewCollector(
		//colly.Debugger(&debug.LogDebugger{}),
		// colly.AllowedDomains("www.sina.com.cn", "www.sina.cn"),
		colly.URLFilters(regexp.MustCompile(`sina.com.cn|sina.cn`)),
		colly.DisallowedDomains("db.auto.sina.com.cn"),
		// colly.CacheDir("./cache"),
		colly.MaxDepth(2),
	)

	detailC := c.Clone()

	// On every a element which has href attribute call callback
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Attr("href")
		fmt.Printf("Link found: %q %s\n", e.Text, link)
		if link == "" {
			return
		}
		err := detailC.Visit(link)
		if err != nil {
			fmt.Println("visit detail page error:", err.Error())
		}
	})

	detailC.OnHTML(".main-content", handleDetailDom)

	detailC.OnRequest(func(r *colly.Request) {
		// fmt.Println("detail request", r.URL.String())
	})
	detailC.OnResponse(func(r *colly.Response) {
		if r.StatusCode != 200 {
			fmt.Println("detail response", r.Request.URL.String(), r.StatusCode)
		}
	})
	detailC.OnError(func(r *colly.Response, err error) {
		fmt.Println("detail error:", r.Request.URL, "\nError:", err)
	})

	c.OnRequest(func(r *colly.Request) {
		fmt.Println("visiting", r.URL.String())
	})

	err := c.Visit(siteEntry)
	if err != nil {
		fmt.Println("visit entry url error:", err.Error())
	}
}

func handleDetailDom(e *colly.HTMLElement) {
	title := e.DOM.Find(".main-title")
	dateStr := e.DOM.Find(".date")
	publishedAt, _ := time.Parse("2006年01月02日 15:04", dateStr.Text())
	source := e.DOM.Find(".source")
	body := e.DOM.Find(".article")
	channelPath := e.DOM.Find(".channel-path")
	channelPathValue := "/" + regexp.MustCompile(`\s|正文`).ReplaceAllString(strings.ReplaceAll(channelPath.Text(), ">", "/"), "")
	channelPathValue = regexp.MustCompile(`/$`).ReplaceAllString(channelPathValue, "")

	now := time.Now()
	article := model.ArticleCreate{
		Rel:           "",
		SimHash:       "",
		Title:         title.Text(),
		TitleTokenStr: "",
		URI:           e.Request.URL.String(),
		Author:        "",
		Media:         source.Text(),
		Platform:      "新浪",
		Domain:        "sina.com.cn",
		Content:       body.Text(),
		ChannelPaths:  channelPathValue,
		PublishedAt:   publishedAt,
		ScrapedAt:     now,
		ImageLinks:    []string{},
		VideoLinks:    []string{},
	}
	article.TitleTokens = utils.ParseText(article.Title)
	for i, v := range article.TitleTokens {
		article.TitleTokenStr += v.Token
		if i < len(article.TitleTokens)-1 {
			article.TitleTokenStr += " "
		}
	}

	if article.TitleTokenStr != "" {
		article.SimHash = utils.GetSimHash(strings.ReplaceAll(article.TitleTokenStr, " ", ""))
		article.Rel = utils.GetRelBySimHash(article.SimHash, article.PublishedAt)
	}

	imgs := e.DOM.Find(".img_wrapper").ChildrenFiltered("img")
	imgs.Each(func(i int, selection *goquery.Selection) {
		src, _ := selection.Attr("src")
		if strings.Index(src, "//") == 0 {
			src = "http:" + src
		}
		article.ImageLinks = append(article.ImageLinks, src)
	})

	UpsertOneByURI("article", article.URI, article)
}

func UpsertOneByURI(col string, uri string, data interface{}) {
	filter := bson.D{{Key: "uri", Value: uri}}
	now := time.Now()
	var d interface{}
	switch item := data.(type) {
	case model.ArticleCreate:
		item.UpdatedAt = now
		d = item
	case model.Article:
		item.UpdatedAt = now
		d = item
	case model.Chief:
		item.UpdatedAt = now
		d = item
	default:
		fmt.Println("Invalid type")
		return
	}
	_, err := db.DB.Collection(col).UpdateOne(
		context.TODO(),
		filter,
		bson.M{
			"$set":         d,
			"$setOnInsert": bson.D{{Key: "createdAt", Value: time.Now()}},
		},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		log.Fatal(err)
	}
}
