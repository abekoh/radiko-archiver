package radiko

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/abekoh/radiko-archiver/internal/config"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunFromURL(t *testing.T) {
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
		`=~^https:\/\/media\.radiko\.jp\/sound\/b\/LFR\/20231015\/20231015_[0-9]{6}_[a-zA-Z0-9]{5}\.aac$`,
		httpmock.NewXmlResponderOrPanic(http.StatusOK, httpmock.File("testdata/sample3.aac")))

	tempDir := t.TempDir()
	ctx := context.Background()
	tsURL := "https://radiko.jp/#!/ts/LFR/20231015010000"
	cnf := &config.Config{
		OutDirPath: tempDir,
	}
	RunFromURL(ctx, tsURL, cnf)

	xmlRes, err := os.ReadFile(filepath.Join(tempDir, "20231015010000_LFR_オードリーのオールナイトニッポン.xml"))
	require.NoError(t, err)
	assert.Equal(t, `<Prog ft="20231015010000" to="20231015030000" ftl="2500" tol="2700" dur="7200">
  <title>オードリーのオールナイトニッポン</title>
  <sub_title></sub_title>
  <desc>オードリーの2人が土曜の夜にじっくりお話してます。&lt;br&gt;若林さん、春日さんのそれぞれのトークが聴けるのは、オールナイトニッポンだけ！&lt;br&gt;テレビでは見せない2人の「素」を是非お聴きください！&lt;br&gt;&lt;br&gt;■1:00～1:35頃&lt;br&gt;オープニングトーク&lt;br&gt;&lt;br&gt;■1:40～1:55頃&lt;br&gt;東京ドームへの道&lt;br&gt;&lt;br&gt;■2:00頃～&lt;br&gt;若林フリートーク&lt;br&gt;&lt;br&gt;■2:20頃～&lt;br&gt;春日フリートーク&lt;br&gt;&lt;br&gt;■2:35頃～&lt;br&gt;「チン！」のコーナー&lt;br&gt;&lt;br&gt;■2:50頃～&lt;br&gt;「死んでもやめんじゃねーぞ！」&lt;br&gt;&lt;br&gt;■2:54頃～&lt;br&gt;エンディングトーク&lt;br&gt;&lt;br&gt;※トークが長くなるとコーナーはおやすみになります。</desc>
  <pfm>オードリー(若林正恭/春日俊彰)</pfm>
  <info>メールアドレス：&lt;br&gt;&lt;a href=&#34;mailto:kw@allnightnippon.com&#34;&gt;kw@allnightnippon.com&lt;/a&gt;&lt;br&gt;&lt;br&gt;番組ホームページは&lt;a href=&#34;https://www.allnightnippon.com/kw/&#34;&gt;こちら&lt;/a&gt;&lt;br&gt;&lt;br&gt;twitterハッシュタグは「&lt;a href=&#34;http://twitter.com/search?q=%23annkw&#34;&gt;#annkw&lt;/a&gt;」twitterアカウントは「&lt;a href=&#34;http://twitter.com/annkw5tyb&#34;&gt;@annkw5tyb&lt;/a&gt;」</info>
  <url>https://www.allnightnippon.com/kw/</url>
</Prog>`, string(xmlRes))

	aacRes, err := os.ReadFile(filepath.Join(tempDir, "20231015010000_LFR_オードリーのオールナイトニッポン.aac"))
	require.NoError(t, err)
	assert.Greater(t, len(aacRes), 0)
}
