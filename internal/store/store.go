package store

import (
	"sync"
)

type Store struct {
	mu        sync.RWMutex
	boards    map[string]*GoldPriceBoard
	histories map[string]map[string]*GoldPriceBoard
}

func NewStore() *Store {
	return &Store{
		boards:    make(map[string]*GoldPriceBoard),
		histories: make(map[string]map[string]*GoldPriceBoard),
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

func (s *Store) SetBoard(brand string, newBoard *GoldPriceBoard) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if board := s.boards[brand]; board == nil {
		s.setHistory(brand, newBoard)
	} else {
		if board.UpdatedAt != newBoard.UpdatedAt {
			s.setHistory(brand, newBoard)
		}
	}
}

func (s *Store) setHistory(brand string, newBoard *GoldPriceBoard) {
	s.boards[brand] = newBoard
	if history := s.histories[brand]; history == nil {
		history = make(map[string]*GoldPriceBoard)

		history[newBoard.UpdatedAt] = newBoard
		s.histories[brand] = history
	} else {
		if _, exist := history[newBoard.UpdatedAt]; !exist {
			history[newBoard.UpdatedAt] = newBoard
		}
	}
}

func (s *Store) GetBoard(brand string) *GoldPriceBoard {
	s.mu.RLock()
	board := s.boards[brand]
	s.mu.RUnlock()
	return board
}

func (s *Store) GetBoards() map[string]*GoldPriceBoard {
	s.mu.RLock()
	defer s.mu.RUnlock()

	boards := make(map[string]*GoldPriceBoard, len(s.boards))
	for k, v := range s.boards {
		boards[k] = v
	}
	return boards
}

func (s *Store) GetHistory(brand string) map[string]*GoldPriceBoard {
	s.mu.RLock()
	defer s.mu.RUnlock()

	history, ok := s.histories[brand]
	if !ok {
		return nil
	}

	rs := make(map[string]*GoldPriceBoard, len(history))
	for k, v := range history {
		rs[k] = v
	}
	return rs
}

func (s *Store) GetHistories() map[string]map[string]*GoldPriceBoard {
	s.mu.RLock()
	defer s.mu.RUnlock()

	histories := make(map[string]map[string]*GoldPriceBoard, len(s.histories))
	for k, v := range s.histories {
		history := make(map[string]*GoldPriceBoard, len(v))
		for sk, sv := range v {
			history[sk] = sv
		}
		histories[k] = v
	}
	return histories
}
