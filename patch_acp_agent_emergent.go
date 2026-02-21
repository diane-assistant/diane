package main

import (
	"fmt"
	"io/ioutil"
	"strings"
)

func main() {
	content, err := ioutil.ReadFile("server/internal/store/acp_agent_emergent.go")
	if err != nil {
		panic(err)
	}

	str := string(content)

	unmarshalOld := `	if tagsData, ok := props["tags"].(string); ok && tagsData != "" {
		_ = json.Unmarshal([]byte(tagsData), &agent.Tags)
	}

	return agent, nil`

	unmarshalNew := `	if tagsData, ok := props["tags"].(string); ok && tagsData != "" {
		_ = json.Unmarshal([]byte(tagsData), &agent.Tags)
	}

	if wcData, ok := props["workspace_config"].(string); ok && wcData != "" {
		var wc WorkspaceConfig
		if err := json.Unmarshal([]byte(wcData), &wc); err == nil {
			agent.WorkspaceConfig = &wc
		}
	}

	return agent, nil`

	str = strings.Replace(str, unmarshalOld, unmarshalNew, 1)

	marshalOld := `	if len(agent.Tags) > 0 {
		if b, err := json.Marshal(agent.Tags); err == nil {
			props["tags"] = string(b)
		}
	} else {
		props["tags"] = "[]"
	}

	return props`

	marshalNew := `	if len(agent.Tags) > 0 {
		if b, err := json.Marshal(agent.Tags); err == nil {
			props["tags"] = string(b)
		}
	} else {
		props["tags"] = "[]"
	}

	if agent.WorkspaceConfig != nil {
		if b, err := json.Marshal(agent.WorkspaceConfig); err == nil {
			props["workspace_config"] = string(b)
		}
	}

	return props`

	str = strings.Replace(str, marshalOld, marshalNew, 1)

	err = ioutil.WriteFile("server/internal/store/acp_agent_emergent.go", []byte(str), 0644)
	if err != nil {
		panic(err)
	}
	fmt.Println("Patched successfully")
}
