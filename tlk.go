package bg

import (
	//"code.google.com/p/go-charset/charset"
	//_ "code.google.com/p/go-charset/data"
	//_ "code.google.com/p/go-charset/charset/iconv"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

type STRREF uint32

type tlkHeader struct {
	Signature, Version [4]byte
	LanguageID         uint16
	StringCount        uint32
	StringOffset       uint32
}

type tlkEntry struct {
	Flags  uint16
	Sound  RESREF
	Volume uint32
	Pitch  uint32
	Offset uint32
	Length uint32
}

type TLK struct {
	header    tlkHeader
	entries   []tlkEntry
	stringBuf []byte
	r         io.ReadSeeker
	codepage  string
}

type TlkJson struct {
	Strings map[string]string
	Sounds  map[string]string
}

const (
	TEXT_PRESENT  = 1
	SOUND_PRESENT = 2
	TOKEN_PRESENT = 4
)

func (t *TLK) SetCodepage(codepage string) {
	t.codepage = codepage
}

func (t *TLK) expandEntries(stringId int) {
	if len(t.entries) <= stringId {
		for {
			t.entries = append(t.entries, tlkEntry{})
			t.header.StringCount++
			if len(t.entries) > stringId {
				break
			}
		}
	}
}

func (t *TLK) GetStringCount() int {
	return len(t.entries)
}

func (t *TLK) String(stringId int) (string, error) {
	if stringId >= len(t.entries) {
		return "", errors.New(fmt.Sprintf("Index out of range: %d > %d", stringId, len(t.entries)))
	}
	if stringId < 0 {
		return "", nil
	}

	entry := t.entries[stringId]
	encodedString := string(t.stringBuf[entry.Offset : entry.Offset+entry.Length])
	return encodedString, nil
}

func (t *TLK) hasToken(str string) bool {
	return true
}

func (t *TLK) AddString(stringId int, str string, sound string) {
	//	fmt.Printf("StrId: %d\nString: %s\nSound: %s\n", stringId, str, sound)
	t.expandEntries(stringId)
	if len(str) > 0 {
		t.entries[stringId].Flags |= TEXT_PRESENT
	}
	if len(sound) > 0 {
		t.entries[stringId].Flags |= SOUND_PRESENT
	}
	if t.hasToken(str) {
		t.entries[stringId].Flags |= TOKEN_PRESENT
	}

	/*if sound != "" {
		copy(t.entries[stringId].Sound[0:], sound[0:])
	}*/
	t.entries[stringId].Offset = uint32(len(t.stringBuf))
	t.entries[stringId].Length = uint32(len(str))
	t.header.StringOffset = uint32(binary.Size(t.header)) + uint32(len(t.entries)*binary.Size(t.entries[0]))
	t.stringBuf = append(t.stringBuf, []byte(str)...)
}

func (t *TLK) Entry(stringId int) (*tlkEntry, error) {
	if stringId >= len(t.entries) {
		return nil, errors.New(fmt.Sprintf("Index out of range: %d >%d", stringId, len(t.entries)))
	}
	if stringId < 0 {
		return nil, nil
	}

	return &t.entries[stringId], nil
}

func (t *TLK) Write(w io.WriteSeeker) error {

	w.Seek(0, os.SEEK_SET)
	err := binary.Write(w, binary.LittleEndian, t.header)
	if err != nil {
		return err
	}
	err = binary.Write(w, binary.LittleEndian, t.entries)
	if err != nil {
		return err
	}
	w.Seek(0, os.SEEK_CUR)
	_, err = w.Write(t.stringBuf)
	if err != nil {
		return err
	}
	return nil

}
func (t *TLK) ConvertToUTF8(w io.WriteSeeker) error {
	w.Seek(0, os.SEEK_SET)
	err := binary.Write(w, binary.LittleEndian, t.header)
	if err != nil {
		return err
	}
	curStringOffset := 0
	strArray := []string{}
	for i := 0; i < len(t.entries); i++ {
		str, err := t.String(i)
		if err != nil {
			return err
		}
		strArray = append(strArray, str)
		t.entries[i].Offset = uint32(curStringOffset)
		t.entries[i].Length = uint32(len(str))
		curStringOffset += int(t.entries[i].Length)
		w.Seek(int64(binary.Size(t.header)+binary.Size(t.entries[0])*i), os.SEEK_SET)
		err = binary.Write(w, binary.LittleEndian, t.entries[i])
		if err != nil {
			return err
		}
	}
	w.Seek(int64(t.header.StringOffset), os.SEEK_SET)
	for _, str := range strArray {
		w.Write([]byte(str))
	}

	return nil
}

func (t *TLK) WriteJson(w io.WriteSeeker) error {
	out := TlkJson{}
	out.Strings = make(map[string]string, 0)
	out.Sounds = make(map[string]string, 0)

	for idx := 0; idx < t.GetStringCount(); idx++ {
		str, err := t.String(idx)
		stringId := fmt.Sprintf("%d", idx)
		if err != nil {
			return err
		}
		out.Strings[stringId] = str
		entry, err := t.Entry(idx)
		if err != nil {
			return err
		}
		if entry.Sound.Valid() {
			out.Sounds[stringId] = entry.Sound.String()
		}

	}
	bytes, err := json.MarshalIndent(out, "", "\t")
	if err != nil {
		return err
	}

	_, err = w.Write(bytes)

	return err
}

func NewTLK() (*TLK, error) {
	tlk := &TLK{codepage: "utf8"}

	tlk.header.Signature = [4]byte{'T', 'L', 'K', ' '}
	tlk.header.Version = [4]byte{'V', '1', ' ', ' '}
	tlk.header.LanguageID = 0

	tlk.entries = make([]tlkEntry, 0)
	tlk.stringBuf = make([]byte, 0)

	return tlk, nil

}

func OpenTlk(r io.ReadSeeker) (*TLK, error) {
	tlk := &TLK{r: r, codepage: "latin1"}
	tlkLen, err := r.Seek(0, os.SEEK_END)
	if err != nil {
		return nil, err
	}
	r.Seek(0, os.SEEK_SET)
	err = binary.Read(r, binary.LittleEndian, &tlk.header)
	if err != nil {
		return nil, err
	}

	tlk.entries = make([]tlkEntry, tlk.header.StringCount)
	err = binary.Read(r, binary.LittleEndian, &tlk.entries)
	if err != nil {
		return nil, err
	}
	tlkPos, err := r.Seek(0, os.SEEK_CUR)
	tlk.stringBuf = make([]byte, tlkLen-tlkPos)
	size, err := r.Read(tlk.stringBuf)
	if err != nil {
		return nil, err
	}
	if size != len(tlk.stringBuf) {
		return nil, err
	}

	return tlk, nil
}
