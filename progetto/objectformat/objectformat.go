// Package objectformat definisce il formato dei file oggetto letti e prodotti dal mio linker
package objectformat

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

const LINK string = "LINK"

type ObjHeader struct {
	SegmentNum           uint
	SymbolNum            uint
	RelocationEntriesNum uint
}

type SegmentFlag int

const (
	Readable SegmentFlag = iota
	Writable
	Present
)

func (f SegmentFlag) String() string {
	switch f {
	case Readable:
		return "R"
	case Writable:
		return "W"
	case Present:
		return "P"
	default:
		return "?"
	}
}

var segmentFlagParsingMap = map[string]SegmentFlag{
	"R": Readable,
	"W": Writable,
	"P": Present,
}

func parseSegmentFlags(segmentFlags string) ([]SegmentFlag, error) {
	var res []SegmentFlag

	for _, c := range segmentFlags {
		if v, ok := segmentFlagParsingMap[string(c)]; ok {
			res = append(res, v)
		} else {
			// flag sconosciuta
			return nil, fmt.Errorf("trovata flag sconosciuta %s", string(c))
		}
	}

	return res, nil
}

type Segment struct {
	Name         string
	StartAddress uint // hex value
	Length       uint // in bytes
	Flags        []SegmentFlag
}

// Next comes the symbol table. Each entry is of the form:
// name value seg kind
// The name is the symbol name.
// The value is the hex value of the symbol.
// Seg is the segment number relative to which the symbol is defined, or 0 for absolute or undefined symbols.
// The kind is a string of letters including D for defined or U for undefined.
// Symbols are also numbered in the order they’re listed, starting at 1.
type symbolKind int

const (
	Defined symbolKind = iota
	Undefined
)

var symbolKindParsingMap = map[string]symbolKind{
	"D": Defined,
	"U": Undefined,
}

func (sk symbolKind) String() string {
	switch sk {
	case Defined:
		return "D"
	case Undefined:
		return "U"
	default:
		return "?"
	}
}

func parseSymbolKind(kind string) (symbolKind, error) {
	if v, ok := symbolKindParsingMap[kind]; ok {
		return v, nil
	}

	return 0, fmt.Errorf("symbolKind %s non riconosciuto", kind)
}

type Symbol struct {
	Name   string
	Value  uint // hex value
	Segnum uint
	Kind   symbolKind
}

// Next come the relocations, one to a line:
// loc seg ref kind ...
// Loc is the location to be relocated,
// seg is the segment within which the location is found,
// ref is the segment or symbol number to be relocated there,
// and kind is an architecture-dependent relocation type. Common types are A4 for a four-byte absolute address, or R4 for a four-byte relative address.
// Some relocation types may have extra fields after the type.
type relocationKind int

const (
	Absolute4 relocationKind = iota
	Relative4
)

var relocationKindParsingMap = map[string]relocationKind{
	"A4": Absolute4,
	"R4": Relative4,
}

func (rk relocationKind) String() string {
	switch rk {
	case Absolute4:
		return "A4"
	case Relative4:
		return "R4"
	default:
		return "?"
	}
}

func parseRelocationKind(kind string) (relocationKind, error) {
	if v, ok := relocationKindParsingMap[kind]; ok {
		return v, nil
	}

	return 0, fmt.Errorf("symbolKind %s non riconosciuto", kind)
}

type RelocationEntry struct {
	Loc    uint // hex value
	Segnum uint
	Ref    uint // segment or symbol number
	Kind   relocationKind
}

type SegmentData []byte

// MyObjectFormat è il formato finale
type MyObjectFormat struct {
	Filename        string
	Header          ObjHeader
	SegmentTable    []Segment
	SymbolTable     []Symbol
	RelocationTable []RelocationEntry
	Data            []SegmentData
}

// helper per ignorare blanks e commenti
func getNextLine(scanner *bufio.Scanner) (string, error) {
	more := scanner.Scan()
	if !more {
		if err := scanner.Err(); err != nil {
			return "", fmt.Errorf("errore durante la lettura del file: %w", err)
		}
		return "", io.EOF
	}

	line := scanner.Text()
	for strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
		scanner.Scan()
		line = scanner.Text()
	}

	return line, nil
}

