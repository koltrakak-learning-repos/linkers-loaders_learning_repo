package objectFormat

import (
	"bufio"
	"fmt"
	"os"
)

func Read(filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("impossibile aprire file %s: %w", filename, err)
	}
	defer f.Close()

	// It returns false when there are no more tokens, either by reaching the end of the input or an error.
	// After Scan returns false, the Scanner.Err method will return any error that occurred during scanning,
	// except that if it was io.EOF, Scanner.Err will return nil.
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Println(line)
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("errore durante la lettura del file: %w", err)
	}

	return nil
}
