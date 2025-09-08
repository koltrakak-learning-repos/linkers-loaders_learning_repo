per compilare senza linkare: gcc -c es.c -o es.o
- questo produce codice macchina non linkato

per fare disassembly
- objdump -h es.o
  - mostra le sezioni del binario
- objdump -d es.o
  - mostra l'assembly

per mostrare la symbol table
- nm es.o
- ma anche objdump -t
