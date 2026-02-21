import re

with open('Diane/Diane/Services/DianeClient.swift', 'r') as f:
    content = f.read()

# Replace the method that causes the error
add_func = """
    func addAgent(agent: AgentConfig) async throws {
        let bodyData = try JSONEncoder().encode(agent)
        _ = try await request("/agents", method: "POST", timeout: 10, body: bodyData)
    }
"""

# Find the old processRequest implementation
content = re.sub(
    r'(    func addAgent\(agent: AgentConfig\) async throws \{\n        // Local CLI only supports basic add right now: name url \[description\]\n        var args = \["agent", "add", agent\.name, agent\.url\]\n        if let desc = agent\.description, !desc\.isEmpty \{\n            args\.append\(desc\)\n        \}\n        _ = try await processRequest\(args: args\)\n    \})',
    add_func.strip('\n'),
    content
)

with open('Diane/Diane/Services/DianeClient.swift', 'w') as f:
    f.write(content)
