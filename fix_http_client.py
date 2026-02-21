import re

with open('Diane/Diane/Services/DianeHTTPClient.swift', 'r') as f:
    content = f.read()

# Fix testAgent
test_agent = """
    func testAgent(name: String) async throws -> AgentTestResult {
        let encodedName = name.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? name
        let data = try await requestWithBody("/agents/\\(encodedName)/test", method: "POST", body: nil)
        return try decode(AgentTestResult.self, from: data)
    }
"""
content = re.sub(
    r'    func testAgent\(name: String\) async throws -> AgentTestResult \{\n        throw DianeHTTPClientError\.readOnlyMode\n    \}',
    test_agent.strip('\n'),
    content
)

# Fix toggleAgent
toggle_agent = """
    func toggleAgent(name: String, enabled: Bool) async throws {
        let encodedName = name.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? name
        let body = ["enabled": enabled]
        let bodyData = try JSONEncoder().encode(body)
        _ = try await requestWithBody("/agents/\\(encodedName)/toggle", method: "POST", body: bodyData)
    }
"""
content = re.sub(
    r'    func toggleAgent\(name: String, enabled: Bool\) async throws \{\n        throw DianeHTTPClientError\.readOnlyMode\n    \}',
    toggle_agent.strip('\n'),
    content
)

# Fix runAgentPrompt
run_agent = """
    func runAgentPrompt(agentName: String, prompt: String, remoteAgentName: String?) async throws -> AgentRunResult {
        let encodedName = agentName.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? agentName
        var body: [String: String] = ["prompt": prompt]
        if let remoteAgent = remoteAgentName {
            body["agent_name"] = remoteAgent
        }
        let bodyData = try JSONEncoder().encode(body)
        let data = try await requestWithBody("/agents/\\(encodedName)/run", method: "POST", body: bodyData)
        return try decodeGo(AgentRunResult.self, from: data)
    }
"""
content = re.sub(
    r'    func runAgentPrompt\(agentName: String, prompt: String, remoteAgentName: String\?\) async throws -> AgentRunResult \{\n        throw DianeHTTPClientError\.readOnlyMode\n    \}',
    run_agent.strip('\n'),
    content
)

# Fix removeAgent
remove_agent = """
    func removeAgent(name: String) async throws {
        let encodedName = name.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? name
        _ = try await requestWithBody("/agents/\\(encodedName)", method: "DELETE")
    }
"""
content = re.sub(
    r'    func removeAgent\(name: String\) async throws \{\n        throw DianeHTTPClientError\.readOnlyMode\n    \}',
    remove_agent.strip('\n'),
    content
)

# Fix updateAgent
update_agent = """
    func updateAgent(name: String, subAgent: String?, enabled: Bool?, description: String?, workdir: String?) async throws {
        let encodedName = name.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? name
        var body: [String: Any] = [:]
        if let subAgent = subAgent {
            body["sub_agent"] = subAgent
        }
        if let enabled = enabled {
            body["enabled"] = enabled
        }
        if let description = description {
            body["description"] = description
        }
        if let workdir = workdir {
            body["workdir"] = workdir
        }
        
        guard !body.isEmpty else { return }
        let bodyData = try JSONSerialization.data(withJSONObject: body)
        _ = try await requestWithBody("/agents/\\(encodedName)/update", method: "POST", body: bodyData)
    }
"""
content = re.sub(
    r'    func updateAgent\(name: String, subAgent: String\?, enabled: Bool\?, description: String\?, workdir: String\?\) async throws \{\n        throw DianeHTTPClientError\.readOnlyMode\n    \}',
    update_agent.strip('\n'),
    content
)

with open('Diane/Diane/Services/DianeHTTPClient.swift', 'w') as f:
    f.write(content)

