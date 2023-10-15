package radiko

import (
	"context"
	"net/http"
	"testing"

	"github.com/abekoh/radiko-archiver/internal/config"
	"github.com/jarcoal/httpmock"
)

func TestRunFromURL(t *testing.T) {
	// given
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("GET",
		"http://radiko.jp/area",
		httpmock.NewStringResponder(http.StatusOK, `document.write('<span class="JP13">TOKYO JAPAN</span>');`))
	auth1Header := make(http.Header)
	auth1Header.Add("X-Radiko-AuthToken", "zsxMY0BGnwAuRpbelwK-JA")
	auth1Header.Add("X-Radiko-KeyLength", "16")
	auth1Header.Add("X-Radiko-KeyOffset", "10")
	httpmock.RegisterResponder("GET",
		"https://radiko.jp/v2/api/auth1",
		httpmock.NewStringResponder(http.StatusOK, `please send a part of key`).HeaderAdd(auth1Header))
	httpmock.RegisterResponder("GET",
		"https://radiko.jp/v2/api/auth2",
		httpmock.NewStringResponder(http.StatusOK, `JP13`))
	httpmock.RegisterResponder("GET",
		"https://radiko.jp/v3/program/date/20231014/JP13.xml",
		httpmock.NewXmlResponderOrPanic(http.StatusOK, httpmock.File("testdata/JP13.xml")))
	httpmock.RegisterResponder("POST",
		"https://radiko.jp/v2/api/ts/playlist.m3u8?ft=20231015010000&l=15&station_id=LFR&to=20231015030000",
		httpmock.NewXmlResponderOrPanic(http.StatusOK, httpmock.File("testdata/uri.m3u8")))
	httpmock.RegisterResponder("GET",
		"https://radiko.jp/v2/api/ts/chunklist/v1cA1fcZ.m3u8",
		httpmock.NewXmlResponderOrPanic(http.StatusOK, httpmock.File("testdata/v1cA1fcZ.m3u8")))
	httpmock.RegisterResponder("GET",
		"https://media.radiko.jp/sound/b/LFR/20231015/20231015_[0-9]{6}-[a-zA-Z0-9]{5}.aac",
		httpmock.NewXmlResponderOrPanic(http.StatusOK, httpmock.File("testdata/sample3.aac")))

	ctx := context.Background()
	tsURL := "https://radiko.jp/#!/ts/LFR/20231015010000"
	cnf := &config.Config{
		OutDirPath: t.TempDir(),
		Radiko: config.Radiko{
			OffsetTimeStr:      "",
			PlannerIntervalStr: "",
			FetchTimeoutStr:    "",
			OffsetTime:         0,
			PlannerInterval:    0,
			FetchTimeout:       0,
		},
	}

	RunFromURL(ctx, tsURL, cnf)

}
