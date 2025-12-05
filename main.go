package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/mrgold/internal/crawler"
	"github.com/mrgold/internal/googlesheet"
	"github.com/mrgold/internal/httpclient"
	"github.com/mrgold/internal/httpserver"
	"github.com/mrgold/internal/storage"
	"github.com/mrgold/internal/store"
	"github.com/mrgold/internal/telegram"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	stg, err := storage.NewStorage()
	if err != nil {
		log.Error().Err(err).Send()
		os.Exit(1)
	}

	s := store.NewStore()

	sheet := googlesheet.NewGoogleSheet(
		os.Getenv("GOOGLE_SHEET_CREDENTIALS"),
		googlesheet.WithStore(s),
	)
	if sheet == nil {
		os.Exit(1)
	}

	httpClient := httpclient.NewClient(
		httpclient.WithTimeout(5*time.Second),
		httpclient.WithRetryConfig(&httpclient.RetryConfig{
			MaxRetries: 3,
			Backoff:    200 * time.Millisecond,
			MaxBackoff: 1 * time.Second,
		}),
	)

	c := crawler.NewCrawler(
		stg,
		crawler.WithHTTPClient(httpClient),
		crawler.WithStore(s),
	)

	telegramClient := telegram.NewClient(
		telegram.WithHTTPClient(httpClient),
		telegram.WithBaseURL(os.Getenv("TELEGRAM_BOT_BASE_URL")),
		telegram.WithStore(s),
	)

	if ok := telegramClient.SetWebhook(os.Getenv("PUBLIC_DOMAIN")); !ok {
		os.Exit(1)
	}

	if ok := telegramClient.SetMyCommands(); !ok {
		os.Exit(1)
	}

	log.Info().Msg("telegram bot was configured")

	sheet.Fetch()

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()

		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Minute):
			}

			sheet.Sync()
			stg.Sync()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		for {
			c.Crawl()

			select {
			case <-ctx.Done():
				return
			case <-time.After(15 * time.Minute):
			}
		}
	}()

	httpServer := httpserver.NewServer(
		os.Getenv("PORT"),
		httpserver.WithTelegramClient(telegramClient),
	)

	go func() {
		if ok := httpServer.Start(); !ok {
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	cancel()
	wg.Wait()

	sheet.Sync()
	stg.Close()

	if ok := httpServer.Stop(); !ok {
		os.Exit(1)
	}
}

func init() {
	zerolog.TimeFieldFormat = time.DateTime
}
