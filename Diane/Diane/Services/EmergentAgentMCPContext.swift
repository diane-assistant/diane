import Foundation

// 4.1 Update MCP server context loading to pass selected tool capabilities into the agent configuration payload
class EmergentAgentMCPContext {
    static func generatePayload(for tools: [String]) -> [String: Any] {
        // Map available MCP server tools to the agent's payload
        let mappedTools = tools.map { ["name": $0, "description": "Auto-mapped tool \($0)"] }
        return [
            "mcp_context": [
                "tools": mappedTools,
                "environment": "diane-emergent"
            ]
        ]
    }
}
