package bg

import (
	"bytes"
	"io"
	"os"

	"code.google.com/p/lzma"
	//	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"path"
)

type bifMiniHeader struct {
	Signature, Version [4]byte
}

type bifHeader struct {
	Signature, Version [4]byte
	VarResCount        uint32
	FixedResCount      uint32
	TableOffset        uint32
}

type bifVarEntry struct {
	ResourceID uint32
	Offset     uint32
	Size       uint32
	Type       uint32
}

type bifFixedEntry struct {
	ResourceID uint32
	Offset     uint32
	Number     uint32
	Size       uint32
	Type       uint32
}

type BIF struct {
	Header          bifHeader
	VariableEntries []bifVarEntry
	FixedEntries    []bifFixedEntry
	r               io.ReadSeeker
}

func (res *bifVarEntry) GetBifId() uint32 {
	return res.ResourceID >> 20
}
func (res *bifVarEntry) GetResourceId() uint32 {
	return res.ResourceID & 0x3fff
}

func (bif *BIF) ReadFile(resourceId uint32) ([]byte, error) {
	for _, varRes := range bif.VariableEntries {
		if varRes.GetResourceId() == resourceId&0x3fff {
			out := make([]byte, varRes.Size)
			bif.r.Seek(int64(varRes.Offset), os.SEEK_SET)
			nBytes, err := io.ReadAtLeast(bif.r, out, int(varRes.Size))
			if err != nil {
				return nil, err
			}
			if nBytes != int(varRes.Size) {
				return nil, errors.New("Bytes read did not match size")
			}
			return out, nil
		}
	}
	fmt.Printf("Err: %v\n", bif.VariableEntries)
	errMsg := fmt.Sprintf("File not found: %d", resourceId)
	return nil, errors.New(errMsg)
}

func (bif *BIF) Print() {
	log.Printf("Header: %+v\n", bif.Header)
	for _, varRes := range bif.VariableEntries {
		log.Printf("\tRes: %+v\n", varRes)
	}
	for _, fixedRes := range bif.FixedEntries {
		log.Printf("\tRes: %+v\n", fixedRes)
	}

}

func OpenBif(r io.ReadSeeker) (*BIF, error) {
	bif := &BIF{r: r}

	header := bifMiniHeader{}
	err := binary.Read(r, binary.LittleEndian, &header)
	if err != nil {
		return nil, err
	}

	strSig := string(header.Signature[0:])
	strVer := string(header.Version[0:])
	// Stock biff
	if strSig == "BIFF" && strVer == "V1  " {
		r.Seek(0, os.SEEK_SET)

		err := binary.Read(r, binary.LittleEndian, &bif.Header)
		if err != nil {
			return nil, err
		}
		r.Seek(int64(bif.Header.TableOffset), os.SEEK_SET)
		bif.VariableEntries = make([]bifVarEntry, bif.Header.VarResCount)
		err = binary.Read(r, binary.LittleEndian, &bif.VariableEntries)
		if err != nil {
			return nil, err
		}

		bif.FixedEntries = make([]bifFixedEntry, bif.Header.FixedResCount)
		err = binary.Read(r, binary.LittleEndian, &bif.FixedEntries)
		if err != nil {
			return nil, err
		}
		return bif, nil
	} else if strSig == "BIF " && strVer == "V1.0" {
		r.Seek(0x008, os.SEEK_SET)
		filenamelength := uint32(0)
		err := binary.Read(r, binary.LittleEndian, &filenamelength)
		if err != nil {
			return nil, err
		}
		filename := make([]byte, filenamelength)
		err = binary.Read(r, binary.LittleEndian, &filename)
		if err != nil {
			return nil, err
		}
		uncompressedDataLength := uint32(0)
		r.Seek(int64(0x0010+filenamelength), os.SEEK_SET)
		err = binary.Read(r, binary.LittleEndian, &uncompressedDataLength)
		if err != nil {
			return nil, err
		}

		compressedDataLength := uint32(0)
		err = binary.Read(r, binary.LittleEndian, &compressedDataLength)
		if err != nil {
			return nil, err
		}

		//r points to the data now
		//decomp_reader, err := zlib.NewReader(r)

	} else if strSig == "BIFC" && strVer == "V1.0" {

	} else if strSig == "BIFL" && strVer == "V1.0" {
		return nil, errors.New("Already a BIFL")

	}
	return bif, nil
}

