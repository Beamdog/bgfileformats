package bg

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type keyHeader struct {
	Signature, Version [4]byte
	BifCount           uint32
	ResourceCount      uint32
	BifOffset          uint32
	ResourceOffset     uint32
}

type KeyHeader keyHeader

type keyBifEntry struct {
	Length         uint32
	OffsetFilename uint32
	LengthFilename uint16
	FileLocation   uint16
}

type keyResourceEntry struct {
	Name     RESREF
	Type     uint16
	Location uint32
}

type keyUniqueResource struct {
	Name string
	Type uint16
}

type KEY struct {
	header    keyHeader
	bifs      []keyBifEntry
	resources []keyResourceEntry
	r         io.ReadSeeker
	root      string
	files     map[keyUniqueResource]*keyResourceEntry
}

var fileTypes = map[string]int{
	"bmp":  1,
	"mve":  2,
	"tga":  3,
	"wav":  4,
	"wfx":  5,
	"plt":  6,
	"bam":  1000,
	"wed":  1001,
	"chu":  1002,
	"tis":  1003,
	"mos":  1004,
	"itm":  1005,
	"spl":  1006,
	"bcs":  1007,
	"ids":  1008,
	"cre":  1009,
	"are":  1010,
	"dlg":  1011,
	"2da":  1012,
	"gam":  1013,
	"sto":  1014,
	"wmp":  1015,
	"eff":  1016,
	"bs":   1017,
	"chr":  1018,
	"vvc":  1019,
	"vef":  1020,
	"pro":  1021,
	"bio":  1022,
	"wbm":  1023,
	"fnt":  1024,
	"gui":  1026,
	"sql":  1027,
	"pvrz": 1028,
	"glsl": 1029,
	"tot":  1030,
	"toh":  1031,
	"menu": 1032,
	"lua":  1033,
	"ini":  2050,
}
var fileTypesExt = map[int]string{}

func init() {
	for ext, num := range fileTypes {
		fileTypesExt[num] = ext
	}
}

func (res *keyResourceEntry) GetBifId() uint32 {
	return res.Location >> 20
}
func (res *keyResourceEntry) GetResourceId() uint32 {
	return res.Location & 0x3fff
}
func (res *keyResourceEntry) GetTilesetId() uint32 {
	return (res.Location & 0x000FC000) >> 14
}
func (res *keyResourceEntry) CleanName() string {
	return strings.ToUpper(strings.Trim(res.Name.String(), "\000"))
}

func OpenKEY(r io.ReadSeeker, root string) (*KEY, error) {
	key := &KEY{r: r, root: root}

	r.Seek(0, os.SEEK_SET)
	err := binary.Read(r, binary.LittleEndian, &key.header)
	if err != nil {
		return nil, err
	}

	r.Seek(int64(key.header.BifOffset), os.SEEK_SET)
	key.bifs = make([]keyBifEntry, key.header.BifCount)
	err = binary.Read(r, binary.LittleEndian, &key.bifs)
	if err != nil {
		return nil, err
	}

	r.Seek(int64(key.header.ResourceOffset), os.SEEK_SET)
	key.resources = make([]keyResourceEntry, key.header.ResourceCount)
	err = binary.Read(r, binary.LittleEndian, &key.resources)
	if err != nil {
		return nil, err
	}
	key.files = make(map[keyUniqueResource]*keyResourceEntry)
	for idx, res := range key.resources {
		kur := keyUniqueResource{Name: res.CleanName(), Type: res.Type}
		key.files[kur] = &key.resources[idx]
	}
	return key, nil
}

func (key *KEY) GetBifId(bifPath string) int {
	for idx, _ := range key.bifs {
		p, _ := key.GetBifPath(uint32(idx))
		thePath := strings.Trim(path.Base(p), " \000")
		if thePath == bifPath {
			return idx
		}
	}
	return -1

}

