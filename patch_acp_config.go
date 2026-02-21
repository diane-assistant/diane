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

	oldStruct := `	Enabled     bool              ` + "`json:\"enabled\"`\n" +
		`	Description string            ` + "`json:\"description,omitempty\"`\n" +
		`	Tags        []string          ` + "`json:\"tags,omitempty\"`\n" +
		`}`

	newStruct := `	Enabled         bool                   ` + "`json:\"enabled\"`\n" +
		`	Description     string                 ` + "`json:\"description,omitempty\"`\n" +
		`	Tags            []string               ` + "`json:\"tags,omitempty\"`\n" +
		`	WorkspaceConfig *store.WorkspaceConfig ` + "`json:\"workspace_config,omitempty\"`\n" +
		`}`

	str = strings.Replace(str, oldStruct, newStruct, 1)

	oldToStore := `	return store.ACPAgentConfig{
		Name:        c.Name,
		URL:         c.URL,
		Type:        c.Type,
		Command:     c.Command,
		Args:        c.Args,
		Env:         c.Env,
		WorkDir:     c.WorkDir,
		Port:        c.Port,
		SubAgent:    c.SubAgent,
		Enabled:     c.Enabled,
		Description: c.Description,
		Tags:        c.Tags,
	}`

	newToStore := `	return store.ACPAgentConfig{
		Name:            c.Name,
		URL:             c.URL,
		Type:            c.Type,
		Command:         c.Command,
		Args:            c.Args,
		Env:             c.Env,
		WorkDir:         c.WorkDir,
		Port:            c.Port,
		SubAgent:        c.SubAgent,
		Enabled:         c.Enabled,
		Description:     c.Description,
		Tags:            c.Tags,
		WorkspaceConfig: c.WorkspaceConfig,
	}`

	str = strings.Replace(str, oldToStore, newToStore, 1)

	oldFromStore := `	return AgentConfig{
		Name:        s.Name,
		URL:         s.URL,
		Type:        s.Type,
		Command:     s.Command,
		Args:        s.Args,
		Env:         s.Env,
		WorkDir:     s.WorkDir,
		Port:        s.Port,
		SubAgent:    s.SubAgent,
		Enabled:     s.Enabled,
		Description: s.Description,
		Tags:        s.Tags,
	}`

	newFromStore := `	return AgentConfig{
		Name:            s.Name,
		URL:             s.URL,
		Type:            s.Type,
		Command:         s.Command,
		Args:            s.Args,
		Env:             s.Env,
		WorkDir:         s.WorkDir,
		Port:            s.Port,
		SubAgent:        s.SubAgent,
		Enabled:         s.Enabled,
		Description:     s.Description,
		Tags:            s.Tags,
		WorkspaceConfig: s.WorkspaceConfig,
	}`

	str = strings.Replace(str, oldFromStore, newFromStore, 1)

	err = ioutil.WriteFile("server/internal/acp/config.go", []byte(str), 0644)
	if err != nil {
		panic(err)
	}
	fmt.Println("Patched acp/config.go")
}
