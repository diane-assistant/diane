import re

with open('Diane/Diane/Services/DianeClient.swift', 'r') as f:
    content = f.read()

# request method doesn't exist on local DianeClient either. We must use processRequest.
# `processRequest(args: [String])` runs the CLI tool. 
# Oh wait, the local `diane agent add` command only takes: name url description. It doesn't take workdir or other fields yet!
add_func = """
    func addAgent(agent: AgentConfig) async throws {
        // Local CLI only supports basic add right now: name url [description]
        var args = ["agent", "add", agent.name, agent.url]
        if let desc = agent.description, !desc.isEmpty {
            args.append(desc)
        }
        _ = try await processRequest(args: args)
    }
"""

content = re.sub(
    r'(    func addAgent\(agent: AgentConfig\) async throws \{\n        let bodyData = try JSONEncoder\(\)\.encode\(agent\)\n        _ = try await request\("/agents", method: "POST", body: bodyData\)\n    \})',
    add_func.strip('\n'),
    content
)

with open('Diane/Diane/Services/DianeClient.swift', 'w') as f:
    f.write(content)
