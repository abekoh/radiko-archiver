package radiko

import (
	"context"
)

func Run(ctx context.Context, rulesPath, outDirPath string) {
	toDispatcher := make(chan []Schedule)
	toFetcher := make(chan Schedule)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	RunPlanner(ctx, toDispatcher, rulesPath)
	RunDispatcher(ctx, toDispatcher, toFetcher)
	RunFetchers(ctx, toFetcher, outDirPath)
}
