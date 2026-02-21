import re

with open('Diane/Diane/Services/DianeClient.swift', 'r') as f:
    content = f.read()

add_func = """
    func addAgent(agent: AgentConfig) async throws {
        try await requestWithBody(endpoint: "/agents", method: "POST", body: agent)
    }
"""

content = re.sub(
    r'(    // MARK: - Agents\n)',
    r'\1' + add_func,
    content
)

with open('Diane/Diane/Services/DianeClient.swift', 'w') as f:
    f.write(content)

