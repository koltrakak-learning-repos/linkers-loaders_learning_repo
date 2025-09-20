package main

import (
	obj "koltrakak/my-linker/myObjectFormat"
	"log"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("ho bisogno di almeno un file oggetto in input come argomento")
	}

	var o obj.MyObjectFormat
	err := o.ParseObjectFile(os.Args[1])
	if err != nil {
		log.Fatalln(err)
	}
}
