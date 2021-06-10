package cache

import (
	"encoding/json"
	"fmt"
	"net/http"

	badger "github.com/dgraph-io/badger/v3"
	"github.com/labstack/echo/v4"
)

type dataStore struct {
	db *badger.DB
}

type Metadata struct {
	Namespace string
	Layers    []Layer
	Manifests []Manifest
}

type Layer struct {
	Digest     string
	Size uint64
	UUID       string
	SkynetLink string
}

type Manifest struct {
	SkynetLink string
	Reference string
}

func (md Metadata) Bytes() []byte {
	bz, err := json.Marshal(md)
	if err != nil {
		fmt.Println(err.Error())
		return []byte{}
	}

	return bz
}

type Store interface {
	Set(key, value []byte) error
	Update(key, value []byte) error
	Get(key []byte) ([]byte, error)
	ListWithPrefix(prefix []byte) ([]byte, error)
	ListAll() ([]byte, error)
	Delete(key []byte) error
	GetSkynetURL(key string, args ...string) (string, error)
	Metadata(ctx echo.Context) error
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

func (ds *dataStore) Metadata(ctx echo.Context) error {
	key := ctx.QueryParam("namespace")

	val, err := ds.Get([]byte(key))
	if err != nil {
		ctx.String(http.StatusNotFound, err.Error())
	}

	return ctx.JSONBlob(http.StatusOK, val)
}

func (ds *dataStore) Update(key, value []byte) error {
	item, err := ds.Get(key)
	if err != nil {
		return ds.Set(key, value)
	}

	var resp Metadata
	if err = json.Unmarshal(item, &resp); err != nil {
		return err
	}

	var v Metadata
	if err = json.Unmarshal(value, &v); err != nil {
		return err
	}

	resp.Layers = append(resp.Layers, v.Layers...)
	resp.Manifests = append(resp.Manifests, v.Manifests...)

	bz, err := json.Marshal(resp)
	if err != nil {
		return err
	}

	return ds.Set(key, bz)
}

func (md Metadata) Find(ref string) (string, error) {
	for _, l := range md.Layers {
		if l.Digest == ref {
			return l.SkynetLink, nil
		}
	}

	for _, m := range md.Manifests {
		if m.Reference == ref {
			return m.SkynetLink, nil
		}
	}

	return "", fmt.Errorf("ref does not exists")
}

func (ds *dataStore) GetSkynetURL(key string, args ...string) (string, error) {

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

	var md Metadata
	err = json.Unmarshal(res, &md)
	if err != nil {
		return "", err
	}

	return md.Find(args[0])
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
	return ds.db.Close()
}
