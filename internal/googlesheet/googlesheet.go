package googlesheet

import (
	"context"
	"fmt"
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
	vr, err := gs.service.Spreadsheets.Values.Get(gs.spreadsheetId, "boards!A2:F").Do()
	if err != nil {
		log.Error().Err(err).Send()
		return
	}

	var boards = make(map[string]*store.GoldPriceBoard)
	for _, value := range vr.Values {
		if len(value) < 7 {
			continue
		}

		brand := value[0].(string)
		brandName := value[1].(string)
		goldCode := value[2].(string)
		goldName := value[3].(string)
		buyPrice := value[4].(string)
		sellPrice := value[5].(string)
		updatedAt := value[6].(string)

		if board, exist := boards[brand]; !exist {
			boards[brand] = &store.GoldPriceBoard{
				BrandName: brandName,
				Golds: []store.Gold{
					{
						Code:      goldCode,
						Name:      goldName,
						BuyPrice:  buyPrice,
						SellPrice: sellPrice,
					},
				},
				UpdatedAt: updatedAt,
			}
		} else {
			board.Golds = append(board.Golds, store.Gold{
				Code:      goldCode,
				Name:      goldName,
				BuyPrice:  buyPrice,
				SellPrice: sellPrice,
			})
		}
	}

	gs.store.SetBoards(boards)
}

func (gs *GoogleSheet) fetchHistories() {
	vr, err := gs.service.Spreadsheets.Values.Get(gs.spreadsheetId, "histories!A2:F").Do()
	if err != nil {
		log.Error().Err(err).Send()
		return
	}

	var histories = make(map[string]map[string]*store.GoldPriceBoard)
	for _, value := range vr.Values {
		if len(value) < 7 {
			continue
		}

		idMatrixSJC := map[string]string{
			"Vàng SJC 1L, 10L, 1KG":                    "01",
			"Vàng SJC 5 chỉ":                           "02",
			"Vàng SJC 0.5 chỉ, 1 chỉ, 2 chỉ":           "03",
			"Vàng nhẫn SJC 99,99% 1 chỉ, 2 chỉ, 5 chỉ": "04",
			"Vàng nhẫn SJC 99,99% 0.5 chỉ, 0.3 chỉ":    "05",
			"Nữ trang 99,99%":                          "06",
			"Nữ trang 99%":                             "07",
			"Nữ trang 75%":                             "08",
			"Nữ trang 58,3%":                           "09",
			"Nữ trang 41,7%":                           "10",
		}

		idMatrixDOJI := map[string]string{
			"AVPL/SJC":                          "01",
			"Nhẫn tròn 9999 (Hưng Thịnh Vượng)": "02",
			"Nữ trang 9999":                     "03",
			"Nữ trang 999":                      "04",
		}

		idMatrixPNJ := map[string]string{
			"Vàng miếng SJC 999.9":    "01",
			"Nhẫn Trơn PNJ 999.9":     "02",
			"Vàng Kim Bảo 999.9":      "03",
			"Vàng Phúc Lộc Tài 999.9": "04",
			"Vàng PNJ - Phượng Hoàng": "05",
			"Vàng nữ trang 999.9":     "06",
			"Vàng nữ trang 999":       "07",
			"Vàng nữ trang 99":        "08",
			"Vàng 750 (18K)":          "09",
			"Vàng 585 (14K)":          "10",
			"Vàng 416 (10K)":          "11",
		}

		idMatrixBTMC := map[string]string{
			"Vàng miếng VRTL Bảo Tín Minh Châu":      "01",
			"Nhẫn tròn trơn Bảo Tín Minh Châu":       "02",
			"Quà mừng bản vị vàng Bảo Tín Minh Châu": "03",
			"Vàng miếng SJC":                         "04",
			"Trang sức Vàng Rồng Thăng Long 999.9":   "05",
			"Trang sức Vàng Rồng Thăng Long 99.9":    "06",
		}

		idMatrixBTMH := map[string]string{
			"Nhẫn ép vỉ Kim Gia Bảo":          "01",
			"Vàng miếng SJC (Cty CP BTMH)":    "02",
			"Nhẫn ép vỉ Vàng Rồng Thăng Long": "03",
			"Đồng vàng Kim Gia Bảo hoa sen":   "04",
			"Vàng nữ trang 999.9":             "05",
			"Vàng nữ trang 99.9":              "06",
			"Tiểu Kim Cát - 0,3 chỉ":          "07",
		}

		code := ""
		brand := value[0].(string)
		name := value[3].(string)
		if brand == "sjc" {
			code = idMatrixSJC[name]
		} else if brand == "doji" {
			code = idMatrixDOJI[name]
		} else if brand == "pnj" {
			code = idMatrixPNJ[name]
		} else if brand == "btmc" {
			code = idMatrixBTMC[name]
		} else if brand == "btmh" {
			code = idMatrixBTMH[name]
		}

		//brand := value[0].(string)
		brandName := value[1].(string)
		goldCode := code
		goldName := value[3].(string)
		buyPrice := value[4].(string)
		sellPrice := value[5].(string)
		updatedAt := value[6].(string)

		if history, exist := histories[brand]; !exist {
			history = make(map[string]*store.GoldPriceBoard)
			history[updatedAt] = &store.GoldPriceBoard{
				BrandName: brandName,
				Golds: []store.Gold{
					{
						Code:      goldCode,
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
							Code:      goldCode,
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
					Code:      goldCode,
					Name:      goldName,
					BuyPrice:  buyPrice,
					SellPrice: sellPrice,
				})
			}
		}
	}

	fmt.Println(histories)

	gs.store.SetHistories(histories)
}

func (gs *GoogleSheet) Sync() {
	gs.syncBoards()
	gs.syncHistories()
}

func (gs *GoogleSheet) syncBoards() {
	values := [][]interface{}{{"Brand", "BrandName", "GoldCode", "GoldName", "BuyPrice", "SellPrice", "UpdatedAt"}}

	for brand, board := range gs.store.GetBoards() {
		for _, gold := range board.Golds {
			values = append(values, []interface{}{
				brand,
				board.BrandName,
				gold.Code,
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
	values := [][]interface{}{{"Brand", "BrandName", "GoldCode", "GoldName", "BuyPrice", "SellPrice", "UpdatedAt"}}

	for brand, history := range gs.store.GetHistories() {
		for _, board := range history {
			for _, gold := range board.Golds {
				values = append(values, []interface{}{
					brand,
					board.BrandName,
					gold.Code,
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
