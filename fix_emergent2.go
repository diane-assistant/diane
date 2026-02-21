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

	oldRun := `		run.Output = append(run.Output, Message{
			Role: "assistant",
			Parts: []MessagePart{
				{
					ContentType: "text/plain",
					Content:     "Emergent agent execution triggered successfully. [Workspace Config: " + fmtSprintfWorkspace(agent) + "]",
				},
			},
			CreatedAt:   &now,
			CompletedAt: &now,
		})`

	newRun := `		run.Output = append(run.Output, Message{
			Role: "assistant",
			Parts: []MessagePart{
				{
					ContentType: "text/plain",
					Content:     "Emergent agent execution triggered successfully. [Workspace Config: " + fmtSprintfWorkspace(agent) + "]",
				},
			},
			CreatedAt:   &now,
			CompletedAt: &now,
		})`

	str = strings.Replace(str, oldRun, newRun, 1)

	err = ioutil.WriteFile("server/internal/acp/emergent.go", []byte(str), 0644)
	if err != nil {
		panic(err)
	}
	fmt.Println("Fixed emergent.go 2")
}
