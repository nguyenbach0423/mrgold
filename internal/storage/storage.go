package storage

import (
	"context"
	"database/sql"
	"io"
	"net/http"
	"os"

	"github.com/coocood/freecache"
	"github.com/rs/zerolog/log"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
	_ "modernc.org/sqlite"
)

type Storage struct {
	googleDrive *drive.Service
	persistent  *sql.DB
	cache       *freecache.Cache
}

func NewStorage() (*Storage, error) {
	googleDrive, err := drive.NewService(context.Background(), option.WithCredentialsJSON([]byte(os.Getenv("GOOGLE_APIS_CREDENTIALS"))))
	if err != nil {
		return nil, err
	}

	var remoteFile *http.Response
	remoteFile, err = googleDrive.Files.Get(os.Getenv("GOOGLE_DRIVE_FILE_ID")).Download()
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = remoteFile.Body.Close()
	}()

	var localFile *os.File
	localFile, err = os.Create("/tmp/mrgold.sqlite")
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = localFile.Close()
	}()

	if _, err = io.Copy(localFile, remoteFile.Body); err != nil {
		return nil, err
	}

	dsn := "file:" + localFile.Name() + "?_foreign_keys=ON&_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000"

	persistent, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	persistent.SetMaxOpenConns(3)
	persistent.SetMaxIdleConns(1)

	cache := freecache.NewCache(100 * 1024 * 1024)

	s := &Storage{
		googleDrive: googleDrive,
		persistent:  persistent,
		cache:       cache,
	}

	//if err = s.migrate(); err != nil {
	//	s.Close()
	//	return nil, err
	//}

	//if err = s.seed(); err != nil {
	//	s.Close()
	//	return nil, err
	//}

	return s, nil
}

func (s *Storage) Sync() {
	localFile, err := os.Open("/tmp/mrgold.sqlite")
	if err != nil {
		log.Error().Err(err).Send()
		return
	}
	defer func() {
		_ = localFile.Close()
	}()

	_, err = s.googleDrive.Files.Update(os.Getenv("GOOGLE_DRIVE_FILE_ID"), nil).Media(localFile).Do()
	if err != nil {
		log.Error().Err(err).Send()
	}
}

func (s *Storage) Close() {
	if s.persistent != nil {
		_ = s.persistent.Close()
	}

	s.Sync()
}

