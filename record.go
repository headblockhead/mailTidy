package main

import (
	"github.com/mitchellh/hashstructure/v2"
)

func NewDatabase() *Database {
	return &Database{
		hashMap: make(map[uint64]struct{}),
	}
}

type Database struct {
	hashMap map[uint64]struct{}
}

func (data *Database) GetResponded(msg *Message) (responded bool, err error) {
	hash, err := hashstructure.Hash(msg, hashstructure.FormatV2, nil)
	if err != nil {
		return false, err
	}
	_, found := data.hashMap[hash]
	return found, nil
}

func (data *Database) AddResponded(msg *Message) (err error) {
	hash, err := hashstructure.Hash(msg, hashstructure.FormatV2, nil)
	if err != nil {
		return err
	}
	data.hashMap[hash] = struct{}{}
	return nil
}

func (data *Database) RemoveResponded(msg *Message) (err error) {
	hash, err := hashstructure.Hash(msg, hashstructure.FormatV2, nil)
	if err != nil {
		return err
	}
	delete(data.hashMap, hash)
	return nil
}
