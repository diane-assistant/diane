package main

import (
	"fmt"
	"io/ioutil"
	"strings"
)

func main() {
	content, err := ioutil.ReadFile("server/internal/acp/config.go")
	if err != nil {
		panic(err)
	}
	str := string(content)

	oldUrl := `	Name        string            ` + "`json:\"name\"`\n" +
		`	URL         string            ` + "`json:\"url\"`\n"

	newUrl := `	Name        string            ` + "`json:\"name\"`\n" +
		`	URL         string            ` + "`json:\"url,omitempty\"`\n"

	str = strings.Replace(str, oldUrl, newUrl, 1)

	err = ioutil.WriteFile("server/internal/acp/config.go", []byte(str), 0644)
	if err != nil {
		panic(err)
	}
	fmt.Println("Patched acp/config.go URL to omitempty")
}
