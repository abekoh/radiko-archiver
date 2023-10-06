package server

import (
	"encoding/xml"
	"time"
)

var JST = time.FixedZone("Asia/Tokyo", 9*60*60)

type RSS struct {
	XMLName xml.Name  `xml:"rss"`
	Channel []Channel `xml:"channel"`
}

type Channel struct {
	Title         string         `xml:"title"`
	Description   string         `xml:"description,omitempty"`
	Generator     string         `xml:"generator,omitempty"`
	Link          string         `xml:"link,omitempty"`
	AtomLink      AtomLink       `xml:"http://www.w3.org/2005/Atom link,omitempty"`
	NewFeedURL    string         `xml:"http://www.itunes.com/dtds/podcast-1.0.dtd new-feed-url,omitempty"`
	Thumbnail     MediaThumbnail `xml:"http://search.yahoo.com/mrss/ thumbnail,omitempty"`
	MediaKeywords string         `xml:"http://search.yahoo.com/mrss/ keywords,omitempty"`
	MediaCategory MediaCategory  `xml:"http://search.yahoo.com/mrss/ category,omitempty"`
	ITunesAuthor  string         `xml:"http://www.itunes.com/dtds/podcast-1.0.dtd author,omitempty"`
	Explicit      string         `xml:"http://www.itunes.com/dtds/podcast-1.0.dtd explicit,omitempty"`
	Image         ITunesImage    `xml:"http://www.itunes.com/dtds/podcast-1.0.dtd image,omitempty"`
	Keywords      string         `xml:"http://www.itunes.com/dtds/podcast-1.0.dtd keywords,omitempty"`
	Subtitle      string         `xml:"http://www.itunes.com/dtds/podcast-1.0.dtd subtitle,omitempty"`
	Summary       string         `xml:"http://www.itunes.com/dtds/podcast-1.0.dtd summary,omitempty"`
	Category      ITunesCategory `xml:"http://www.itunes.com/dtds/podcast-1.0.dtd category,omitempty"`
	Owner         ITunesOwner    `xml:"http://www.itunes.com/dtds/podcast-1.0.dtd owner,omitempty"`
	Language      string         `xml:"language,omitempty"`
	Item          []Item         `xml:"item"`
}

type AtomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr"`
}

type MediaThumbnail struct {
	URL string `xml:"url,attr"`
}

type MediaCategory struct {
	Scheme  string `xml:"scheme,attr"`
	Content string `xml:",chardata"`
}

type ITunesImage struct {
	Href string `xml:"href,attr"`
}

type ITunesCategory struct {
	Text string `xml:"text,attr"`
}

type ITunesOwner struct {
	Name  string `xml:"http://www.itunes.com/dtds/podcast-1.0.dtd name"`
	Email string `xml:"http://www.itunes.com/dtds/podcast-1.0.dtd email"`
}

type Item struct {
	Title        string        `xml:"title,omitempty"`
	Description  string        `xml:"description,omitempty"`
	PubDate      string        `xml:"pubDate,omitempty"`
	Link         string        `xml:"link,omitempty"`
	GUID         GUID          `xml:"guid,omitempty"`
	Author       string        `xml:"http://www.itunes.com/dtds/podcast-1.0.dtd author,omitempty"`
	Creator      string        `xml:"http://purl.org/dc/elements/1.1/ creator,omitempty"`
	Contributors []Contributor `xml:"http://www.w3.org/2005/Atom contributor,omitempty"`
	Explicit     string        `xml:"http://www.itunes.com/dtds/podcast-1.0.dtd explicit,omitempty"`
	Subtitle     string        `xml:"http://www.itunes.com/dtds/podcast-1.0.dtd subtitle,omitempty"`
	Duration     string        `xml:"http://www.itunes.com/dtds/podcast-1.0.dtd duration,omitempty"`
	Enclosure    Enclosure     `xml:"enclosure,omitempty"`
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
