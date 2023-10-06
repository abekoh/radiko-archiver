package server

import (
	"encoding/xml"
	"time"
)

var JST = time.FixedZone("Asia/Tokyo", 9*60*60)

type RSS struct {
	XMLName xml.Name  `xml:"rss"`
	Version string    `xml:"version,attr"`
	Atom    string    `xml:"xmlns:atom,attr"`
	Itunes  string    `xml:"xmlns:itunes,attr"`
	Media   string    `xml:"xmlns:media,attr"`
	DC      string    `xml:"xmlns:dc,attr"`
	Channel []Channel `xml:"channel"`
}

type Channel struct {
	Title        string      `xml:"title"`
	Description  string      `xml:"description,omitempty"`
	Generator    string      `xml:"generator,omitempty"`
	Link         string      `xml:"link,omitempty"`
	NewFeedURL   string      `xml:"itunes:new-feed-url,omitempty"`
	ITunesAuthor string      `xml:"itunes:author,omitempty"`
	Explicit     string      `xml:"itunes:explicit,omitempty"`
	Keywords     string      `xml:"itunes:keywords,omitempty"`
	Subtitle     string      `xml:"itunes:subtitle,omitempty"`
	Summary      string      `xml:"itunes:summary,omitempty"`
	Owner        ITunesOwner `xml:"itunes:owner,omitempty"`
	Language     string      `xml:"language,omitempty"`
	Item         []Item      `xml:"item"`
}

type ITunesOwner struct {
	Name  string `xml:"itunes:name"`
	Email string `xml:"itunes:email,omitempty"`
}

type Item struct {
	Title       string    `xml:"title,omitempty"`
	Description string    `xml:"description,omitempty"`
	PubDate     string    `xml:"pubDate,omitempty"`
	Link        string    `xml:"link,omitempty"`
	GUID        GUID      `xml:"guid,omitempty"`
	Author      string    `xml:"itunes:author,omitempty"`
	Explicit    string    `xml:"itunes:explicit,omitempty"`
	Subtitle    string    `xml:"itunes:subtitle,omitempty"`
	Duration    string    `xml:"itunes:duration,omitempty"`
	Enclosure   Enclosure `xml:"enclosure,omitempty"`
}

type GUID struct {
	IsPermaLink bool   `xml:"isPermaLink,attr"`
	Content     string `xml:",chardata"`
}

type Contributor struct {
	Name string `xml:"http://www.w3.org/2005/Atom name"`
	URI  string `xml:"http://www.w3.org/2005/Atom uri"`
}

type Enclosure struct {
	URL    string `xml:"url,attr"`
	Type   string `xml:"type,attr"`
	Length int64  `xml:"length,attr"`
}
