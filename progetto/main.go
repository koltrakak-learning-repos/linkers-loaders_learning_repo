package main

import (
	"fmt"
	lnk "koltrakak/my-linker/linker"
	"log"
	"os"
)

func main() {
	if len(os.Args) < 3 {
		log.Fatal("ho bisogno di almeno un file oggetto in input come argomento, e il file di output come ultimo argomento")
	}

	// var o *obj.MyObjectFormat
	// o, err := obj.ParseObjectFile(os.Args[1])
	// if err != nil {
	// 	log.Fatalln(err)
	// }
	// fmt.Println(o)

	outObj, err := lnk.Link(os.Args[1 : len(os.Args)-1])
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println("### output prodotto")
	// fmt.Println(outObj)

	// TODO: aggiungi questo dentro link
	outObj.Filename = os.Args[len(os.Args)-1]

	err = outObj.WriteObjectFile(outObj.Filename)
	if err != nil {
		log.Fatalln(err)
	}
}
