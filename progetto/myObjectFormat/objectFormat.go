package objectFormat

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

const LINK string = "LINK"

type objHeader struct {
	segment_num            uint
	symbol_num             uint
	relocation_entries_num uint
}

// Next comes the segment definitions. Each segment definition contains the segment name, the address where the segment logically starts, the length of the segment in bytes,
// and a string of code letters describing the segment.
// Code letters include R for readable, W for writable, and P for present in the object file. (Other letters may be present as well.)
//
//	A typical set of segments for an a.out like file would be:
//
// .text 1000 2500 RP
// .data 4000 C00 RWP
// .bss 5000 1900 RW
// Segments are numbered in the order their definitions appear, with the first segment being number 1.
type segmentFlag int

const (
	Readable segmentFlag = iota
	Writable
	Present
)

func (f segmentFlag) String() string {
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

var segmentFlagParsingMap = map[string]segmentFlag{
	"R": Readable,
	"W": Writable,
	"P": Present,
}

func parseSegmentFlags(segmentFlags string) ([]segmentFlag, error) {
	var res []segmentFlag

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

type segment struct {
	name          string
	start_address uint
	length        uint // in bytes
	flags         []segmentFlag
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

type symbol struct {
	name   string
	value  uint // hex value ?
	segnum uint
	kind   symbolKind
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

type relocationEntry struct {
	loc    uint
	segnum uint
	ref    uint // segment or symbol number
	kind   relocationKind
}

// Following the relocations comes the object data. The data for each segment is a single long hex string followed by a newline.
// Each pair of hex digits represents one byte.
type segmentData []byte

// il formato finale è quindi
type MyObjectFormat struct {
	header          objHeader
	segmentTable    []segment
	symbolTable     []symbol
	relocationTable []relocationEntry
	data            []segmentData
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

	/* parsing dell'header == prime due linee */
	magic, err := getNextLine(scanner)
	if err != nil {
		return nil, err
	}
	if magic != LINK {
		return nil, fmt.Errorf("magic number sbagliato! %s non è del formato giusto", filename)
	}

	obj_dims, err := getNextLine(scanner)
	if err != nil {
		return nil, err
	}
	_, err = fmt.Sscanf(obj_dims, "%d %d %d", &obj.header.segment_num, &obj.header.symbol_num, &obj.header.relocation_entries_num)
	if err != nil {
		return nil, fmt.Errorf("errore nella lettura dell'header: %w", err)
	}
	fmt.Println("### HEADER", obj.header)

	obj.segmentTable = make([]segment, 0, obj.header.segment_num)
	obj.symbolTable = make([]symbol, 0, obj.header.symbol_num)
	obj.relocationTable = make([]relocationEntry, 0, obj.header.relocation_entries_num)
	obj.data = make([]segmentData, 0, obj.header.segment_num)

	/* parsing dei segmenti */
	var i uint = 0
	for ; i < obj.header.segment_num; i++ {
		segmentString, err := getNextLine(scanner)
		if err != nil {
			return nil, err
		}

		var s segment
		var segment_flags string
		_, err = fmt.Sscanf(segmentString, "%s %d %d %s", &s.name, &s.start_address, &s.length, &segment_flags)
		if err != nil {
			return nil, fmt.Errorf("errore nella lettura del segmento %d -> %s: %w", i+1, segmentString, err)
		}
		s.flags, err = parseSegmentFlags(segment_flags)
		if err != nil {
			return nil, err
		}
		obj.segmentTable = append(obj.segmentTable, s)
	}
	fmt.Println("### Segmenti", obj.segmentTable)

	/* parsing dei simboli */
	i = 0
	for ; i < obj.header.symbol_num; i++ {
		symbolString, err := getNextLine(scanner)
		if err != nil {
			return nil, err
		}

		var s symbol
		var kindString string
		_, err = fmt.Sscanf(symbolString, "%s %d %d %s", &s.name, &s.value, &s.segnum, &kindString)
		if err != nil {
			return nil, fmt.Errorf("errore nella lettura del simbolo %d -> %s: %w", i+1, symbolString, err)
		}
		s.kind, err = parseSymbolKind(kindString)
		if err != nil {
			return nil, err
		}
		obj.symbolTable = append(obj.symbolTable, s)
	}
	fmt.Println("### Simboli", obj.symbolTable)

	/* parsing delle relocation entries */
	i = 0
	for ; i < obj.header.relocation_entries_num; i++ {
		relocationString, err := getNextLine(scanner)
		if err != nil {
			return nil, err
		}

		var r relocationEntry
		var kindString string
		_, err = fmt.Sscanf(relocationString, "%d %d %d %s", &r.loc, &r.segnum, &r.ref, &kindString)
		if err != nil {
			return nil, fmt.Errorf("errore nella lettura della relocation entry %d -> %s: %w", i+1, relocationString, err)
		}
		r.kind, err = parseRelocationKind(kindString)
		if err != nil {
			return nil, err
		}
		obj.relocationTable = append(obj.relocationTable, r)
	}
	fmt.Println("### Relocation entries", obj.relocationTable)

	/* dati dei segmenti */
	i = 0
	for ; i < obj.header.segment_num; i++ {
		segmentDataHexString, err := getNextLine(scanner)
		if err != nil {
			return nil, err
		}
		segmentData, err := hex.DecodeString(segmentDataHexString)
		if err != nil {
			return nil, err
		}

		obj.data = append(obj.data, segmentData)
	}
	fmt.Println("### Dati dei segmenti", obj.data)

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
	//header
	_, err = fmt.Fprintf(f, "%d %d %d\n", obj.header.segment_num, obj.header.symbol_num, obj.header.relocation_entries_num)
	if err != nil {
		return err
	}
	// segments
	fmt.Fprintln(f, "# segments")
	for i := 0; i < int(obj.header.segment_num); i++ {
		flags := ""
		for _, f := range obj.segmentTable[i].flags {
			flags += f.String()
		}

		_, err = fmt.Fprintf(f, "%s %d %d %s\n", obj.segmentTable[i].name, obj.segmentTable[i].start_address, obj.segmentTable[i].length, flags)
		if err != nil {
			return err
		}
	}
	// symbols
	fmt.Fprintln(f, "# symbols")
	for i := 0; i < int(obj.header.symbol_num); i++ {
		_, err = fmt.Fprintf(f, "%s %d %d %s\n", obj.symbolTable[i].name, obj.symbolTable[i].value, obj.symbolTable[i].segnum, obj.symbolTable[i].kind.String())
		if err != nil {
			return err
		}
	}
	// relocatins
	fmt.Fprintln(f, "# relocations")
	for i := 0; i < int(obj.header.relocation_entries_num); i++ {
		_, err = fmt.Fprintf(f, "%d %d %d %s\n", obj.relocationTable[i].loc, obj.relocationTable[i].segnum, obj.relocationTable[i].ref, obj.relocationTable[i].kind.String())
		if err != nil {
			return err
		}
	}
	// data
	fmt.Fprintln(f, "# segment data")
	for i := 0; i < int(obj.header.segment_num); i++ {
		_, err = fmt.Fprintln(f, hex.EncodeToString(obj.data[i]))
		if err != nil {
			return err
		}
	}
	return nil
}
