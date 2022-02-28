package coin

import (
	"errors"
	"math/rand"
	"strconv"

	"xxx/contract"
	"xxx/crypto"
	"xxx/db"
	"xxx/log"
	"xxx/types"
)

var clog = log.New("contract.coin")

const (
	Name       = "coin"
	IssueOp    = "issue"
	TransferOp = "transfer"
	FreezeOp   = "freeze"
	UnfreezeOp = "unfreeze"

	CoinX int64 = 100000000
)

type Coin struct {
	*contract.Container
}

func Init(c *contract.Container) {
	c.Register(&Coin{c})
}

func (c *Coin) Name() string {
	return Name
}

func (c *Coin) Clone() contract.Contract {
	return &Coin{c.Container}
}

const keyBase = "contract-coin:"

func (c *Coin) Exec(from, to, op string, data []byte) error {
	if op == TransferOp {
		amount, err := types.UnmarshalAmount(data)
		if err != nil {
			return err
		}
		return c.Transfer(from, to, amount)
	} else if op == IssueOp {
		amount, err := types.UnmarshalAmount(data)
		if err != nil {
			return err
		}
		return c.Issue(to, amount)
	}
	return nil
}

func (c *Coin) SetIssue(amount int64) error {
	// db := c.GetDB()
	// issAll, err := QueryBalance("issue", db)
	// if err != nil {
	// 	clog.Error("query balance error", "err", err)
	// }
	// issAll += amount
	// buf := []byte(strconv.FormatInt(issAll, 10))
	// issue_key := keyBase + "issue"
	// return db.Set([]byte(issue_key), buf)
	return nil
}

func (c *Coin) Issue(to string, amount int64) error {
	db := c.GetDB()

	balance, err := QueryBalance(to, db)
	if err != nil {
		clog.Errorw("query balance error", "err", err)
	}

	balance += amount
	if balance < 0 {
		return errors.New("not enough balance")
	}
	clog.Infow("Issue", "to", to, "amount", amount)
	buf := []byte(strconv.FormatInt(balance, 10))
	return db.Set([]byte(keyBase+to), buf)
}

func (c *Coin) Fee(from string, amount int64) error {
	db := c.GetDB()

	balance, err := QueryBalance(from, db)
	if err != nil {
		clog.Errorw("query balance error", "err", err)
	}
	balance -= amount
	if balance < 0 {
		return errors.New("not enough balance")
	}
	// clog.Infow("coin.Fee", "from", from, "amount", amount)
	buf := []byte(strconv.FormatInt(balance, 10))
	return db.Set([]byte(keyBase+from), buf)
}

func (c *Coin) Transfer(from, to string, amount int64) error {
	// clog.Infow("coin.Transfer", "from", from, "to", to, "amount", amount)
	db := c.GetDB()
	from_balance, err := QueryBalance(from, db)
	if err != nil {
		return err
	}

	to_balance, _ := QueryBalance(to, db)

	from_balance -= amount
	if from_balance < 0 {
		return errors.New("not enough balance")
	}
	to_balance += amount
	if to_balance < 0 {
		return errors.New("not enough balance")
	}

	fromBuf := []byte(strconv.FormatInt(from_balance, 10))
	toBuf := []byte(strconv.FormatInt(to_balance, 10))

	db.Set([]byte(keyBase+from), fromBuf)
	db.Set([]byte(keyBase+to), toBuf)

	return nil
}

func QueryIssueAll(db db.KV) (int64, error) {
	return QueryBalance("issue", db)
}

func QueryBalance(addr string, db db.KV) (int64, error) {
	key := keyBase + addr
	val, err := db.Get([]byte(key))
	if err != nil {
		return 0, err
	}

	amount, err := strconv.ParseInt(string(val), 10, 64)
	if err != nil {
		return 0, err
	}
	return amount, nil
}

func CreateIssueTx(priv crypto.PrivateKey, to string, amount int64) (*types.Tx, error) {
	a := &types.Amount{
		A: amount,
	}

	data, err := types.Marshal(a)
	if err != nil {
		return nil, err
	}

	tx := &types.Tx{
		To:       to,
		Contract: Name,
		Op:       IssueOp,
		Data:     data,
		Height:   0,
		Nonce:    rand.Int63(),
	}
	tx.Sign(priv)
	return tx, nil
}

func CreateTransferTx(priv crypto.PrivateKey, to string, amount, height int64) (*types.Tx, error) {
	a := &types.Amount{
		A: amount,
	}

	data, err := types.Marshal(a)
	if err != nil {
		return nil, err
	}

	tx := &types.Tx{
		To:       to,
		Contract: Name,
		Op:       TransferOp,
		Data:     data,
		Height:   height,
		Nonce:    rand.Int63(),
	}

	tx.Sign(priv)
	return tx, nil
}

var coinAddr string

func init() {
	coinAddr = crypto.NewAddress([]byte(Name))
}

func Address() string {
	return coinAddr
}
