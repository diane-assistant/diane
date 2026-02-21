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

	oldRun := `	// Handle ACP agents (proper ACP protocol over stdio)
	if agent.Type == "acp" || agent.Type == "" {
		return m.runACPAgent(agent, prompt)
	}`

	newRun := `	// If agent has no URL, it implies an emergent agent via internal execution
	if agent.URL == "" {
		return m.runEmergentAgent(agent, prompt)
	}

	// Handle ACP agents (proper ACP protocol over stdio)
	if agent.Type == "acp" || agent.Type == "" {
		return m.runACPAgent(agent, prompt)
	}`

	str = strings.Replace(str, oldRun, newRun, 1)

	oldTest := `	// Handle ACP agents (proper ACP protocol over stdio)
	if agent.Type == "acp" || agent.Type == "" {
		return m.testACPAgent(agent, result)
	}`

	newTest := `	// If agent has no URL, it implies an emergent agent via internal execution
	if agent.URL == "" {
		return m.testEmergentAgent(agent, result)
	}

	// Handle ACP agents (proper ACP protocol over stdio)
	if agent.Type == "acp" || agent.Type == "" {
		return m.testACPAgent(agent, result)
	}`

	str = strings.Replace(str, oldTest, newTest, 1)

	err = ioutil.WriteFile("server/internal/acp/config.go", []byte(str), 0644)
	if err != nil {
		panic(err)
	}
	fmt.Println("Patched acp/config.go with emergent branching")
}
