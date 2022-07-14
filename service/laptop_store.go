package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"pcbook/pb"
	"sync"

	"github.com/jinzhu/copier"
)

var ErrAlreadyExists = errors.New("record already exists")

type LaptopStore interface {
	Save(laptop *pb.Laptop) error
	Find(id string) (*pb.Laptop, error)
	Search(ctx context.Context, filter *pb.Filter, found func(laptop *pb.Laptop) error) error
}

type InMemoryLaptopStore struct {
	mutex sync.RWMutex
	data  map[string]*pb.Laptop
}

func NewInMemoryLaptopStore() *InMemoryLaptopStore {
	return &InMemoryLaptopStore{
		data: make(map[string]*pb.Laptop),
	}
}

func (store *InMemoryLaptopStore) Save(laptop *pb.Laptop) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	if store.data[laptop.Id] != nil {
		return ErrAlreadyExists
	}

	other, err := deepCopy(laptop)
	if err != nil {
		return err
	}

	store.data[laptop.Id] = other
	return nil
}

func (store *InMemoryLaptopStore) Find(id string) (*pb.Laptop, error) {
	store.mutex.RLock()
	defer store.mutex.RUnlock()

	laptop := store.data[id]
	if laptop == nil {
		return nil, fmt.Errorf("laptop with id %s not found", id)
	}

	return deepCopy(laptop)
}

func (store *InMemoryLaptopStore) Search(ctx context.Context, filter *pb.Filter, found func(laptop *pb.Laptop) error) error {
	store.mutex.RLock()
	defer store.mutex.RUnlock()

	for _, laptop := range store.data {
		// time.Sleep(time.Second * 1)
		// log.Print("checking laptop id:", laptop.GetId())

		switch ctx.Err() {
		case context.Canceled:
			log.Print("CreateLaptop request is cancelled")
			return errors.New("cancelled")
		case context.DeadlineExceeded:
			log.Print("CreateLaptop request is timed out")
			return errors.New("timed out")
		default:
		}

		if isQualified(filter, laptop) {
			other, err := deepCopy(laptop)
			if err != nil {
				return err
			}

			err = found(other)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func isQualified(filter *pb.Filter, laptop *pb.Laptop) bool {
	if laptop.GetPriceUsd() > filter.GetMaxPriceUsd() {
		return false
	}

	if laptop.GetCpu().GetNumberCores() < filter.GetMinCpuCores() {
		return false
	}

	if laptop.GetCpu().GetMinGhz() < filter.GetMinCpuGhz() {
		return false
	}

	if toBit(laptop.GetRam()) < toBit(filter.GetMinRam()) {
		return false
	}

	return true
}

func toBit(ram *pb.Memory) uint64 {
	var shift int
	switch ram.Unit {
	case pb.Memory_BIT:
		shift = 0
	case pb.Memory_BYTE:
		shift = 3
	case pb.Memory_KILOBYTE:
		shift = 13
	case pb.Memory_MEGABYTE:
		shift = 23
	case pb.Memory_GIGABYTE:
		shift = 33
	case pb.Memory_TERABYTE:
		shift = 43
	default:
		return 0
	}
	return ram.GetValue() << shift
}

func deepCopy(laptop *pb.Laptop) (*pb.Laptop, error) {
	other := &pb.Laptop{}
	err := copier.Copy(other, laptop)
	if err != nil {
		return nil, fmt.Errorf("conot copy laptop: %w", err)
	}
	return other, nil
}