func RepackageBiff(keyFile io.ReadSeeker, bifIn io.ReadSeeker, filesPath string, bifOutPath string) error {
	key, err := OpenKEY(keyFile, "")
	if err != nil {
		return err
	}

	bif, err := OpenBif(bifIn)
	if err != nil {
		return err
	}

	bifOut, err := os.Create(bifOutPath)
	if err != nil {
		return err
	}
	defer bifOut.Close()

	bifName := path.Base(bifOutPath)

	outOffset := binary.Size(bif.Header)
	dataOffset := uint32(outOffset + binary.Size(bif.VariableEntries) + binary.Size(bif.FixedEntries))
	biffId := key.GetBifId(bifName)

	bif.Header.TableOffset = uint32(outOffset)
	err = binary.Write(bifOut, binary.LittleEndian, bif.Header)
	if err != nil {
		return err
	}
	for idx, entry := range bif.VariableEntries {
		res, err := key.GetResourceName(uint32(biffId), uint32(idx))
		if err != nil {
			return err
		}

		filePath := filesPath + "/" + res
		var dataIn []byte
		if fileInfo, err := os.Stat(filePath); os.IsNotExist(err) {
			fmt.Printf("No replacement found for: %s\n", res)
			dataIn = make([]byte, entry.Size)
			bifIn.Seek(int64(entry.Offset), os.SEEK_SET)
			bytesRead, err := io.ReadAtLeast(bifIn, dataIn, len(dataIn))
			if err != nil {
				return err
			}
			if bytesRead != len(dataIn) {
				fmt.Printf("Didnt read enough bytes\n")
				return nil
			}
		} else {
			entry.Size = uint32(fileInfo.Size())
			dataIn = make([]byte, entry.Size)
			fileIn, err := os.Open(filePath)
			if err != nil {
				return err
			}
			defer fileIn.Close()

			n, err := fileIn.Read(dataIn)
			if err != nil {
				return err
			}

			if n != len(dataIn) {
				fmt.Printf("Didnt read enough bytes\n")
				return nil
			}
		}

		bifOut.Seek(int64(outOffset), os.SEEK_SET)
		entry.Offset = dataOffset
		binary.Write(bifOut, binary.LittleEndian, entry)
		bifOut.Seek(int64(dataOffset), os.SEEK_SET)
		binary.Write(bifOut, binary.LittleEndian, dataIn)

		dataOffset += entry.Size
		outOffset += binary.Size(bif.VariableEntries[0])
	}
	for _, entry := range bif.FixedEntries {
		b := make([]byte, entry.Size*entry.Number)

		bifIn.Seek(int64(entry.Offset), os.SEEK_SET)
		io.ReadAtLeast(bifIn, b, len(b))

		bifOut.Seek(int64(outOffset), os.SEEK_SET)
		entry.Offset = dataOffset
		binary.Write(bifOut, binary.LittleEndian, entry)
		bifOut.Seek(int64(dataOffset), os.SEEK_SET)
		bifOut.Write(b)
		outOffset += binary.Size(bif.FixedEntries[0])
		dataOffset += uint32(len(b))
	}

	return nil
}

