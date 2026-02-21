with open('Diane/Diane/Services/DianeClient.swift', 'r') as f:
    content = f.read()

# Make double sure we inject it inside the class
add_func = """
    func addAgent(agent: AgentConfig) async throws {
        var args = ["agent", "add", agent.name, agent.url]
        if let desc = agent.description, !desc.isEmpty {
            args.append(desc)
        }
        _ = try await processRequest(args: args)
    }
"""

if "func addAgent(agent: AgentConfig)" not in content:
    content = content.replace("    // MARK: - Gallery\n", add_func + "\n    // MARK: - Gallery\n")

with open('Diane/Diane/Services/DianeClient.swift', 'w') as f:
    f.write(content)
