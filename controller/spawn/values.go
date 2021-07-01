package spawn

import (
	"bytes"
	_ "embed"
	"text/template"

	"sigs.k8s.io/yaml"
)

//go:embed base.yaml
var valuesBase string

// merge values with parameters passed in reqest
// gives a values object that is ready to be used by helm
func dealbotValues(d *Daemon) (map[string]interface{}, error) {
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
