package spawn

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"text/template"

	"github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/lotus/chain/wallet"

	"gopkg.in/yaml.v2"
)

//go:embed base.yaml
var valuesBase string

// merge values with parameters passed in reqest
// gives a values object that is ready to be used by helm
func dealbotValues(d *Daemon) (map[string]interface{}, error) {
	if !(d.Wallet != nil && d.Wallet.Address != "" && d.Wallet.Exported != "") {
		newWallet(d)
	}

	buf := bytes.NewBuffer([]byte{})
	tmpl, err := template.New("base").Parse(valuesBase)
	if err != nil {
		return nil, err
	}
	err = tmpl.Execute(buf, d)
	if err != nil {
		return nil, err
	}
	vals := make(map[string]interface{})
	err = yaml.Unmarshal(buf.Bytes(), &vals)
	if err != nil {
		log.Errorw("cannot unmarshal base yaml", "err", err)
		return nil, err
	}
	return vals, nil
}

func newWallet(d *Daemon) {
	w, _ := wallet.NewWallet(wallet.NewMemKeyStore())
	ctx := context.Background()
	a, _ := w.WalletNew(ctx, types.KTBLS)
	ki, _ := w.WalletExport(ctx, a)
	b, _ := json.Marshal(ki)
	d.Wallet = &Wallet{
		Address:  a.String(),
		Exported: hex.EncodeToString(b),
	}
}
