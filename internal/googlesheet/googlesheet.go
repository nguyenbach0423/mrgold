package googlesheet

import (
	"context"

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

	return &GoogleSheet{
		service: service,
		store:   store.NewStore(),
	}
}

func WithStore(store *store.Store) func(*GoogleSheet) {
	return func(gs *GoogleSheet) {
		gs.store = store
	}
}

func (gs *GoogleSheet) SyncData() {

}
