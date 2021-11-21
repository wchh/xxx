package ycc

import (
	"errors"
	"math/rand"
	"strconv"

	"xxx/contract"
	"xxx/contract/coin"
	"xxx/crypto"
	"xxx/db"
	"xxx/log"
	"xxx/types"
)

var ylog = log.New("ycc")

const (
	Name       = "ycc"
	DepositOp  = "deposit"
	WithdrawOp = "withdraw"
	PunishOp   = "punish"
	MineOp     = "mine"

	BlockReward = coin.CoinX * 30
	VoteReward  = coin.CoinX / 2        // 0.5 ycc
	MakerReward = coin.CoinX * 22 / 100 // 0.22 ycc
	VoteX       = coin.CoinX * 10000
)

type Ycc struct {
	*contract.Container
}

func Init(c *contract.Container) {
	c.Register(&Ycc{c})
}

func (c *Ycc) Name() string {
	return Name
}

func (c *Ycc) Clone() contract.Contract {
	return &Ycc{c.Container}
}

const keyBase = "contract-ycc:"

func (y *Ycc) Exec(from, to, op string, data []byte) error {
	if op == DepositOp {
		amount, err := types.UnmarshalAmount(data)
		if err != nil {
			return err
		}
		if to == "" {
			to = from
		}
		y.deposit(from, to, amount)
	} else if op == WithdrawOp {
		amount, err := types.UnmarshalAmount(data)
		if err != nil {
			return err
		}
		if to != "" {
			if from != to {
				return errors.New("to address is error")
			}
		} else {
			to = from
		}
		y.withdraw(from, to, amount)
	} else if op == MineOp {
		msg := new(types.Ycc_Mine)
		err := types.Unmarshal(data, msg)
		if err != nil {
			return err
		}
		y.mine(msg)
	} else {
		ylog.Error("not support")
	}
	return nil
}

func (y *Ycc) deposit(from, to string, amount int64) error {
	c := y.Container.GetContract(coin.Name).Clone()
	co := c.(*coin.Coin)
	err := co.Transfer(from, to, amount)
	if err != nil {
		return err
	}

	db := y.GetDB()
	err = setAllDeposit(amount, db)
	if err != nil {
		return err
	}

	dpa, err := QueryDeposit(from, db)
	if err != nil {
		ylog.Error("query deposit error", "err", err)
	}
	dpa += amount
	buf := strconv.FormatInt(dpa, 10)
	return db.Set([]byte(keyBase+from), []byte(buf))
}

func (y *Ycc) withdraw(from, to string, amount int64) error {
	db := y.GetDB()
	dpa, err := QueryDeposit(from, db)
	if err != nil {
		ylog.Error("query deposit error", "err", err)
	}
	dpa -= amount
	if dpa < 0 {
		return errors.New("not enough deposit")
	}
	buf := strconv.FormatInt(dpa, 10)
	err = db.Set([]byte(keyBase+from), []byte(buf))
	if err != nil {
		return err
	}

	err = setAllDeposit(-amount, db)
	if err != nil {
		return err
	}

	c := y.Container.GetContract(coin.Name).Clone()
	co := c.(*coin.Coin)
	return co.Transfer(Address(), to, amount)
}

var yccAddr string

func init() {
	yccAddr = crypto.NewAddress([]byte(Name))
}

func Address() string {
	return yccAddr
}

func (y *Ycc) Punish(addr string, amount int64) error {
	return y.withdraw(addr, y.FundAddr, amount)
}

func (y *Ycc) mine(msg types.Message) error {
	c := y.Container.GetContract(coin.Name).Clone()
	co := c.(*coin.Coin)
	all := BlockReward

	m := msg.(*types.Ycc_Mine)
	for _, v := range m.Votes {
		pub := (crypto.PublicKey)(v.Sig.PublicKey)
		amount := VoteReward * int64(len(v.MyHashs))
		err := co.Issue(pub.Address(), amount)
		if err != nil {
			return err
		}
		all -= amount
	}

	minepub := (crypto.PublicKey)(m.Sort.Sig.PublicKey)
	err := co.Issue(minepub.Address(), MakerReward)
	if err != nil {
		return err
	}

	all -= MakerReward
	return co.Issue(y.FundAddr, all)
}

func QueryAllDeposit(db db.KV) (int64, error) {
	key := keyBase + "all"
	val, err := db.Get([]byte(key))
	if err != nil {
		return 0, nil
	}

	return strconv.ParseInt(string(val), 10, 64)
}

func setAllDeposit(new int64, db db.KV) error {
	all, err := QueryAllDeposit(db)
	if err != nil {
		ylog.Error("query alldeposit error", "err", err)
	}

	key := keyBase + "all"
	return db.Set([]byte(key), []byte(strconv.FormatInt(all+new/VoteX, 10)))
}

func QueryDeposit(addr string, db db.KV) (int64, error) {
	key := keyBase + addr
	val, err := db.Get([]byte(key))
	if err != nil {
		return 0, nil
	}

	return strconv.ParseInt(string(val), 10, 64)
}

func CreateDepositTx(priv crypto.PrivateKey, addr string, amount, height int64) (*types.Tx, error) {
	d := &types.Amount{
		A: amount,
	}
	data, err := types.Marshal(d)
	if err != nil {
		return nil, err
	}

	tx := &types.Tx{
		To:       addr,
		Contract: Name,
		Op:       DepositOp,
		Data:     data,
		Height:   height,
		Nonce:    rand.Int63(),
	}
	tx.Sign(priv)
	return tx, nil
}

func CreateWithdrawTx(priv crypto.PrivateKey, amount, height int64) (*types.Tx, error) {
	d := &types.Amount{
		A: amount,
	}
	data, err := types.Marshal(d)
	if err != nil {
		return nil, err
	}

	tx := &types.Tx{
		Contract: Name,
		Op:       WithdrawOp,
		Data:     data,
		Height:   height,
		Nonce:    rand.Int63(),
	}
	tx.Sign(priv)
	return tx, nil
}
