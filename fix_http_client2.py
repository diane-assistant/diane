import re

with open('Diane/Diane/Services/DianeHTTPClient.swift', 'r') as f:
    content = f.read()

add_func = """
    func addAgent(agent: AgentConfig) async throws {
        let bodyData = try JSONEncoder().encode(agent)
        _ = try await requestWithBody("/agents", method: "POST", body: bodyData)
    }
"""

content = re.sub(
    r'(    // MARK: - Agents\n)',
    r'\1' + add_func,
    content
)

with open('Diane/Diane/Services/DianeHTTPClient.swift', 'w') as f:
    f.write(content)

with open('Diane/Diane/Services/DianeClientProtocol.swift', 'r') as f:
    proto = f.read()

proto = re.sub(
    r'(    // MARK: - Agents\n)',
    r'\1    func addAgent(agent: AgentConfig) async throws\n',
    proto
)
with open('Diane/Diane/Services/DianeClientProtocol.swift', 'w') as f:
    f.write(proto)

