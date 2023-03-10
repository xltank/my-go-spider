package model

import "time"

type ArticleCreate struct {
	Rel           string    `json:"rel,omitempty" bson:"rel"`
	SimHash       string    `json:"simHash,omitempty" bson:"simHash"`
	Title         string    `json:"title,omitempty" bson:"title"`
	TitleTokenStr string    `json:"titleTokenStr,omitempty" bson:"titleTokenStr"`
	TitleTokens   []Token   `json:"title_tokens,omitempty" bson:"titleTokens"`
	URI           string    `json:"uri,omitempty" bson:"uri"`
	Author        string    `json:"author,omitempty" bson:"author"`             // 某某
	Media         string    `json:"media,omitempty" bson:"media"`               // 中国日报
	Domain        string    `json:"domain,omitempty" bson:"domain"`             // sina.com.cn
	Platform      string    `json:"platform,omitempty" bson:"platform"`         // 新浪网
	Content       string    `json:"content,omitempty" bson:"content"`           // 正文内容
	ChannelPaths  string    `json:"channelPaths,omitempty" bson:"channelPaths"` // 新浪科技> 互联网 > 正文 -> 新浪科技/互联网
	UpdatedAt     time.Time `json:"updatedAt,omitempty" bson:"updatedAt"`       // datetime
	PublishedAt   time.Time `json:"publishedAt,omitempty" bson:"publishedAt"`   // 2023年01月03日 20:57 -> datetime
	ScrapedAt     time.Time `json:"scrapedAt,omitempty" bson:"scrapedAt"`
	ImageLinks    []string  `json:"imageLinks,omitempty" bson:"imageLinks"`
	VideoLinks    []string  `json:"videoLinks,omitempty" bson:"videoLinks"`
}

/*
   CreatedAt 在 $set 和 $setOnInsert 同时存在时会报错
*/
type Article struct {
	ArticleCreate
	CreatedAt time.Time `json:"createdAt,omitempty" bson:"createdAt"` // datetime
}

type Chief struct {
	Article
	CreatedAt  time.Time `json:"createdAt,omitempty" bson:"createdAt"` // datetime
	TopicCount int32     `json:"topicCount" bson:"topicCount"`
}

type Token struct {
	Token       string `json:"token,omitempty" bson:"token"`
	StartOffset int32  `json:"start_offset,omitempty" bson:"startOffset"`
	EndOffset   int32  `json:"end_offset,omitempty" bson:"endOffset"`
	Type        string `json:"type,omitempty" bson:"type"`
	Position    int32  `json:"position,omitempty" bson:"position"`
}

type AnalyzedResult struct {
	Tokens []Token `json:"tokens,omitempty"`
}
