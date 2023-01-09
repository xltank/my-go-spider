package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"my-go-spider/db"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var siteEntry = "https://www.sina.com.cn"

func main() {

	db.Connect("mongodb://localhost:27017")

	c := colly.NewCollector(
		//colly.Debugger(&debug.LogDebugger{}),
		// colly.AllowedDomains("www.sina.com.cn", "www.sina.cn"),
		colly.URLFilters(regexp.MustCompile(`sina.com.cn|sina.cn`)),
		colly.DisallowedDomains("db.auto.sina.com.cn"),
		colly.CacheDir("./cache"),
		colly.MaxDepth(3),
	)

	detailC := c.Clone()

	// On every a element which has href attribute call callback
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Attr("href")
		// Print link
		fmt.Printf("Link found: %q -> %s\n", e.Text, link)
		// Visit link found on page
		// Only those links are visited which are in AllowedDomains
		//c.Visit(e.Request.AbsoluteURL(link))
		if link == "" {
			return
		}
		err := detailC.Visit(link)
		if err != nil {
			fmt.Println("visit detail page error", err.Error())
		}
	})

	detailC.OnHTML(".main-content", func(e *colly.HTMLElement) {
		title := e.DOM.Find(".main-title")
		dateStr := e.DOM.Find(".date")
		publishedAt, _ := time.Parse("2006年01月02日 15:04", dateStr.Text())
		source := e.DOM.Find(".source")
		body := e.DOM.Find(".article")
		channelPath := e.DOM.Find(".channel-path")
		channelPathValue := "/" + regexp.MustCompile(`\s|正文`).ReplaceAllString(strings.ReplaceAll(channelPath.Text(), ">", "/"), "")
		channelPathValue = regexp.MustCompile(`/$`).ReplaceAllString(channelPathValue, "")

		now := time.Now()
		article := db.ArticleCreate{
			Rel:          "",
			Title:        title.Text(),
			URI:          e.Request.URL.String(),
			Author:       "",
			Media:        source.Text(),
			Platform:     "新浪",
			Domain:       "sina.com.cn",
			Content:      body.Text(),
			ChannelPaths: channelPathValue,
			PublishedAt:  publishedAt,
			ScrapedAt:    now,
			ImageLinks:   []string{},
			VideoLinks:   []string{},
		}

		/*chief := db.Chief{
			Article: db.Article{
				Rel:          "",
				Title:        title.Text(),
				URI:          e.Request.URL.String(),
				Author:       "",
				Media:        source.Text(),
				Platform:     "新浪",
				Domain:       "sina.com.cn",
				Content:      body.Text(),
				ChannelPaths: channelPathValue,
				PublishedAt:  publishedAt,
				ScrapedAt:    now,
				ImageLinks:   []string{},
				VideoLinks:   []string{},
			},
			TopicCount: 1,
		}*/

		imgs := e.DOM.Find(".img_wrapper").ChildrenFiltered("img")
		imgs.Each(func(i int, selection *goquery.Selection) {
			src, _ := selection.Attr("src")
			if strings.Index(src, "//") == 0 {
				src = "http:" + src
			}
			article.ImageLinks = append(article.ImageLinks, src)
		})

		UpsertOneByURI("article", article.URI, article)

		s, err := json.MarshalIndent(article, "", "")
		if err != nil {
			log.Fatal("Error, marshal article" + err.Error())
		}
		fmt.Println("--->", string(s))
	})

	detailC.OnRequest(func(r *colly.Request) {
		fmt.Println("detail request", r.URL.String())
		//r.Ctx.Put("url", r.URL.String())
	})
	detailC.OnResponse(func(r *colly.Response) {
		fmt.Println("detail response", r.Request.URL.String(), r.StatusCode)
		//r.Ctx.Put("url", r.URL.String())
	})
	detailC.OnError(func(r *colly.Response, err error) {
		fmt.Println("detail error:", r.Request.URL, "\nError:", err)
	})

	// Before making a request print "Visiting ..."
	c.OnRequest(func(r *colly.Request) {
		fmt.Println("Link Visiting", r.URL.String())
		//r.Ctx.Put("url", r.URL.String())
	})

	//c.OnResponse(func(r *colly.Response) {
	//	fmt.Println("url:" + r.Ctx.Get("url"))
	//})

	// Start scraping on https://hackerspaces.org
	err := c.Visit(siteEntry)
	if err != nil {
		fmt.Println("visit entry url", err.Error())
	}
}

func UpsertOneByURI(col string, uri string, data interface{}) {
	filter := bson.D{{Key: "uri", Value: uri}}
	now := time.Now()
	var d interface{}
	switch item := data.(type) {
	case db.ArticleCreate:
		item.UpdatedAt = now
		d = item
	case db.Article:
		item.UpdatedAt = now
		d = item
	case db.Chief:
		item.UpdatedAt = now
		d = item
	default:
		fmt.Println("Invalid type")
		return
	}
	r, err := db.DB.Collection(col).UpdateOne(
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

	fmt.Println(r)
}

func analyze(text string) [] {}