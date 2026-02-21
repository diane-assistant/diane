import re

with open('Diane/DianeTests/TestHelpers/MockDianeClient.swift', 'r') as f:
    content = f.read()

add_func = """
    var mockAddAgent: (() throws -> Void)?
    func addAgent(agent: AgentConfig) async throws {
        if let mock = mockAddAgent {
            try mock()
        }
    }
"""

content = re.sub(
    r'(    // MARK: - Agents\n)',
    r'\1' + add_func,
    content
)

with open('Diane/DianeTests/TestHelpers/MockDianeClient.swift', 'w') as f:
    f.write(content)

