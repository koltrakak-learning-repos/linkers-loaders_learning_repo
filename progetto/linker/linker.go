// Package linker definisce il mio linker
package linker

import (
	"encoding/json"
	"fmt"
	obj "koltrakak/my-linker/objectformat"
)

const (
	WORD_SIZE = 8
	PAGE_SIZE = 4096
)

func Link(inputFileNames []string) (*obj.MyObjectFormat, error) {
	var inputObjs []*obj.MyObjectFormat
	for _, f := range inputFileNames {
		o, err := obj.ParseObjectFile(f)
		if err != nil {
			return nil, err
		}
		inputObjs = append(inputObjs, o)
	}

	outputObj, segmentAllocationTable := allocateStorage(inputObjs)

	// pretty print
	pretty, _ := json.MarshalIndent(outputObj, "", "  ")
	fmt.Println("### outputObj")
	fmt.Println(string(pretty))
	pretty, _ = json.MarshalIndent(segmentAllocationTable, "", "  ")
	fmt.Println("### segmentAllocationTable")
	fmt.Println(string(pretty))

	// in alternativa esiste anche questo che mi stampa i miei enumerativi ma è da configurare
	// dato che di base spara fuori troppa roba... non ho voglia
	// "github.com/davecgh/go-spew/spew"
	// spew.Dump(outputObj)
	// spew.Dump(segmentAllocationTable)

	segNumSegNameMap := map[uint]string{}
	for i, s := range outputObj.SegmentTable {
		segNumSegNameMap[uint(i)+1] = s.Name // nei file oggetto i segnum partono da 1
	}

	globalSymbolTable, err := resolveSymbols(inputObjs, segmentAllocationTable, segNumSegNameMap, outputObj)
	if err != nil {
		return nil, err
	}
	pretty, _ = json.MarshalIndent(globalSymbolTable, "", "  ")
	fmt.Println("### globalSymbolTable")
	fmt.Println(string(pretty))

	return outputObj, nil
}

/****** STORAGE ALLOCATION ******/

// la chiave è il nome del file
type SegmentAllocationTableEntry struct {
	InputFilename string
	Segment       obj.Segment
}

// la chiave è il nome del simbolo
type SegmentAllocationTable map[string][]SegmentAllocationTableEntry

func align(x uint, alignment uint) uint {
	return (x + (alignment - 1)) &^ (alignment - 1) // nand mi azzera i LSB
}

func allocateStorage(inputObjs []*obj.MyObjectFormat) (*obj.MyObjectFormat, SegmentAllocationTable) {
	// in questa tabella salvo le informazioni di allocazione di ogni segmento di ogni input file.
	// Nel file di oggetto di output queste informazioni sarebbero perse dato che unifico tutti i segmenti
	// con lo stesso nome in un unico segmentone
	// La chiave è il nome del segmento dato che devo raggruppare le entry con questa logica (vedi sotto)
	segmentAllocationTable := SegmentAllocationTable{}

	outputObj := obj.MyObjectFormat{
		Header: obj.ObjHeader{},
		SegmentTable: []obj.Segment{
			{
				Name:         ".text",
				StartAddress: 0x1000, // text inizia alla seconda pagina dato che la prima è riservata ad heeader
				Length:       0,
				Flags:        []obj.SegmentFlag{obj.Readable},
			},
			// non so ancora quanto sarà grande .text e quindi non so dove far iniziare .data (stesso discorso per .bss)
			{
				Name:         ".data",
				StartAddress: 0x0,
				Length:       0,
				Flags:        []obj.SegmentFlag{obj.Readable, obj.Writable},
			},
			{
				Name:         ".bss",
				StartAddress: 0x0,
				Length:       0,
				Flags:        []obj.SegmentFlag{obj.Readable, obj.Writable},
			},
		},
		SymbolTable:     []obj.Symbol{},
		RelocationTable: []obj.RelocationEntry{},
		Data:            []obj.SegmentData{},
	}

	segmentPointerMap := map[string]*obj.Segment{
		".text": &outputObj.SegmentTable[0],
		".data": &outputObj.SegmentTable[1],
		".bss":  &outputObj.SegmentTable[2],
	}

	// scorro tutti i miei input e calcolo le lunghezze dei segmenti
	for _, io := range inputObjs {
		for _, s := range io.SegmentTable { // go fa automaticamente la dereferenziazione quando accedo ai campi di un puntatore
			// unifico i segmenti che hanno lo stesso nome
			outputSegPointer, ok := segmentPointerMap[s.Name]
			if ok {
				outputSegPointer.Length += s.Length
			} else {
				s.StartAddress = 0x0 // lo dovrò rilocare
				outputObj.SegmentTable = append(outputObj.SegmentTable, s)
				segmentPointerMap[s.Name] = &outputObj.SegmentTable[len(outputObj.SegmentTable)-1]
			}

			// salvo il mio segmento in modo da non perderlo con l'unificazione
			var offset uint
			curSegmentTypeDimInTable := len(segmentAllocationTable[s.Name])
			// inizialmente, per ogni segmento calcolo solamente l'offset all'interno del suo segmentone
			// sotto faccio la rilocazione per ottenere lo StartAddress finale
			if curSegmentTypeDimInTable > 0 {
				prev := segmentAllocationTable[s.Name][curSegmentTypeDimInTable-1].Segment // prendo l'ultimo nella lista
				offset = prev.StartAddress + prev.Length
			} else {
				offset = 0
			}
			segmentAllocationTable[s.Name] = append(segmentAllocationTable[s.Name], SegmentAllocationTableEntry{
				InputFilename: io.Filename,
				Segment: obj.Segment{
					Name:         s.Name,
					StartAddress: offset,
					Length:       s.Length,
					Flags:        s.Flags,
				},
			})
		}
	}

	// aggiusto gli StartAddress (salto .text dato che va già bene)
	// A reasonable allocation strategy would be to put at 1000 the segments with RP attributes,
	// then starting at the next 1000 boundary RWP attributes, then on a 4 boundary RW attributes.
	// ...
	// io me ne frego e carico segmenti diversi in pagine diverse in page boundary distinti
	// FIXME: non caricare tutto a pagine diverse
	for i := 1; i < len(outputObj.SegmentTable); i++ {
		prev := outputObj.SegmentTable[i-1]
		s := &outputObj.SegmentTable[i]
		baseAddress := align(prev.StartAddress+prev.Length, PAGE_SIZE)
		s.StartAddress = baseAddress

		for _, entry := range segmentAllocationTable[s.Name] {
			entry.Segment.StartAddress += baseAddress // aggiungo il baseAddress a tutti i segmentini dentro al segmentone corrente
		}
	}

	return &outputObj, segmentAllocationTable
}

