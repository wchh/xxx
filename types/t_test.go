package types

import (
	"math/rand"
	"testing"
	"xxx/crypto"
)

func TestTxSign(t *testing.T) {
	data := []byte("1234567890abc")
	tx := &Tx{
		To:       "3ftLmbf3MEPXTJYmd4HsRtKqUgy",
		Contract: "coin",
		Op:       "transfer",
		Data:     data,
		Nonce:    rand.Int63(),
	}
	priv, err := crypto.PrivateKeyFromString("0080242bfc85666aa8ce21846fa78d24898509fa8a60dd47ae80556798739617")
	if err != nil {
		t.Error(err)
		return
	}

	tx.Sign(priv)
	if !tx.Verify() {
		t.Error("tx sign and verify error")
	}
}