func (key *KEY) GetBifPath(bifId uint32) (string, error) {
	if int(bifId) > len(key.bifs) {
		return "", errors.New("Invalid bifId")
	}
	bifEntry := key.bifs[bifId]
	_, err := key.r.Seek(int64(bifEntry.OffsetFilename), os.SEEK_SET)
	if err != nil {
		return "", err
	}
	bufStr := make([]byte, bifEntry.LengthFilename)
	nBytes, err := io.ReadAtLeast(key.r, bufStr, int(bifEntry.LengthFilename))
	if err != nil {
		return "", err
	}
	return path.Clean(strings.Replace(strings.Trim(string(bufStr[0:nBytes]), "\000"), "\\", "/", -1)), nil
}

func (key *KEY) TypeToExt(ext uint16) string {
	return fileTypesExt[int(ext)]
}
func (key *KEY) ExtToType(ext string) int {
	fileExt := strings.Trim(ext, ".")
	return fileTypes[fileExt]
}
func (key *KEY) GetFilesByType(ext int) []string {
	var names []string
	for _, res := range key.resources {
		if res.Type == uint16(ext) {
			name := string(res.CleanName()) + "." + key.TypeToExt(res.Type)
			names = append(names, name)
		}
	}

	return names
}

func (key *KEY) GetResourceName(biffId uint32, resourceId uint32) (string, error) {
	nID := uint32((biffId << 20) | (resourceId & 0x3fff))
	for _, res := range key.resources {
		if res.Location == nID {
			name := string(res.CleanName()) + "." + key.TypeToExt(res.Type)
			return name, nil
		}
	}
	return "", errors.New("Resource not found")
}

func (key *KEY) Print() {
	log.Printf("Key Header:\n%+v\n", key.header)
	log.Printf("Bifs:\n")
	for _, bif := range key.bifs {
		log.Printf("\t%+v\n", bif)
	}
	log.Printf("Resources:\n")
	for _, res := range key.resources {
		log.Printf("\t%s.%s BifID: %d ResID: %d  Raw: %+v\n", res.CleanName(), key.TypeToExt(res.Type), res.GetBifId(), res.GetResourceId(), res)
	}
}

func (key *KEY) Validate() {
	for idx, bif := range key.bifs {
		bifPath, _ := key.GetBifPath(uint32(idx))

		fmt.Printf("Idx: %d Path: %s Location: %d\n", idx, bifPath, bif.FileLocation)
	}
	for _, resource := range key.resources {
		bifPath, err := key.GetBifPath(resource.GetBifId())
		if err != nil {
			log.Fatal(err)
		}
		diskPath := path.Join(key.root, bifPath)
		_, err = os.Stat(diskPath)
		if err != nil {
			//fmt.Printf("Can't find bif: %s\n", diskPath)
		}

	}
}

func (key *KEY) Explode(dir string) error {
	for _, res := range key.resources {
		bifPath, err := key.GetBifPath(res.GetBifId())
		if err != nil {
			return err
		}
		newPath := path.Clean(strings.Replace(bifPath, "\\", "/", -1))
		bifName := path.Base(newPath)
		dirName := path.Join(dir, strings.Replace(bifName, ".bif", "", 1))
		err = os.MkdirAll(dirName, 0777)
		if err != nil {
			return err
		}
		fileName := res.CleanName() + "." + key.TypeToExt(res.Type)
		if fileName != "." {
			fmt.Printf("Extracting %s to : %s/%s\n", fileName, dirName, fileName)
			data, err := key.OpenFile(fileName)
			if err != nil {
				log.Printf("Err: %v\n", err)
			} else {
				filePath := path.Clean(path.Join(dirName, fileName))
				outFile, err := os.Create(filePath)
				if err != nil {
					return err
				}
				_, err = outFile.Write(data)
				if err != nil {
					return err
				}
				outFile.Close()
			}
		}

	}
	return nil
}

