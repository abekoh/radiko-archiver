# radiko-archiver

**⚠️ Never use it for anything other than personal use. ⚠️**

## Features

- Automatically download the audio of Radiko programs in accordance with the rules. It only supports programs that allow time-shifted listening.
- Upload downloaded files into Dropbox.
- Provide a RSS feed page for Podcast.

## Requirements

- Go
- FFmpeg

## Setup

Install with `go install`

```go
go install github.com/abekoh/radiko-archiver/cmd/...@latest
```

Setup config.toml

```toml
out_dir_path = "out"
rules_path = "rules.toml"

[radiko]
# Download the audio file after offset_time has elapsed since the start of the program.
offset_time = "6h"
planner_interval = "10m"
fetch_timeout = "3m"

[feed]
enabled = true
port = 8080
base_url = "http://localhost:8080"

[dropbox]
enabled = true
```

Setup rules.toml
```toml
[[rules]]
name = "星野源のオールナイトニッポン"
station_id = "LFR"
weekday = "Wed"
start = "01:00"

[[rules]]
name = "バナナマンのバナナムーンGOLD"
station_id = "TBS"
weekday = "Sat"
start = "01:00"

[[rules]]
name = "オードリーのオールナイトニッポン"
station_id = "LFR"
weekday = "Sun"
start = "01:00"
```

Setup Dropbox token
```sh
export DROPBOX_TOKEN=XXXXXXXXXX
```

## Usage

Start workers.
```sh
radiko-archiver
```

With config path.
```sh
radiko-archiver -config myconfig.toml
```

Only download with radiko time-shifted URL.
```
radiko-archiver -now https://radiko.jp/#!/ts/LFR/20231001010000
```

## References

- [yyoshiki41/radigo: Record radiko 📻](https://github.com/yyoshiki41/radigo)
