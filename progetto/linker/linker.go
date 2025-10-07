// Package linker definisce il mio linker
package linker

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	obj "koltrakak/my-linker/objectformat"
)

const (
	WORD_SIZE = 8
	PAGE_SIZE = 4096
)

func Link(inputFileNames []string) (*obj.MyObjectFormat, error) {
	// parse input objects
	var inputObjs []*obj.MyObjectFormat
	for _, f := range inputFileNames {
		o, err := obj.ParseObjectFile(f)
		if err != nil {
			return nil, err
		}
		inputObjs = append(inputObjs, o)
	}

	// allocate storage in output object
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

	// resolve Symbols
	segNumSegNameMap := map[uint]string{}
	for i, s := range outputObj.SegmentTable {
		segNumSegNameMap[uint(i)+1] = s.Name // nei file oggetto i segnum partono da 1
	}
	globalSymbolTable, err := resolveSymbols(inputObjs, segmentAllocationTable, segNumSegNameMap)
	if err != nil {
		return nil, err
	}
	pretty, _ = json.MarshalIndent(globalSymbolTable, "", "  ")
	fmt.Println("### globalSymbolTable")
	fmt.Println(string(pretty))

	// apply fixups
	applyFixups(inputObjs, globalSymbolTable, segmentAllocationTable, segNumSegNameMap)

	// write fixed data segments
	writeFixedData(inputObjs, outputObj)

	return outputObj, nil
}

/****** STORAGE ALLOCATION ******/

// In questa tabella salvo le informazioni di allocazione di ogni segmento di ogni input file.
// Nel file di oggetto di output queste informazioni sarebbero perse dato che unifico tutti i segmenti
// con lo stesso nome in un unico segmentone
// La chiave è multipla: nome del segmento + nome del file
type SegmentAllocationTable map[string]map[string]*obj.Segment

func align(x uint, alignment uint) uint {
	return (x + (alignment - 1)) &^ (alignment - 1) // nand mi azzera i LSB
}

