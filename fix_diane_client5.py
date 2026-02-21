import re

with open('Diane/Diane/Services/DianeClient.swift', 'r') as f:
    content = f.read()

add_func = """
    func addAgent(agent: AgentConfig) async throws {
        let bodyData = try JSONEncoder().encode(agent)
        _ = try await request("/agents", method: "POST", body: bodyData)
    }
"""

content = re.sub(
    r'(    func addAgent\(agent: AgentConfig\) async throws \{\n        let args = \["agent", "add", agent.name, agent.url, agent.description \?\? ""\]\n        _ = try await processRequest\(args: args\)\n    \})',
    add_func.strip('\n'),
    content
)

with open('Diane/Diane/Services/DianeClient.swift', 'w') as f:
    f.write(content)
