package store

import (
	"sync"
)

type Store struct {
	mu     sync.RWMutex
	boards map[string]*GoldPriceBoard
}

func NewStore() *Store {
	return &Store{
		boards: make(map[string]*GoldPriceBoard),
	}
}

type Gold struct {
	Name      string
	BuyPrice  string
	SellPrice string
}

type GoldPriceBoard struct {
	BrandName string
	Golds     []Gold
	UpdatedAt string
}

func (s *Store) SetBoard(k string, v *GoldPriceBoard) {
	s.mu.Lock()
	s.boards[k] = v
	s.mu.Unlock()
}

func (s *Store) GetBoard(k string) *GoldPriceBoard {
	s.mu.RLock()
	v := s.boards[k]
	s.mu.RUnlock()
	return v
}
