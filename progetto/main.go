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

	obj.Read(os.Args[1])
}
