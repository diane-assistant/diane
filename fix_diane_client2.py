import re

with open('Diane/Diane/Services/DianeClient.swift', 'r') as f:
    content = f.read()

# I realize the previous sed probably missed because I didn't verify it was inserted correctly in DianeClient.swift
add_func = """
    func addAgent(agent: AgentConfig) async throws {
        try await requestWithBody(endpoint: "/agents", method: "POST", body: agent)
    }
"""

if "func addAgent(agent: AgentConfig)" not in content:
    content = re.sub(
        r'(    // MARK: - Gallery\n)',
        add_func + r'\n\1',
        content
    )

with open('Diane/Diane/Services/DianeClient.swift', 'w') as f:
    f.write(content)

