package main

import (
	"context"
	"log"
	"time"

	"github.com/criticast/criticast/internal/goid"
	"github.com/criticast/criticast/internal/groundtruth"
)

func logGT(ctx context.Context, site, span string) {
	tok, ok := ctx.Value(requestTokenKey).(string)
	if !ok || tok == "" {
		log.Printf("groundtruth: missing token at site=%s", site)
		return
	}
	logG(tok, site, span, "")
}

func logG(token, site, span, extra string) {
	g, err := goid.Current()
	if err != nil {
		log.Printf("groundtruth: goid: %v", err)
	}
	rec := groundtruth.Record{
		TS:    time.Now().UTC(),
		Goid:  g,
		Token: token,
		Site:  site,
		Span:  span,
		Extra: extra,
	}
	line, err := rec.FormatLine()
	if err != nil {
		log.Printf("groundtruth: %v", err)
		return
	}
	log.Print(line)
}
