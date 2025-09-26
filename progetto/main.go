package main

import (
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

	err = outObj.WriteObjectFile(os.Args[len(os.Args)])
	if err != nil {
		log.Fatalln(err)
	}
}
