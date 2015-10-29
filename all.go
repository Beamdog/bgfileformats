package bg

import (
	"encoding/json"
	"strings"
)

type LONGSTRING struct {
	Value [32]byte
}

func (r *LONGSTRING) String() string {
	str := strings.Split(string(r.Value[0:]), "\x00")[0]
	return str
}

func (r *LONGSTRING) MarshalJSON() ([]byte, error) {
	return []byte("\"" + r.String() + "\""), nil
}

type RESREF struct {
	Name [8]byte
}

func NewResref(name string) RESREF {
	r := RESREF{}
	copy(r.Name[:], []byte(name))
	return r
}

func (r *RESREF) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.String())
}

func (r *RESREF) Valid() bool {
	return r.String() != ""
}

func (r RESREF) String() string {
	str := strings.Split(string(r.Name[0:]), "\x00")[0]
	return str
}
