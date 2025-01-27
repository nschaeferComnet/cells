package configx

import (
	json "github.com/pydio/cells/v4/common/utils/jsonx"
)

type ref struct {
	v map[string]interface{}
}

func Reference(s string) Ref {
	return &ref{
		v: map[string]interface{}{
			"$ref": s,
		},
	}
}

func GetReference(i interface{}) (string, bool) {
	if r, ok :=  i.(*ref); ok {
		return r.Get(), true
	}
	return "", false
}

func (r *ref) Get() string {
	return r.v["$ref"].(string)
}

func (r *ref) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.v)
}