/****** SYMBOL RESOLUTION ******/

type SymbolTableEntry struct {
	FileName string
	Symbol   obj.Symbol
}

// GlobalSymbolTable la chiave è il nome del simbolo
type GlobalSymbolTable map[string]SymbolTableEntry

func resolveSymbols(inputObjs []*obj.MyObjectFormat,
	segmentAllocationTable SegmentAllocationTable,
	segNumSegNameMap map[uint]string,
	outputObj *obj.MyObjectFormat) (GlobalSymbolTable, error) {
	globalSymbolTable := GlobalSymbolTable{}
	unresolvedReferences := map[string][]SymbolTableEntry{}

	for _, o := range inputObjs {
		for _, sym := range o.SymbolTable {
			if sym.Kind == obj.Defined {
				// check if a symbol is defined multiple times
				_, ok := globalSymbolTable[sym.Name]
				if ok {
					return nil, fmt.Errorf("il simbolo %s è stato definito più volte: %s, %s", sym.Name, globalSymbolTable[sym.Name].FileName, o.Filename)
				} else {
					// risolvo il valore del simbolo tenendo conto di dove il suo segmento
					// di definizione (presente in uno dei vari file di input) è stato
					// rilocato nell'output file
					segName, ok := segNumSegNameMap[sym.Segnum]
					if !ok {
						return nil, fmt.Errorf("trovato simbolo definito dentro a un segnum non esistente: %v->%d", sym, sym.Segnum)
					}
					// FIXME: probabilmente con struttura dati più intelligenti non ci sarebbe bisogno di scorrere.
					// Non ho voglia di implementarle
					outSegmentMap := map[string]uint{}
					for _, s := range outputObj.SegmentTable {
						outSegmentMap[s.Name] = s.StartAddress
					}

					for _, s := range segmentAllocationTable[segName] {
						if s.InputFilename == o.Filename {
							fmt.Println(sym.Name, sym.Value, s.Segment.StartAddress, outSegmentMap[s.Segment.Name])
							sym.Value += s.Segment.StartAddress + outSegmentMap[s.Segment.Name]
							break
						}
					}
					// aggiungo il simbolo risolto alla tabella globale
					globalSymbolTable[sym.Name] = SymbolTableEntry{
						FileName: o.Filename,
						Symbol:   sym,
					}
					delete(unresolvedReferences, sym.Name)
				}
			} else {
				unresolvedReferences[sym.Name] = append(unresolvedReferences[sym.Name], SymbolTableEntry{
					FileName: o.Filename,
					Symbol:   sym,
				})
			}

		}
	}

	// check if there are references with no definition
	if len(unresolvedReferences) > 0 {
		errString := ""
		for k, v := range unresolvedReferences {
			for _, r := range v {
				errString += fmt.Sprintf("il simbolo %s all'interno del file %s, non è stato definito\n", k, r.FileName)
			}
		}
		return nil, fmt.Errorf("%s", errString)
	}

	return globalSymbolTable, nil
}
