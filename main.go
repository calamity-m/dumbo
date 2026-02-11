package main

import (
	"fmt"

	"github.com/calamity-m/dumbo/dumbo"
)

func main() {
	err := dumbo.Execute()
	if err != nil {
		fmt.Println(err)
	}
}