func (key *KEY) OpenFile(name string) ([]byte, error) {
	resName := strings.ToUpper(strings.Split(name, ".")[0])
	resType := key.ExtToType(filepath.Ext(name))
	kur := keyUniqueResource{Name: resName, Type: uint16(resType)}
	res := key.files[kur]
	if res == nil {
		// Attempt to open from file system
		f, err := os.Open(filepath.Join(key.root, "override", name))
		if err != nil {
			return nil, fmt.Errorf("Unable to find file in key or override: %s", name)
		}
		buf, err := ioutil.ReadAll(f)
		if err != nil {
			return nil, fmt.Errorf("Unable to read file: %s", name)
		}
		return buf, nil
	}
	bifPath, _ := key.GetBifPath(res.GetBifId())

	f, err := os.Open(path.Join(key.root, bifPath))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	bif, err := OpenBif(f)
	if err != nil {
		return nil, err
	}
	buf, err := bif.ReadFile(res.Location)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

/*

func CreateKeyFromDir(input_dir string, output_dir string) error {
	paths := []string{}
	bifs := map[string][]string{}
	sortedBifs := []string{}

	input_dir = filepath.Clean(input_dir)
	output_dir = filepath.Clean(output_dir)

	filepath.Walk(input_dir, func(path string, info os.FileInfo, err error) error { paths = append(paths, path); return err })

	for _, filepath := range paths {
		chunks := strings.Split(filepath[len(input_dir):], string(os.PathSeparator))
		if len(chunks) > 2 {
			bifs[chunks[1]] = append(bifs[chunks[1]], chunks[len(chunks)-1])
		}
	}

	for k := range bifs {
		sortedBifs = append(sortedBifs, k)
	}
	sort.Strings(sortedBifs)

	keyFile, err := os.Create(filepath.Join(output_dir, "chitin.key"))
	if err != nil {
		return err
	}
	defer keyFile.Close()

	header := keyHeader{
		Signature: [4]byte{'K', 'E', 'Y', ' '},
		Version:   [4]byte{'V', '1', ' ', ' '},
	}
	header.BifCount = uint32(len(bifs))
	header.ResourceCount = uint32(len(paths))
	header.BifOffset = uint32(binary.Size(header))
	binary.Write(keyFile, binary.LittleEndian, header)

	fileNameOffset := uint32(binary.Size(header) + binary.Size(keyBifEntry{})*int(header.BifCount))

	bifEntryOffset, _ := keyFile.Seek(0, os.SEEK_CUR)
	for idx, bif := range sortedBifs {
		files := bifs[bif]
		biffSize, err := MakeBiffFromDir(filepath.Join(output_dir, "data", bif+".bif"), filepath.Join(input_dir, bif), files, idx)
		if err != nil {
			return err
		}
		bifInternalName := "data/" + bif + ".bif"
		log.Printf("Bif[%d]: %s.bif Size: %d\n", idx, bif, biffSize)
		keyFile.Seek(bifEntryOffset, os.SEEK_SET)
		entry := keyBifEntry{uint32(biffSize), uint32(fileNameOffset), uint16(len(bifInternalName)), 0}
		binary.Write(keyFile, binary.LittleEndian, entry)
		bifEntryOffset += int64(binary.Size(entry))

		keyFile.Seek(int64(fileNameOffset), os.SEEK_SET)
		keyFile.Write([]byte(bifInternalName[0:]))
		fileNameOffset += uint32(len(bifInternalName))
	}
	// rewrite header with correct resource offset
	keyFile.Seek(0, os.SEEK_SET)
	header.ResourceOffset = uint32(binary.Size(header)+binary.Size(keyBifEntry{})*int(header.BifCount)) + fileNameOffset
	binary.Write(keyFile, binary.LittleEndian, header)

	keyFile.Seek(int64(header.ResourceOffset), os.SEEK_SET)
	for biffId, bif := range sortedBifs {
		idx := 0
		files := bifs[bif]
		for _, file := range files {
			resName := strings.ToUpper(strings.Replace(file, filepath.Ext(file), "", 1))
			res := keyResourceEntry{Type: uint16(ExtToType(path.Ext(file))), Location: uint32(idx | (biffId << 20))}
			copy(res.Name[:], resName)

			binary.Write(keyFile, binary.LittleEndian, res)

			idx += 1
		}
	}

	return nil
}
*/