func allocateStorage(inputObjs []*obj.MyObjectFormat) (*obj.MyObjectFormat, SegmentAllocationTable) {
	// Questa è una struttura dati di appoggio che uso per calcolare
	// correttamente gli offset dei segmentini con lo stesso nome nei
	// vari file di input, dentro al segmentone corrispondente nel
	// file di output.
	// La chiave è il nome del segmento
	segmentUnificationTable := map[string][]*obj.Segment{}
	segmentAllocationTable := SegmentAllocationTable{}

	outputObj := obj.MyObjectFormat{
		Header: obj.ObjHeader{},
		// sicuramente ho questi segmenti nel mio file di output
		SegmentTable: []*obj.Segment{
			{
				Name:         ".text",
				StartAddress: 0x1000, // text inizia alla seconda pagina dato che la prima è riservata ad header
				Length:       0,
				Flags: map[obj.SegmentFlag]bool{
					obj.Readable: true,
					obj.Present:  true,
				},
			},
			// non so ancora quanto sarà grande .text e quindi non so dove far iniziare .data (stesso discorso per .bss)
			{
				Name:         ".data",
				StartAddress: 0x0,
				Length:       0,
				Flags: map[obj.SegmentFlag]bool{
					obj.Readable: true,
					obj.Writable: true,
					obj.Present:  true,
				},
			},
			{
				Name:         ".bss",
				StartAddress: 0x0,
				Length:       0,
				Flags: map[obj.SegmentFlag]bool{
					obj.Readable: true,
					obj.Writable: true,
				},
			},
		},
		SymbolTable:     []*obj.Symbol{},         // questa probabilmente sarà vuota
		RelocationTable: []obj.RelocationEntry{}, // anche questa
		Data:            []obj.SegmentData{},
	}

	// mappa di supporto per non dover scorrere la tabella linearmente
	outputSegmentPointerMap := map[string]*obj.Segment{
		".text": outputObj.SegmentTable[0],
		".data": outputObj.SegmentTable[1],
		".bss":  outputObj.SegmentTable[2],
	}

	// scorro tutti i miei input e calcolo le lunghezze dei segmenti
	for _, io := range inputObjs {
		for _, seg := range io.SegmentTable { // go fa automaticamente la dereferenziazione quando accedo ai campi di un puntatore
			// popolo i segmenti del file di output unificando segmenti
			// con lo stesso nome
			outputSegPointer, ok := outputSegmentPointerMap[seg.Name]
			if ok {
				outputSegPointer.Length += seg.Length
			} else {
				seg.StartAddress = 0x0 // lo dovrò rilocare
				// aggiungo il segmento di tipo ignoto sia alla mappa di supporto
				// che alla tabella del file di output (siccome è un puntatore le
				// modifiche come quella di sopra sono visibili ad entrambi)
				outputSegmentPointerMap[seg.Name] = seg
				outputObj.SegmentTable = append(outputObj.SegmentTable, seg)
			}
			// salvo il mio segmento in modo da non perderlo con l'unificazione
			var curSegOffset uint
			numUnifiedCurSegmentType := len(segmentUnificationTable[seg.Name]) // len restituisce 0 se lo slice è nil
			// Inizialmente, per ogni segmento calcolo solamente l'offset all'interno del suo segmentone.
			// Sotto faccio la rilocazione per ottenere lo StartAddress finale nel file di output
			if numUnifiedCurSegmentType > 0 {
				prev := segmentUnificationTable[seg.Name][numUnifiedCurSegmentType-1] // prendo l'ultimo che ho aggiunto
				curSegOffset = prev.StartAddress + prev.Length
			} else {
				curSegOffset = 0
			}
			seg.StartAddress = curSegOffset
			// qua salvo seg per poter calcolare l'offset del prossimo segmento dello stesso tipo
			segmentUnificationTable[seg.Name] = append(segmentUnificationTable[seg.Name], seg)
			// qua salvo seg per non perdere le informazioni sui vari segmentini nel file di output finale
			_, ok = segmentAllocationTable[seg.Name]
			if !ok {
				// alloco la sottomappa che ha come chiave il nome del file se necessario
				segmentAllocationTable[seg.Name] = make(map[string]*obj.Segment)
			}
			segmentAllocationTable[seg.Name][io.Filename] = seg
			// HO SALVATO DEI PUNTATORI! Modifiche a segmenti in segmentUnificationTable
			// saranno visibili anche in segmentAllocationTable
		}
	}

	// non scordiamoci di aggiornare l'header ora che sappiamo quanti segmenti ha
	// il file di output
	outputObj.Header.SegmentNum = uint(len(outputObj.SegmentTable))
	// Aggiusto gli StartAddress
	// sia dei segmentoni nel file di output,
	// che dei segmentini nella segmentAllocationTable
	var prevSeg *obj.Segment = nil
	for _, outSeg := range outputObj.SegmentTable {
		// "A reasonable allocation strategy would be to put at 1000 the segments with RP attributes,
		// then starting at the next 1000 boundary RWP attributes, then on a 4 boundary RW attributes."
		// ...
		// io me ne frego e carico segmenti diversi in pagine diverse in page boundary distinti
		// FIXME: non caricare tutto a pagine diverse
		var baseAddress uint
		if prevSeg == nil {
			// Il primo segmento non ha un precedente. Il base address dei
			// segmentini è quindi uguale allo startAddress del segmentone
			baseAddress = align(outSeg.StartAddress, PAGE_SIZE)
		} else {
			baseAddress = align(prevSeg.StartAddress+prevSeg.Length, PAGE_SIZE)
		}
		outSeg.StartAddress = baseAddress

		// aggiungo il baseAddress a tutti i segmentini dentro al segmentone corrente
		for _, segmentino := range segmentUnificationTable[outSeg.Name] {
			// NB: qua sto modificando anche la segmentAllocationTable
			// dato che punta alla stessa struct
			segmentino.StartAddress += baseAddress
		}
		prevSeg = outSeg
	}

	// infine, ora che so quanta memoria occupa ogni segmento
	// posso allocare il vettore dei dati
	for _, seg := range outputObj.SegmentTable {
		if seg.Flags[obj.Present] {
			outputObj.Data = append(outputObj.Data, make(obj.SegmentData, seg.Length))
			// i dati ce li copio dopo che ho applicato i fixup
		}
	}

	return &outputObj, segmentAllocationTable
}

/****** SYMBOL RESOLUTION ******/

type SymbolTableEntry struct {
	FileName string
	Symbol   *obj.Symbol
}

// GlobalSymbolTable la chiave è il nome del simbolo
type GlobalSymbolTable map[string]SymbolTableEntry

