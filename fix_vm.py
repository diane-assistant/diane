import re

with open('Diane/Diane/ViewModels/AgentsViewModel.swift', 'r') as f:
    content = f.read()

# Add to state
add_state = """
    var showGallerySheet = false
    var newAgentURL = ""
    var newAgentDescription = ""
"""
content = re.sub(
    r'(    var showAddAgent = false\n)',
    r'\1' + add_state,
    content
)

# Add custom agent func
add_func = """
    func addCustomAgent() async {
        isInstalling = true
        installError = nil
        
        do {
            let agent = AgentConfig(
                name: newAgentName,
                url: newAgentURL,
                type: "acp",
                command: nil,
                args: nil,
                env: nil,
                workdir: newAgentWorkdir.isEmpty ? nil : newAgentWorkdir,
                port: nil,
                subAgent: nil,
                enabled: true,
                description: newAgentDescription.isEmpty ? nil : newAgentDescription,
                tags: nil
            )
            
            try await client.addAgent(agent: agent)
            
            // Refresh
            agents = try await client.getAgents()
            showAddAgent = false
            
            // Reset
            newAgentName = ""
            newAgentURL = ""
            newAgentDescription = ""
            newAgentWorkdir = ""
        } catch {
            installError = error.localizedDescription
        }
        
        isInstalling = false
    }
"""

content = re.sub(
    r'(    func startEditing\(\) \{\n)',
    add_func + r'\n\1',
    content
)

with open('Diane/Diane/ViewModels/AgentsViewModel.swift', 'w') as f:
    f.write(content)

