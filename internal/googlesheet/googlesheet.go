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
	service       *sheets.Service
	store         *store.Store
	spreadsheetId string
}

func NewGoogleSheet(credentials string, opts ...func(*GoogleSheet)) *GoogleSheet {
	service, err := sheets.NewService(context.Background(), option.WithCredentialsJSON([]byte(credentials)))
	if err != nil {
		log.Error().Err(err).Send()
		return nil
	}

	gs := &GoogleSheet{
		service:       service,
		store:         store.NewStore(),
		spreadsheetId: os.Getenv("GOOGLE_SHEET_ID"),
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

func (gs *GoogleSheet) Fetch() {
	gs.fetchBoards()
	gs.fetchHistories()
}

func (gs *GoogleSheet) fetchBoards() {
	vr, err := gs.service.Spreadsheets.Values.Get(gs.spreadsheetId, "boards!A2").Do()
	if err != nil {
		log.Error().Err(err).Send()
		return
	}

	var boards = make(map[string]*store.GoldPriceBoard)
	for _, value := range vr.Values {
		log.Info().Interface("value", value).Msg("no")

		if len(value) < 6 {
			continue
		}

		brand := value[0].(string)
		brandName := value[1].(string)
		goldName := value[2].(string)
		buyPrice := value[3].(string)
		sellPrice := value[4].(string)
		updatedAt := value[5].(string)

		if board, exist := boards[brand]; !exist {
			boards[brand] = &store.GoldPriceBoard{
				BrandName: brandName,
				Golds: []store.Gold{
					{
						Name:      goldName,
						BuyPrice:  buyPrice,
						SellPrice: sellPrice,
					},
				},
				UpdatedAt: updatedAt,
			}
		} else {
			board.Golds = append(board.Golds, store.Gold{
				Name:      goldName,
				BuyPrice:  buyPrice,
				SellPrice: sellPrice,
			})
		}
	}

	gs.store.SetBoards(boards)
}

func (gs *GoogleSheet) fetchHistories() {
	vr, err := gs.service.Spreadsheets.Values.Get(gs.spreadsheetId, "histories!A2").Do()
	if err != nil {
		log.Error().Err(err).Send()
		return
	}

	var histories = make(map[string]map[string]*store.GoldPriceBoard)
	for _, value := range vr.Values {
		log.Info().Interface("value", value).Msg("yes")

		if len(value) < 6 {
			continue
		}

		brand := value[0].(string)
		brandName := value[1].(string)
		goldName := value[2].(string)
		buyPrice := value[3].(string)
		sellPrice := value[4].(string)
		updatedAt := value[5].(string)

		if history, exist := histories[brand]; !exist {
			history = make(map[string]*store.GoldPriceBoard)
			history[updatedAt] = &store.GoldPriceBoard{
				BrandName: brandName,
				Golds: []store.Gold{
					{
						Name:      goldName,
						BuyPrice:  buyPrice,
						SellPrice: sellPrice,
					},
				},
				UpdatedAt: updatedAt,
			}

			histories[brand] = history
		} else {
			var board *store.GoldPriceBoard
			if board, exist = history[updatedAt]; !exist {
				board = &store.GoldPriceBoard{
					BrandName: brandName,
					Golds: []store.Gold{
						{
							Name:      goldName,
							BuyPrice:  buyPrice,
							SellPrice: sellPrice,
						},
					},
					UpdatedAt: updatedAt,
				}
				history[updatedAt] = board
			} else {
				board.Golds = append(board.Golds, store.Gold{
					Name:      goldName,
					BuyPrice:  buyPrice,
					SellPrice: sellPrice,
				})
			}
		}
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

	_, err := gs.service.Spreadsheets.Values.Update(os.Getenv("GOOGLE_SHEET_ID"), "boards!A1", vr).ValueInputOption("RAW").Do()
	if err != nil {
		log.Error().Err(err).Send()
	}
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

	_, err := gs.service.Spreadsheets.Values.Update(os.Getenv("GOOGLE_SHEET_ID"), "histories!A1", vr).ValueInputOption("RAW").Do()
	if err != nil {
		log.Error().Err(err).Send()
	}
}
