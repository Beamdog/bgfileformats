package bg

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
)

type idsEntry struct {
	Id   int
	Name string
	Args []idsArg
}

// Can be one of Object, Point, String, Integer
type idsArg struct {
	Type int
	Name string
}

type IDS struct {
	Entries []idsEntry
}

const (
	IDS_UNKNOWN = iota
	IDS_OBJECT
	IDS_ACTION
	IDS_STRING
	IDS_POINT
	IDS_INT
)

func typeToId(name string) int {
	switch name {
	case "O":
		return IDS_OBJECT
	case "A":
		return IDS_ACTION
	case "S":
		return IDS_STRING
	case "P":
		return IDS_POINT
	case "I":
		return IDS_INT
	default:
		log.Printf("WTF? : %s", name)
		return IDS_UNKNOWN
	}

}

func strToArg(arg string) (*idsArg, error) {
	argChunks := strings.Split(arg, ":")
	if len(argChunks) != 2 {
		return nil, fmt.Errorf("Arg: [%s], could not be split on :", arg)
	}
	nameChunks := strings.Split(argChunks[1], "*")

	return &idsArg{Type: typeToId(argChunks[0]), Name: nameChunks[0]}, nil
}

func OpenIDS(r io.ReadSeeker) (*IDS, error) {
	scanner := bufio.NewScanner(r)

	ids := IDS{}

	// Read header
	scanner.Scan()

	for scanner.Scan() {
		words := strings.SplitN(scanner.Text(), " ", 2)
		if len(words) != 2 {
			log.Printf("unable to split text: %s [%q]", scanner.Text(), words)
			continue
		}
		num, err := strconv.ParseUint(words[0], 0, 32)
		if err != nil {
			log.Printf("Unable to convert to number: %+v", err)
			continue
		}

		chunks := strings.Split(words[1], "(")
		// No arguments
		if len(chunks) == 1 {
			ids.Entries = append(ids.Entries, idsEntry{Id: int(num), Name: chunks[0]})
			// Has arguments
		} else if len(chunks) == 2 {
			function := chunks[0]
			strArgs := strings.Split(strings.TrimRight(chunks[1], ")"), ",")
			e := idsEntry{Id: int(num), Name: function}
			for _, arg := range strArgs {
				if arg != "" {
					args, err := strToArg(arg)
					if err != nil {
						log.Printf("Error converting %s to %+v %+v %+v", arg, err, strArgs, chunks)
					} else {
						e.Args = append(e.Args, *args)
					}
				}
			}
			ids.Entries = append(ids.Entries, e)
		} else {
			log.Printf("Error splitting text: %s", words[1])
			continue
		}
	}

	return &ids, nil
}
