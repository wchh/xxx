package db

import (
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/util"
)

func NewLDB(path string) (DB, error) {
	db, err := leveldb.OpenFile(path, nil)
	if err != nil {
		return nil, err
	}
	return &LDB{db}, nil
}

type LDB struct {
	db *leveldb.DB
}

func (db *LDB) Get(key []byte) ([]byte, error) {
	return db.db.Get(key, nil)
}

func (db *LDB) Set(key, val []byte) error {
	return db.db.Put(key, val, nil)
}

func (db *LDB) Delete(key []byte) error {
	return db.db.Delete(key, nil)
}

func (db *LDB) Write(b Batch) error {
	return db.db.Write(b.(*lbatch).b, nil)
}

func (db *LDB) NewIter(r *Range) Iter {
	return db.db.NewIterator(&util.Range{Limit: r.Limit, Start: r.Start}, nil)
}

func (db *LDB) OpenTransaction() (Transaction, error) {
	t, err := db.db.OpenTransaction()
	if err != nil {
		return nil, err
	}
	return &ltranscation{t}, nil
}

type lbatch struct {
	b *leveldb.Batch
}

func NewBatch() Batch {
	return &lbatch{new(leveldb.Batch)}
}

func (b *lbatch) Set(key, val []byte) error {
	b.b.Put(key, val)
	return nil
}

func (b *lbatch) Delete(key []byte) error {
	b.b.Delete(key)
	return nil
}

type ltranscation struct {
	t *leveldb.Transaction
}

func (t *ltranscation) Get(key []byte) ([]byte, error) {
	return t.t.Get(key, nil)
}

func (t *ltranscation) Set(key, val []byte) error {
	return t.t.Put(key, val, nil)
}

func (t *ltranscation) Delete(key []byte) error {
	return t.t.Delete(key, nil)
}

func (t *ltranscation) Write(b Batch) error {
	return t.t.Write(b.(*lbatch).b, nil)
}

func (t *ltranscation) NewIter(r *Range) Iter {
	return t.t.NewIterator(&util.Range{Limit: r.Limit, Start: r.Start}, nil)
}

func (t *ltranscation) Commit() error {
	return t.t.Commit()
}

func (t *ltranscation) Discard() {
	t.t.Discard()
}

type BKV struct {
	KV
	B Batch
}

func NewBKV(kv KV) *BKV {
	return &BKV{kv, NewBatch()}
}

func (b *BKV) Set(key, val []byte) error {
	b.B.Set(key, val)
	return nil
}

func (b *BKV) Delete(key []byte) error {
	b.B.Delete(key)
	return nil
}
