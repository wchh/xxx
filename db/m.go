package db

import "sync"

// memery db and imp Transaction interface
type MDB struct {
	db DB
	mu sync.RWMutex
	mp map[string][]byte
}

func NewMDB(db DB) *MDB {
	return &MDB{db:db, mp:make(map[string][]byte)}
}

func (m *MDB) Get(key []byte) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	val, ok := m.mp[string(key)]
	if !ok {
		v, err := m.db.Get(key)
		if err != nil {
			return nil, err
		}
		val = v
	}
	return val, nil
}

func (m *MDB) Set(key, val []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.mp[string(key)] = val
	return nil
}

func (m *MDB) Delete(key []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.mp, string(key))
	return nil
}

func (m *MDB) Write(b Batch) error {
	panic("not support")
}

func (m *MDB) Commit() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var b Batch
	useBatch := len(m.mp) > 100
	if useBatch {
		b = NewBatch()
	} else {
		b = m.db
	}
	for k, v := range m.mp {
		err := b.Set([]byte(k), v)
		if err != nil {
			return err
		}
	}
	if useBatch {
		return m.db.Write(b)
	}
	return nil
}

func (m *MDB) Discard() {
}

func (m *MDB) NewIter(r *Range) Iter {
	return nil
}
