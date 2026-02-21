import re

with open('Diane/Diane/Services/DianeClient.swift', 'r') as f:
    content = f.read()

# requestWithBody is on DianeHTTPClient, not DianeClient.
# In DianeClient we use processRequest
add_func = """
    func addAgent(agent: AgentConfig) async throws {
        let args = ["agent", "add", agent.name, agent.url, agent.description ?? ""]
        _ = try await processRequest(args: args)
    }
"""

content = re.sub(
    r'(    func addAgent\(agent: AgentConfig\) async throws \{\n        try await requestWithBody\(endpoint: "/agents", method: "POST", body: agent\)\n    \})',
    add_func.strip('\n'),
    content
)

with open('Diane/Diane/Services/DianeClient.swift', 'w') as f:
    f.write(content)
