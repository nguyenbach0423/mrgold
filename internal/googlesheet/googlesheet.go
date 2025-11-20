package googlesheet

import (
	"context"
	"os"

	"github.com/mrgold/internal/store"
	"github.com/rs/zerolog/log"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

type GoogleSheet struct {
	service *sheets.Service
	store   *store.Store
}

func NewGoogleSheet(credentials string, opts ...func(*GoogleSheet)) *GoogleSheet {
	service, err := sheets.NewService(context.Background(), option.WithCredentialsJSON([]byte(credentials)))
	if err != nil {
		log.Error().Err(err).Send()
		return nil
	}

	gs := &GoogleSheet{
		service: service,
		store:   store.NewStore(),
	}

	for _, opt := range opts {
		opt(gs)
	}

	return gs
}

func WithStore(store *store.Store) func(*GoogleSheet) {
	return func(gs *GoogleSheet) {
		gs.store = store
	}
}

func (gs *GoogleSheet) Sync() {
	gs.syncBoards()
	gs.syncHistories()
}

func (gs *GoogleSheet) syncBoards() {
	values := [][]interface{}{{"Brand", "BrandName", "GoldName", "BuyPrice", "SellPrice", "UpdatedAt"}}

	for brand, board := range gs.store.GetBoards() {
		for _, gold := range board.Golds {
			values = append(values, []interface{}{
				brand,
				board.BrandName,
				gold.Name,
				gold.BuyPrice,
				gold.SellPrice,
				board.UpdatedAt,
			})
		}
	}

	vr := &sheets.ValueRange{
		Values: values,
	}

	_ = gs.service.Spreadsheets.Values.Update(os.Getenv("GOOGLE_SHEET_ID"), "boards!A1", vr)
}

func (gs *GoogleSheet) syncHistories() {
	values := [][]interface{}{{"Brand", "BrandName", "GoldName", "BuyPrice", "SellPrice", "UpdatedAt"}}

	for brand, history := range gs.store.GetHistories() {
		for _, board := range history {
			for _, gold := range board.Golds {
				values = append(values, []interface{}{
					brand,
					board.BrandName,
					gold.Name,
					gold.BuyPrice,
					gold.SellPrice,
					board.UpdatedAt,
				})
			}
		}
	}

	vr := &sheets.ValueRange{
		Values: values,
	}

	_ = gs.service.Spreadsheets.Values.Update(os.Getenv("GOOGLE_SHEET_ID"), "histories!A1", vr)
}
