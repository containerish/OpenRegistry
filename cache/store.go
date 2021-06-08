package cache

import (
	"encoding/json"
	"strings"

	badger "github.com/dgraph-io/badger/v3"
)

type dataStore struct {
	db *badger.DB
}

type Store interface {
	Set(key, value []byte) error
	Update(key, value []byte) error
	Get(key []byte) ([]byte, error)
	ListWithPrefix(prefix []byte) ([]byte, error)
	ListAll() ([]byte, error)
	Delete(key []byte) error
	GetSkynetURL(key string, args ...string) (string, error)
	Close() error
}

func New(storeLocation string) (Store, error) {
	if storeLocation == "" {
		storeLocation = "/tmp/badger"
	}

	db, err := badger.Open(badger.DefaultOptions(storeLocation))
	if err != nil {
		return nil, err
	}

	return &dataStore{db: db}, nil
}

func (ds *dataStore) Update(key, value []byte) error {
	item, err := ds.Get(key)
	if err != nil {
		return ds.Set(key, value)
	}

	resp := make(map[string]string)
	if err = json.Unmarshal(item, &resp); err != nil {
		return err
	}

	v := make(map[string]string)
	if err = json.Unmarshal(value, &v); err != nil {
		return err
	}

	for k, v := range v {
		resp[k] = v
	}

	bz, err := json.Marshal(resp)
	if err != nil {
		return err
	}

	return ds.Set(key, bz)
}

func (ds *dataStore) GetSkynetURL(key string, args ...string) (string, error) {
	if len(args) > 0 {
		key = key + "/" + strings.Join(args, "/")
	}

	var res []byte
	err := ds.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}

		return item.Value(func(v []byte) error {
			res = make([]byte, len(v))
			copy(res, v)
			return nil
		})
	})

	if err != nil {
		return "", err
	}

	var info struct {
		SkynetLink string `json:"skynetLink"`
	}

	err = json.Unmarshal(res, &info)
	return info.SkynetLink, err

}

func (ds *dataStore) Set(key, value []byte) error {
	txn := ds.db.NewTransaction(true)

	if err := txn.Set(key, value); err != nil {
		return err
	}

	return txn.Commit()
}

func (ds *dataStore) Get(key []byte) ([]byte, error) {
	var res []byte

	err := ds.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		err = item.Value(func(v []byte) error {
			res = make([]byte, len(v))
			copy(res, v)
			return nil
		})
		return err
	})
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (ds *dataStore) ListAll() ([]byte, error) {
	var res []byte

	err := ds.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			err := item.Value(func(v []byte) error {
				res = make([]byte, len(v))
				copy(res, v)
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})

	return res, err
}

func (ds *dataStore) ListWithPrefix(prefix []byte) ([]byte, error) {
	var res []byte

	err := ds.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(v []byte) error {
				res = make([]byte, len(v))
				copy(res, v)
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})

	return res, err
}

func (ds *dataStore) Delete(key []byte) error {
	txn := ds.db.NewTransaction(true)
	if err := txn.Delete(key); err != nil {
		return err
	}

	return txn.Commit()
}

func (ds *dataStore) Close() error {
	return ds.Close()
}