func resolveSymbols(inputObjs []*obj.MyObjectFormat,
	segmentAllocationTable SegmentAllocationTable,
	segNumSegNameMap map[uint]string) (GlobalSymbolTable, error) {

	globalSymbolTable := GlobalSymbolTable{}
	unresolvedReferences := map[string][]SymbolTableEntry{}

	// scorro le symbol table di tutti i miei oggetti
	for _, io := range inputObjs {
		for _, sym := range io.SymbolTable {
			if sym.Kind == obj.Defined {
				// check if a symbol is defined multiple times
				_, ok := globalSymbolTable[sym.Name]
				if ok {
					return nil, fmt.Errorf("il simbolo %s è stato definito più volte: %s, %s", sym.Name, globalSymbolTable[sym.Name].FileName, io.Filename)
				} else {
					// risolvo il valore del simbolo tenendo conto di dove il suo segmento di definizione
					// (presente in uno dei vari file di input) è stato rilocato nell'output file
					segName, ok := segNumSegNameMap[sym.Segnum]
					if !ok {
						return nil, fmt.Errorf("trovato simbolo definito dentro a un segnum non esistente: %v->%d", sym, sym.Segnum)
					}
					segBaseAddress := segmentAllocationTable[segName][io.Filename].StartAddress
					// DEBUG:
					fmt.Println("symbol:", sym.Name)
					fmt.Println("	segment-relative value:", sym.Value)
					fmt.Println("	input segment base address:", segBaseAddress)
					sym.Value += segBaseAddress

					// aggiungo il simbolo risolto alla tabella globale
					globalSymbolTable[sym.Name] = SymbolTableEntry{
						FileName: io.Filename,
						Symbol:   sym,
					}
					delete(unresolvedReferences, sym.Name)
				}
			} else {
				unresolvedReferences[sym.Name] = append(unresolvedReferences[sym.Name], SymbolTableEntry{
					FileName: io.Filename,
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

/****** FIXUP APPLICATION ******/

// per semplificarmi la vita, i segmenti vengono trattati come simboli e sono presenti nella symbol table.
// "this makes segment relative relocation a special case of symbol relative one"

// TODO: questo è altamente parallelizzabile dato che tutti i fixup sono indipendenti
func applyFixups(inputObjs []*obj.MyObjectFormat,
	globalSymbolTable GlobalSymbolTable,
	segmentAllocationTable SegmentAllocationTable,
	segNumSegNameMap map[uint]string) (*obj.MyObjectFormat, error) {

	// scorro tutte le relocation entry di tutti gli input file
	for _, io := range inputObjs {
		for _, re := range io.RelocationTable {
			var relocationValue uint
			fixupLocationValue := io.Data[re.Segnum-1][re.Loc : re.Loc+4] // devo togliere uno dati che i segnum partono da 1
			symbolName := io.SymbolTable[re.Ref-1].Name                   // devo togliere uno dato che i symbolnum partono da 1
			symbol := globalSymbolTable[symbolName].Symbol
			defined := io.SymbolTable[re.Ref-1].Kind == obj.Defined // devo togliere uno dato che i symbolnum partono da 1
			segOfSymbol := segNumSegNameMap[symbol.Segnum]
			// Devo applicare i fixup considerando 3 variabili:
			// - location della relocation entry e simbolo (defined) con cui la
			//   risolvo, sono nello stesso segmento?
			// - tipo della relocation entry (assoluta, relativa, ...)
			// - il simbolo con cui risolvo la relocation entry è definito o no?
			// non ho voglia di spiegare come queste informazioni vanno utilizzate
			// (futuro me non ti arrabbiare)
			switch re.Kind {
			case obj.Absolute4:
				if defined {
					relocationValue = segmentAllocationTable[segOfSymbol][io.Filename].StartAddress
				} else {
					// per simboli non definiti il valore nella location è zero,
					// sommo quindi il valore finale del simbolo
					relocationValue = globalSymbolTable[symbolName].Symbol.Value
				}

			case obj.Relative4:
				segOfFixup := segNumSegNameMap[re.Segnum]
				fixupOutBaseAddress := segmentAllocationTable[segOfFixup][io.Filename].StartAddress
				fixupOutLocation := re.Loc + fixupOutBaseAddress

				if defined {
					if segOfFixup == segOfSymbol {
						// non devo fare niente, l'offset continua ad essere corretto
					} else {
						symbolOutBaseAddress := segmentAllocationTable[segOfSymbol][io.Filename].StartAddress
						// aggiungo di quanto si è spostato il mio target,
						// tolgo di quanto mi sono spostato io
						relocationValue = symbolOutBaseAddress - fixupOutBaseAddress
					}
				} else {
					// se il riferimento è relativo devo saltare della differenza tra le due posizioni
					relocationValue = globalSymbolTable[symbolName].Symbol.Value - fixupOutLocation
				}

			default:
				return nil, fmt.Errorf("trovata relocation entry di tipo non supportato: %s", re.Kind)
			}

			fmt.Println("### fixup applied")
			val := binary.BigEndian.Uint32(fixupLocationValue)
			fmt.Printf("%x + %x\n", fixupLocationValue, relocationValue)
			val += uint32(relocationValue)
			binary.BigEndian.PutUint32(fixupLocationValue, val)
			fmt.Printf("%x\n", fixupLocationValue)
		}
	}

	return nil, nil
}

func writeFixedData(inputObjs []*obj.MyObjectFormat, outputObj *obj.MyObjectFormat) {
	for _, io := range inputObjs {
		// FIXME: qua sto assumendo che l'ordine dei segmenti sia lo stesso
		// sia tra gli inputfile che nell'outputfile. Questo non necessariamente
		// è vero. Un approccio migliore sarebbe stato usare una mappa anche per
		// i segmenti dati
		for i, dataSeg := range io.Data {
			outputObj.Data[i] = append(outputObj.Data[i], dataSeg...)
		}
	}
}
