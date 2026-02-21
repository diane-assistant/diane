package main

import (
	"fmt"
	"io/ioutil"
	"strings"
)

func main() {
	content, err := ioutil.ReadFile("server/internal/acp/emergent.go")
	if err != nil {
		panic(err)
	}
	str := string(content)

	oldImp := `import (
	"crypto/rand"
	"encoding/hex"
	"time"
)`

	newImp := `import (
	"crypto/rand"
	"encoding/hex"
	"time"
)`

	str = strings.Replace(str, oldImp, newImp, 1)

	err = ioutil.WriteFile("server/internal/acp/emergent.go", []byte(str), 0644)
	if err != nil {
		panic(err)
	}
	fmt.Println("Patched emergent.go")
}