func (s *Storage) migrate() error {
	queries := []string{
		`create table if not exists brands (
			id integer primary key autoincrement,
			code text not null unique,
			name text not null
		)`,
		`create table if not exists golds (
			id integer primary key autoincrement,
			brand_id integer not null,
			code text not null unique,
			name text not null,
			is_visible integer not null default 1,
			unique (brand_id, code),
    		foreign key (brand_id) references brands(id)
		)`,
		`create table if not exists gold_prices (
			id integer primary key autoincrement,
			gold_id integer not null,
			buy_price integer,
			sell_price integer,
			buy_price_text text,
			sell_price_text text,
			updated_at text not null,
			unique (gold_id, updated_at),
    		foreign key (gold_id) references golds(id)
		)`,
		`create table if not exists crawl_meta (
    		id integer primary key autoincrement,
			brand_id integer not null unique,
			updated_at text not null,
			crawled_at text not null,
			foreign key (brand_id) references brands(id)
		)`,
	}

	tx, err := s.persistent.Begin()
	if err != nil {
		return err
	}

	for _, query := range queries {
		_, err = tx.Exec(query)
		if err != nil {
			_ = tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}

func (s *Storage) seed() error {
	tx, err := s.persistent.Begin()
	if err != nil {
		return err
	}

	brands := []struct {
		Code  string
		Name  string
		Golds []struct {
			Code      string
			Name      string
			IsVisible int
		}
	}{
		{
			Code: "sjc", Name: "SJC",
			Golds: []struct {
				Code      string
				Name      string
				IsVisible int
			}{
				{"sjc01", "Vàng SJC 1L, 10L, 1KG", 1},
				{"sjc02", "Vàng SJC 5 chỉ", 1},
				{"sjc03", "Vàng SJC 0.5 chỉ, 1 chỉ, 2 chỉ", 1},
				{"sjc04", "Vàng nhẫn SJC 99,99% 1 chỉ, 2 chỉ, 5 chỉ", 1},
				{"sjc05", "Vàng nhẫn SJC 99,99% 0.5 chỉ, 0.3 chỉ", 1},
				{"sjc06", "Nữ trang 99,99%", 1},
				{"sjc07", "Nữ trang 99%", 1},
				{"sjc08", "Nữ trang 75%", 1},
				{"sjc09", "Nữ trang 68%", 0},
				{"sjc10", "Nữ trang 61%", 0},
				{"sjc11", "Nữ trang 58,3%", 1},
				{"sjc12", "Nữ trang 41,7%", 1},
			},
		},
		{
			Code: "doji", Name: "DOJI",
			Golds: []struct {
				Code      string
				Name      string
				IsVisible int
			}{
				{"doji01", "AVPL/SJC", 1},
				{"doji02", "Nhẫn tròn 9999 (Hưng Thịnh Vượng)", 1},
				{"doji03", "Nữ trang 9999", 1},
				{"doji04", "Nữ trang 999", 1},
			},
		},
		{
			Code: "pnj", Name: "PNJ",
			Golds: []struct {
				Code      string
				Name      string
				IsVisible int
			}{
				{"pnj01", "Vàng miếng SJC 999.9", 1},
				{"pnj02", "Nhẫn Trơn PNJ 999.9", 1},
				{"pnj03", "Vàng Kim Bảo 999.9", 1},
				{"pnj04", "Vàng Phúc Lộc Tài 999.9", 1},
				{"pnj05", "Vàng PNJ - Phượng Hoàng", 1},
				{"pnj06", "Vàng nữ trang 999.9", 1},
				{"pnj07", "Vàng nữ trang 999", 1},
				{"pnj08", "Vàng nữ trang 9920", 1},
				{"pnj09", "Vàng nữ trang 99", 1},
				{"pnj10", "Vàng 916 (22K)", 0},
				{"pnj11", "Vàng 750 (18K)", 1},
				{"pnj12", "Vàng 680 (16.3K)", 0},
				{"pnj13", "Vàng 650 (15.6K)", 0},
				{"pnj14", "Vàng 610 (14.6K)", 0},
				{"pnj15", "Vàng 585 (14K)", 1},
				{"pnj16", "Vàng 416 (10K)", 1},
				{"pnj17", "Vàng 375 (9K)", 0},
				{"pnj18", "Vàng 333 (8K)", 0},
			},
		},
		{
			Code: "btmc", Name: "Bảo Tín Minh Châu",
			Golds: []struct {
				Code      string
				Name      string
				IsVisible int
			}{
				{"btmc01", "Vàng miếng VRTL Bảo Tín Minh Châu", 1},
				{"btmc02", "Nhẫn tròn trơn Bảo Tín Minh Châu", 1},
				{"btmc03", "Quà mừng bản vị vàng Bảo Tín Minh Châu", 1},
				{"btmc04", "Vàng miếng SJC", 1},
				{"btmc05", "Trang sức Vàng Rồng Thăng Long 999.9", 1},
				{"btmc06", "Trang sức Vàng Rồng Thăng Long 99.9", 1},
			},
		},
		{
			Code: "btmh", Name: "Bảo Tín Mạnh Hải",
			Golds: []struct {
				Code      string
				Name      string
				IsVisible int
			}{
				{"btmh01", "Nhẫn ép vỉ Kim Gia Bảo", 1},
				{"btmh02", "Vàng miếng SJC (Cty CP BTMH)", 1},
				{"btmh03", "Nhẫn ép vỉ Vàng Rồng Thăng Long", 1},
				{"btmh04", "Đồng vàng Kim Gia Bảo hoa sen", 1},
				{"btmh05", "Vàng nữ trang 999.9", 1},
				{"btmh06", "Vàng nữ trang 99.9", 1},
				{"btmh07", "Tiểu Kim Cát - 0,3 chỉ", 1},
			},
		},
	}

	for _, brand := range brands {
		var result sql.Result
		result, err = tx.Exec(
			`insert or ignore into brands (code, name) values (?, ?)`,
			brand.Code, brand.Name,
		)
		if err != nil {
			_ = tx.Rollback()
			return err
		}

		var brandID int64
		brandID, err = result.LastInsertId()
		if err != nil {
			_ = tx.Rollback()
			return err
		}

		for _, gold := range brand.Golds {
			_, err = tx.Exec(
				`insert or ignore into golds (brand_id, code, name, is_visible) values (?, ?, ?, ?)`,
				brandID, gold.Code, gold.Name, gold.IsVisible,
			)
			if err != nil {
				_ = tx.Rollback()
				return err
			}
		}
	}

	return tx.Commit()
}

type Brand struct {
	ID   int
	Code string
	Name string
}

type Gold struct {
	ID        int
	BrandID   int
	Code      string
	Name      string
	IsVisible int
}

type GoldPrice struct {
	ID            int
	GoldID        int
	BuyPrice      int
	SellPrice     int
	BuyPriceText  string
	SellPriceText string
	UpdatedAt     string
}

type CrawlMeta struct {
	ID        int
	BrandID   int
	UpdatedAt string
	CrawledAt string
}

func (s *Storage) GetGoldIDs(brandCode string) map[string]int {
	goldIDs := make(map[string]int)

	query := `select id, code from golds where brand_id = (select id from brands where code = ?)`
	rows, err := s.persistent.Query(query, brandCode)
	if err != nil {
		return goldIDs
	}
	defer rows.Close()

	for rows.Next() {
		var goldID int
		var code string
		if err = rows.Scan(&goldID, &code); err != nil {
			continue
		}
		goldIDs[code] = goldID
	}

	return goldIDs
}

func (s *Storage) SaveGoldPrices(goldPrices []GoldPrice) error {
	tx, err := s.persistent.Begin()
	if err != nil {
		return err
	}

	for _, goldPrice := range goldPrices {
		_, err = tx.Exec(
			`insert into gold_prices (gold_id, buy_price, sell_price, buy_price_text, sell_price_text, updated_at) values (?, ?, ?, ?, ?, ?)`,
			goldPrice.GoldID, goldPrice.BuyPrice, goldPrice.SellPrice, goldPrice.BuyPriceText, goldPrice.SellPriceText, goldPrice.UpdatedAt,
		)
		if err != nil {
			_ = tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}
