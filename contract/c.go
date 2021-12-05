package contract

import (
	"errors"
	"xxx/config"
	"xxx/crypto"
	"xxx/db"
	"xxx/log"
	"xxx/types"
)

var clog = new(log.Logger)

func init() {
	log.Register("contract", clog)
}

type feeContract interface {
	Fee(from string, amount int64) error
}

type Contract interface {
	// Check(op string, data []byte) error
	Exec(from, to, op string, data []byte) error
	Name() string
	Clone() Contract
}

// type Base struct {
// 	db *db.StateDB
// }

// func (b *Base) Check(msg types.Message) error { return nil }
// func (b *Base) Exec(msg types.Message) error  { return nil }
// func (b *Base) Name() string                  { return "base" }
// func (b *Base) Clone() Contract               { return nil }
// func (b *Base) GetDB() *db.StateDB            { return b.db }

type Container struct {
	*config.ConstractConfig
	contractMap map[string]Contract
	db          db.KV
	height      int64
}

func New(conf *config.ConstractConfig) *Container {
	c := &Container{contractMap: make(map[string]Contract), ConstractConfig: conf}
	// c.Register(&Base{db: db})
	return c
}

func (cl *Container) Register(c Contract) {
	cl.contractMap[c.Name()] = c
}

func (cl *Container) GetContract(name string) Contract {
	return cl.contractMap[name]
}

func (c *Container) SetHeight(height int64) {
	c.height = height
}

func (c *Container) Height() int64 {
	return c.height
}

func (cl *Container) GetDB() db.KV {
	return cl.db
}

func (cl *Container) SetDB(db db.KV) {
	cl.db = db
}

func (cl *Container) processFee(from string) error {
	fee := cl.GetContract("coin").(feeContract)
	return fee.Fee(from, cl.TxFee)
}

func (cl *Container) ExecTx(tx *types.Tx) error {
	clog.Infow("container.ExecTx", "contract", tx.Contract, "op", tx.Op, "to", tx.To)
	from := crypto.PubkeyToAddr(tx.Sig.PublicKey)
	if cl.height != 0 {
		err := cl.processFee(from)
		if err != nil {
			return err
		}
	}

	c := cl.GetContract(tx.Contract)
	if c == nil {
		return errors.New("contract NOT support")
	}
	return c.Exec(from, tx.To, tx.Op, tx.Data)
}

// var DefaultContainer = &Container{contractMap: make(map[string]Contract)}
