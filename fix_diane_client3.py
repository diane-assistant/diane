with open('Diane/Diane/Services/DianeClient.swift', 'r') as f:
    content = f.read()

# Just append it before the last brace
add_func = """
    func addAgent(agent: AgentConfig) async throws {
        try await requestWithBody(endpoint: "/agents", method: "POST", body: agent)
    }
}
"""

content = content.rsplit('}', 1)[0] + add_func

with open('Diane/Diane/Services/DianeClient.swift', 'w') as f:
    f.write(content)
