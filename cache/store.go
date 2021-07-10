package cache

import (
	"encoding/json"
	"fmt"
	"net/http"

	badger "github.com/dgraph-io/badger/v3"
	"github.com/jay-dee7/parachute/types"
	"github.com/labstack/echo/v4"
)

type dataStore struct {
	db *badger.DB
}

type Store interface {
	Get(key []byte) ([]byte, error)
	Set(key, value []byte) error
	GetDigest(digest string) (*types.LayerRef, error)
	SetDigest(digest, skylink string) error
	Update(key, value []byte) error
	ListAll() ([]byte, error)
	ListWithPrefix(prefix []byte) ([]byte, error)
	Delete(key []byte) error
	GetSkynetURL(key string, ref string) (string, error)
	ResolveManifestRef(namespace, ref string) (string, error)
	Metadata(ctx echo.Context) error
	LayerDigests(ctx echo.Context) error
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

func (ds *dataStore) LayerDigests(ctx echo.Context) error {
	bz, err := ds.ListWithPrefix([]byte(layerDigestNamespace))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	return ctx.JSONBlob(http.StatusOK, bz)
}

func (ds *dataStore) Metadata(ctx echo.Context) error {
	key := ctx.QueryParam("namespace")
	if key == "" {
		bz, err := ds.ListAll()
		if err != nil {
			return ctx.JSON(http.StatusOK, echo.Map{
				"message": "so empty!!",
			})
		}

		var metadataList []types.Metadata
		if err = json.Unmarshal(bz, &metadataList); err != nil {
			return ctx.JSON(http.StatusInternalServerError, echo.Map{
				"error": err.Error(),
			})
		}

		return ctx.JSON(http.StatusOK, metadataList)
	}

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

	if len(v.Manifest.Layers) != 0 {
		resp.Manifest.Layers = ds.removeDuplicateLayers(resp.Manifest.Layers, v.Manifest.Layers)
	}

	resp.Manifest.Config = append(resp.Manifest.Config, v.Manifest.Config...)

	if v.Manifest.MediaType != "" {
		resp.Manifest.MediaType = v.Manifest.MediaType
	}

	if v.Manifest.SchemaVersion != 0 {
		resp.Manifest.SchemaVersion = v.Manifest.SchemaVersion
	}

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

	if err := ds.db.View(fn); err != nil {
		return "", err
	}

	var md types.Metadata
	err := json.Unmarshal(res, &md)
	if err != nil {
		return "", err
	}

	for _, c := range md.Manifest.Config {
		mdRef := c.Reference
		mdDigest := c.Digest
		if ref == mdRef || ref == mdDigest {
			return c.SkynetLink, nil
		}
	}

	return "", fmt.Errorf("ref not found")
}

const layerDigestNamespace = "layers/digests"

func (ds *dataStore) SetDigest(digest, skylink string) error {
	key := fmt.Sprintf("%s/%s", layerDigestNamespace, digest)
	value := types.LayerRef{
		Digest:  digest,
		Skylink: skylink,
	}

	if err := ds.Set([]byte(key), value.Bytes()); err != nil {
		return err
	}

	return nil
}

func (ds *dataStore) GetDigest(digest string) (*types.LayerRef, error) {
	key := fmt.Sprintf("%s/%s", layerDigestNamespace, digest)
	bz, err := ds.Get([]byte(key))
	if err != nil {
		return nil, err
	}

	var layerRef types.LayerRef
	err = json.Unmarshal(bz, &layerRef)
	return &layerRef, err
}

func (ds *dataStore) GetSkynetURL(key, ref string) (string, error) {

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
	var buf []types.Metadata

	err := ds.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			err := item.Value(func(v []byte) error {
				var md types.Metadata
				if err := json.Unmarshal(v, &md); err != nil {
					return err
				}
				buf = append(buf, md)
				return nil
			})

			if err != nil {
				return err
			}
		}
		return nil
	})

	bz, _ := json.Marshal(buf)

	return bz, err
}

func (ds *dataStore) ListWithPrefix(prefix []byte) ([]byte, error) {
	var buf []*types.LayerRef

	err := ds.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(v []byte) error {
				var layerRef types.LayerRef
				if err := json.Unmarshal(v, &layerRef); err != nil {
					return err
				}

				buf = append(buf, &layerRef)
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})

	bz, _ := json.Marshal(buf)
	return bz, err
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
