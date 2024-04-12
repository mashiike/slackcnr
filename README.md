# Slack Channel Name Resolver for golang

## Usage

```go
package main

import (
	"context"
	"errors"
	"log"
	"os"

	"github.com/mashiike/slackcnr"
	"github.com/slack-go/slack"
)

func main() {
	client := slack.New(os.Getenv("SLACK_BOT_TOKEN"))

	resolver := slackcnr.New(
		client,
		slackcnr.WithSearchPublicChannels(),
		slackcnr.WithExcludeArchived(),
	)
	ctx := context.Background()
	channel, err := resolver.Lookup(ctx, "general")
	if err != nil {
		if errors.Is(err, slackcnr.ErrNotFound) {
			log.Println("`general` channel not found")
			return
		}
		log.Println(err)
		return
	}
	log.Printf("channel: %s", channel.ID)
}
```

## License
MIT
