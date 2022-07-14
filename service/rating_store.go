package service

import "sync"

type RatingStore interface {
	Add(laptopId string, rating float64) (*Rating, error)
}

type Rating struct {
	Count uint32
	Sum   float64
}

type InMemoryRatingStore struct {
	mutex   sync.RWMutex
	ratings map[string]*Rating
}

func NewInMemoryRatingStore() *InMemoryRatingStore {
	return &InMemoryRatingStore{ratings: make(map[string]*Rating)}
}

func (store *InMemoryRatingStore) Add(laptopId string, rating float64) (*Rating, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	r, ok := store.ratings[laptopId]
	if !ok {
		r = &Rating{
			Count: 1,
			Sum:   rating,
		}
	} else {
		r.Count++
		r.Sum += rating
	}
	store.ratings[laptopId] = r
	return r, nil
}