func ConvertToBIFL(r io.ReadSeeker, w io.WriteSeeker) error {
	r.Seek(0, os.SEEK_SET)
	w.Seek(0, os.SEEK_SET)
	bif, err := OpenBif(r)

	bif.Header.Signature = [4]byte{'B', 'I', 'F', 'L'}
	bif.Header.Version = [4]byte{'V', '1', '.', '0'}
	err = binary.Write(w, binary.LittleEndian, bif.Header)
	if err != nil {
		return err
	}
	err = binary.Write(w, binary.LittleEndian, bif.VariableEntries)
	if err != nil {
		return err
	}
	err = binary.Write(w, binary.LittleEndian, bif.FixedEntries)
	if err != nil {
		return err
	}
	outOffset := binary.Size(bif.Header)
	dataOffset := uint32(outOffset + binary.Size(bif.VariableEntries) + binary.Size(bif.FixedEntries))
	for _, entry := range bif.VariableEntries {
		dataIn := make([]byte, entry.Size)
		var lzmaOut bytes.Buffer
		out := lzma.NewWriter(&lzmaOut)

		r.Seek(int64(entry.Offset), os.SEEK_SET)
		io.ReadAtLeast(r, dataIn, len(dataIn))
		out.Write(dataIn)
		out.Close()

		w.Seek(int64(outOffset), os.SEEK_SET)
		entry.Offset = dataOffset
		binary.Write(w, binary.LittleEndian, entry)
		w.Seek(int64(dataOffset), os.SEEK_SET)
		compressedSize := uint32(lzmaOut.Len())
		if compressedSize < entry.Size {
			binary.Write(w, binary.LittleEndian, compressedSize)
			lzmaOut.WriteTo(w)
			// Length of compressed data plus 4 bytes for our 32bit int
			dataOffset += (compressedSize + 4)
		} else {
			fmt.Printf("Compressed size is larger: %d > %d\n", compressedSize, entry.Size)
			binary.Write(w, binary.LittleEndian, uint32(0))
			binary.Write(w, binary.LittleEndian, dataIn)
			dataOffset += (entry.Size + 4)
		}
		outOffset += binary.Size(bif.VariableEntries[0])
	}
	for _, entry := range bif.FixedEntries {
		b := make([]byte, entry.Size*entry.Number)
		//out := lzma.NewWriter(&b)

		r.Seek(int64(entry.Offset), os.SEEK_SET)
		io.ReadAtLeast(r, b, len(b))

		w.Seek(int64(outOffset), os.SEEK_SET)
		entry.Offset = dataOffset
		binary.Write(w, binary.LittleEndian, entry)
		w.Seek(int64(dataOffset), os.SEEK_SET)
		w.Write(b)
		outOffset += binary.Size(bif.FixedEntries[0])
		dataOffset += uint32(len(b))
	}
	return nil
}

/*
func MakeBiffFromDir(outputFile string, fileRoot string, files []string, biffId int) (int, error) {
	bifFile, err := os.Create(outputFile)
	if err != nil {
		return 0, err
	}
	defer bifFile.Close()

	header := &bifHeader{Signature: [4]byte{'B', 'I', 'F', 'F'}, Version: [4]byte{'V', '1', ' ', ' '}, VarResCount: uint32(len(files))}
	header.TableOffset = uint32(binary.Size(header))
	err = binary.Write(bifFile, binary.LittleEndian, header)
	if err != nil {
		return 0, err
	}
	entrySize := binary.Size(&bifVarEntry{})

	outOffset := binary.Size(header)
	dataOffset := uint32(outOffset + binary.Size(&bifVarEntry{})*len(files))

	for idx, file := range files {
		entry := &bifVarEntry{ResourceID: uint32(idx | (biffId << 20)), Type: uint32(ExtToType(filepath.Ext(file)))}

		filePath := filepath.Join(fileRoot, file)
		var dataIn []byte
		if fileInfo, err := os.Stat(filePath); os.IsNotExist(err) {
			return 0, err
		} else {
			entry.Size = uint32(fileInfo.Size())
			dataIn, err = ioutil.ReadFile(filePath)
			if err != nil {
				return 0, err
			}
		}

		bifFile.Seek(int64(outOffset), os.SEEK_SET)
		entry.Offset = dataOffset
		binary.Write(bifFile, binary.LittleEndian, entry)
		bifFile.Seek(int64(dataOffset), os.SEEK_SET)
		bifFile.Write(dataIn)
		//binary.Write(bifFile, binary.LittleEndian, dataIn)

		dataOffset += entry.Size
		outOffset += entrySize
	}
	return int(dataOffset), nil

}
*/
