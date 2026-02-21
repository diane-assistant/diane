with open('Diane/Diane/Services/DianeClient.swift', 'r') as f:
    content = f.read()

# Just remove the trailing addAgent that was appended outside the class
import re
content = re.sub(
    r'(    func addAgent\(agent: AgentConfig\) async throws \{\n.*?\n    \}\n)\}\n$',
    r'}',
    content,
    flags=re.DOTALL
)

# And now inject it properly inside the class
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

if "func addAgent(agent: AgentConfig)" not in content:
    content = re.sub(
        r'(    // MARK: - Gallery\n)',
        add_func + r'\n\1',
        content
    )

with open('Diane/Diane/Services/DianeClient.swift', 'w') as f:
    f.write(content)
