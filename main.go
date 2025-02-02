package main

import (
	"fmt"
	"os"
)

func main() {
	// Pt 1 create recycle bin
	home, home_err := os.UserHomeDir()
	if home_err != nil {
		fmt.Println(home_err)
	}
	fmt.Println(home)
	// NOTE: IDEA: Command to easily retrieve a file in the recycle bin
}
