import re

with open('Diane/Diane/Services/DianeClient.swift', 'r') as f:
    content = f.read()

# Replace the method that causes the error
add_func = """
    func addAgent(agent: AgentConfig) async throws {
        // Local CLI only supports basic add right now: name url [description]
        var args = ["agent", "add", agent.name, agent.url]
        if let desc = agent.description, !desc.isEmpty {
            args.append(desc)
        }
        _ = try await processRequest(args: args)
    }
}
"""

content = re.sub(
    r'(    func addAgent\(agent: AgentConfig\) async throws \{\n        let bodyData = try JSONEncoder\(\)\.encode\(agent\)\n        _ = try await request\("/agents", method: "POST", timeout: 10, body: bodyData\)\n    \}\n\})',
    add_func.strip('\n') + '\n',
    content
)

with open('Diane/Diane/Services/DianeClient.swift', 'w') as f:
    f.write(content)
