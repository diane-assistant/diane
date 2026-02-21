import re

with open('Diane/Diane/ViewModels/AgentsViewModel.swift', 'r') as f:
    content = f.read()

# Add edit state
edit_state = """
    // MARK: - Edit Agent State
    var showEditAgent = false
    var editAgentDescription = ""
    var editAgentWorkdir = ""
    var isEditing = false
    var editError: String?
"""
content = re.sub(
    r'(    var installError: String\?\n)',
    r'\1\n' + edit_state,
    content
)

# Add edit functions
edit_funcs = """
    func startEditing() {
        guard let agent = selectedAgent else { return }
        editAgentDescription = agent.description ?? ""
        editAgentWorkdir = agent.workdir ?? ""
        editError = nil
        showEditAgent = true
    }

    func saveEdit() async {
        guard let agent = selectedAgent else { return }
        
        isEditing = true
        editError = nil
        FileLogger.shared.info("Updating agent '\\(agent.name)'", category: "Agents")
        
        do {
            let newDesc = editAgentDescription.isEmpty ? nil : editAgentDescription
            let newWorkdir = editAgentWorkdir.isEmpty ? nil : editAgentWorkdir
            
            try await client.updateAgent(
                name: agent.name,
                subAgent: nil,
                enabled: nil,
                description: newDesc,
                workdir: newWorkdir
            )
            
            // Refresh
            agents = try await client.getAgents()
            if let updated = agents.first(where: { $0.name == agent.name }) {
                selectedAgent = updated
            }
            showEditAgent = false
        } catch {
            editError = error.localizedDescription
            FileLogger.shared.error("Failed to update agent '\\(agent.name)': \\(error.localizedDescription)", category: "Agents")
        }
        
        isEditing = false
    }
"""

content = re.sub(
    r'(    // MARK: - Helpers\n)',
    edit_funcs + r'\n\1',
    content
)

with open('Diane/Diane/ViewModels/AgentsViewModel.swift', 'w') as f:
    f.write(content)

