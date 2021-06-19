package cache

import (
	"encoding/json"
	"fmt"
	"net/http"

	badger "github.com/dgraph-io/badger/v3"
	"github.com/fatih/color"
	"github.com/jay-dee7/parachute/types"
	"github.com/labstack/echo/v4"
)

type dataStore struct {
	db *badger.DB
}

type Store interface {
	Get(key []byte) ([]byte, error)
	Set(key, value []byte) error
	Update(key, value []byte) error
	ListAll() ([]byte, error)
	ListWithPrefix(prefix []byte) ([]byte, error)
	Delete(key []byte) error
	GetSkynetURL(key string, ref string) (string, error)
	ResolveManifestRef(namespace, ref string) (string, error)
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
		return ctx.String(http.StatusNotFound, err.Error())
	}

	return ctx.JSONBlob(http.StatusOK, val)
}

func (ds *dataStore) Update(key, value []byte) error {
	item, err := ds.Get(key)
	if err != nil {
		return ds.Set(key, value)
	}

	var resp types.Metadata
	if err = json.Unmarshal(item, &resp); err != nil {
		return err
	}

	var v types.Metadata
	if err = json.Unmarshal(value, &v); err != nil {
		return err
	}

	resp.Manifest.Layers = ds.removeDuplicateLayers(resp.Manifest.Layers, v.Manifest.Layers)
	resp.Manifest.Config = v.Manifest.Config
	resp.Manifest.MediaType = v.Manifest.MediaType
	resp.Manifest.SchemaVersion = v.Manifest.SchemaVersion

	bz, err := json.Marshal(resp)
	if err != nil {
		return err
	}

	return ds.Set(key, bz)
}

func (ds *dataStore) removeDuplicateLayers(src, dst []*types.Layer) []*types.Layer {
	list := make([]*types.Layer, len(src)+len(dst))
	list = append(list, src...)
	list = append(list, dst...)

	seenMap := make(map[string]bool)
	var layers []*types.Layer

	for _, l := range list {
		if l != nil && !seenMap[l.Digest] {
			seenMap[l.Digest] = true
			layers = append(layers, l)
		}
	}

	return layers
}

func (ds *dataStore) ResolveManifestRef(namespace, ref string) (string, error) {
	color.Yellow("key=%s ref=%s\n", namespace, ref)
	var res []byte
	fn := func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(namespace))
		if err != nil {
			return err
		}

		return item.Value(func(v []byte) error {
			res = make([]byte, len(v))
			copy(res, v)
			return nil
		})
	}

	err := ds.db.View(fn)

	if err != nil {
		return "", err
	}

	var md types.Metadata
	err = json.Unmarshal(res, &md)
	if err != nil {
		return "", err
	}

	mdRef := md.Manifest.Config.Reference
	mdDigest := md.Manifest.Config.Digest
	if ref == mdRef || ref == mdDigest {
		return md.Manifest.Config.SkynetLink, nil
	}

	return "", fmt.Errorf("ref not found")
}

func (ds *dataStore) GetSkynetURL(key, ref string) (string, error) {

	color.Yellow("key=%s ref=%s\n", key, ref)
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

	var md types.Metadata
	err = json.Unmarshal(res, &md)
	if err != nil {
		return "", err
	}

	return md.FindLinkForDigest(ref)
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