func ParseObjectFile(filename string) (*MyObjectFormat, error) {
	obj := &MyObjectFormat{}
	var err error

	f, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("impossibile aprire file %s: %w", filename, err)
	}
	defer f.Close()

	// It returns false when there are no more tokens, either by reaching the end of the input or an error.
	// After Scan returns false, the Scanner.Err method will return any error that occurred during scanning,
	// except that if it was io.EOF, Scanner.Err will return nil.
	scanner := bufio.NewScanner(f)

	obj.Filename = filename

	/* parsing dell'header == prime due linee */
	magic, err := getNextLine(scanner)
	if err != nil {
		return nil, err
	}
	if magic != LINK {
		return nil, fmt.Errorf("magic number sbagliato! %s non è del formato giusto", filename)
	}

	objDims, err := getNextLine(scanner)
	if err != nil {
		return nil, err
	}
	_, err = fmt.Sscanf(objDims, "%d %d %d", &obj.Header.SegmentNum, &obj.Header.SymbolNum, &obj.Header.RelocationEntriesNum)
	if err != nil {
		return nil, fmt.Errorf("errore nella lettura dell'header: %w", err)
	}
	fmt.Println("### HEADER", obj.Header)

	obj.SegmentTable = make([]Segment, 0, obj.Header.SegmentNum)
	obj.SymbolTable = make([]Symbol, 0, obj.Header.SymbolNum)
	obj.RelocationTable = make([]RelocationEntry, 0, obj.Header.RelocationEntriesNum)
	obj.Data = make([]SegmentData, 0, obj.Header.SegmentNum)

	/* parsing dei segmenti */
	var i uint = 0
	for ; i < obj.Header.SegmentNum; i++ {
		segmentString, err := getNextLine(scanner)
		if err != nil {
			return nil, err
		}

		var s Segment
		var segmentFlags string
		_, err = fmt.Sscanf(segmentString, "%s %x %d %s", &s.Name, &s.StartAddress, &s.Length, &segmentFlags)
		if err != nil {
			return nil, fmt.Errorf("errore nella lettura del segmento %d -> %s: %w", i+1, segmentString, err)
		}
		s.Flags, err = parseSegmentFlags(segmentFlags)
		if err != nil {
			return nil, err
		}
		obj.SegmentTable = append(obj.SegmentTable, s)
	}
	fmt.Println("### Segmenti", obj.SegmentTable)

	/* parsing dei simboli */
	i = 0
	for ; i < obj.Header.SymbolNum; i++ {
		symbolString, err := getNextLine(scanner)
		if err != nil {
			return nil, err
		}

		var s Symbol
		var kindString string
		_, err = fmt.Sscanf(symbolString, "%s %x %d %s", &s.Name, &s.Value, &s.Segnum, &kindString)
		if err != nil {
			return nil, fmt.Errorf("errore nella lettura del simbolo %d -> %s: %w", i+1, symbolString, err)
		}
		s.Kind, err = parseSymbolKind(kindString)
		if err != nil {
			return nil, err
		}
		obj.SymbolTable = append(obj.SymbolTable, s)
	}
	fmt.Println("### Simboli", obj.SymbolTable)

	/* parsing delle relocation entries */
	i = 0
	for ; i < obj.Header.RelocationEntriesNum; i++ {
		relocationString, err := getNextLine(scanner)
		if err != nil {
			return nil, err
		}

		var r RelocationEntry
		var kindString string
		_, err = fmt.Sscanf(relocationString, "%x %d %d %s", &r.Loc, &r.Segnum, &r.Ref, &kindString)
		if err != nil {
			return nil, fmt.Errorf("errore nella lettura della relocation entry %d -> %s: %w", i+1, relocationString, err)
		}
		r.Kind, err = parseRelocationKind(kindString)
		if err != nil {
			return nil, err
		}
		obj.RelocationTable = append(obj.RelocationTable, r)
	}
	fmt.Println("### Relocation entries", obj.RelocationTable)

	/* dati dei segmenti */
	i = 0
	for ; i < obj.Header.SegmentNum; i++ {
		segmentDataHexString, err := getNextLine(scanner)
		if err != nil {
			return nil, err
		}
		segmentData, err := hex.DecodeString(segmentDataHexString)
		if err != nil {
			return nil, err
		}

		obj.Data = append(obj.Data, segmentData)
	}
	fmt.Println("### Dati dei segmenti", obj.Data)

	return obj, nil
}

func (obj *MyObjectFormat) WriteObjectFile(filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("impossibile aprire file %s: %w", filename, err)
	}
	defer f.Close()

	// magic
	_, err = fmt.Fprintln(f, "LINK")
	if err != nil {
		return err
	}
	// header
	_, err = fmt.Fprintf(f, "%d %d %d\n", obj.Header.SegmentNum, obj.Header.SymbolNum, obj.Header.RelocationEntriesNum)
	if err != nil {
		return err
	}
	// segments
	fmt.Fprintln(f, "# segments")
	for i := 0; i < int(obj.Header.SegmentNum); i++ {
		flags := ""
		for _, f := range obj.SegmentTable[i].Flags {
			flags += f.String()
		}

		_, err = fmt.Fprintf(f, "%s %d %d %s\n", obj.SegmentTable[i].Name, obj.SegmentTable[i].StartAddress, obj.SegmentTable[i].Length, flags)
		if err != nil {
			return err
		}
	}
	// symbols
	fmt.Fprintln(f, "# symbols")
	for i := 0; i < int(obj.Header.SymbolNum); i++ {
		_, err = fmt.Fprintf(f, "%s %d %d %s\n", obj.SymbolTable[i].Name, obj.SymbolTable[i].Value, obj.SymbolTable[i].Segnum, obj.SymbolTable[i].Kind.String())
		if err != nil {
			return err
		}
	}
	// relocatins
	fmt.Fprintln(f, "# relocations")
	for i := 0; i < int(obj.Header.RelocationEntriesNum); i++ {
		_, err = fmt.Fprintf(f, "%d %d %d %s\n", obj.RelocationTable[i].Loc, obj.RelocationTable[i].Segnum, obj.RelocationTable[i].Ref, obj.RelocationTable[i].Kind.String())
		if err != nil {
			return err
		}
	}
	// data
	fmt.Fprintln(f, "# segment data")
	for i := 0; i < int(obj.Header.SegmentNum); i++ {
		_, err = fmt.Fprintln(f, hex.EncodeToString(obj.Data[i]))
		if err != nil {
			return err
		}
	}
	return nil
}
