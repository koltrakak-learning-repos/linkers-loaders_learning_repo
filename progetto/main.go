package main

import (
	"fmt"
	obj "koltrakak/my-linker/myObjectFormat"
	"log"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("ho bisogno di almeno un file oggetto in input come argomento")
	}

	var o *obj.MyObjectFormat
	o, err := obj.ParseObjectFile(os.Args[1])
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println(o)

	err = o.WriteObjectFile("output.myo")
	if err != nil {
		log.Fatalln(err)
	}
}
