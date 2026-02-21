import Foundation

func main() {
    let path = "Diane/Diane/ViewModels/AgentsViewModel.swift"
    let content = try! String(contentsOfFile: path)
    
    let oldProperties = """
    var newAgentURL = ""
    var newAgentDescription = ""
    var newAgentWorkdir = ""
    var newAgentCommand = ""
    var newAgentArgs = ""
"""
    
    let newProperties = """
    var newAgentURL = ""
    var newAgentDescription = ""
    var newAgentWorkdir = ""
    var newAgentCommand = ""
    var newAgentArgs = ""
    
    // Workspace Config State
    var newAgentBaseImage = ""
    var newAgentRepoURL = ""
    var newAgentRepoBranch = ""
    var newAgentProvider = ""
    var newAgentSetupCommands = [String]()
"""
    
    var updated = content.replacingOccurrences(of: oldProperties, with: newProperties)
    
    let oldReset = """
    func resetAddForm() {
        newAgentURL = ""
        newAgentDescription = ""
        newAgentWorkdir = ""
        newAgentCommand = ""
        newAgentArgs = ""
    }
"""
    
    let newReset = """
    func resetAddForm() {
        newAgentURL = ""
        newAgentDescription = ""
        newAgentWorkdir = ""
        newAgentCommand = ""
        newAgentArgs = ""
        newAgentBaseImage = ""
        newAgentRepoURL = ""
        newAgentRepoBranch = ""
        newAgentProvider = ""
        newAgentSetupCommands = []
    }
"""
    
    updated = updated.replacingOccurrences(of: oldReset, with: newReset)
    
    let oldAdd = """
        let args = newAgentArgs.components(separatedBy: .whitespaces)
            .filter { !$0.isEmpty }

        let agent = AgentConfig(
            name: name,
            url: newAgentURL,
            type: "acp_agent",
            command: newAgentCommand.isEmpty ? nil : newAgentCommand,
            args: args.isEmpty ? nil : args,
            env: nil,
            workdir: newAgentWorkdir.isEmpty ? nil : newAgentWorkdir,
            port: nil,
            subAgent: nil,
            enabled: true,
            description: newAgentDescription.isEmpty ? nil : newAgentDescription,
            tags: nil
        )
"""
    
    let newAdd = """
        let args = newAgentArgs.components(separatedBy: .whitespaces)
            .filter { !$0.isEmpty }

        var workspaceConfig: WorkspaceConfig? = nil
        if !newAgentBaseImage.isEmpty || !newAgentRepoURL.isEmpty || !newAgentSetupCommands.isEmpty {
            workspaceConfig = WorkspaceConfig(
                baseImage: newAgentBaseImage.isEmpty ? nil : newAgentBaseImage,
                repoUrl: newAgentRepoURL.isEmpty ? nil : newAgentRepoURL,
                repoBranch: newAgentRepoBranch.isEmpty ? nil : newAgentRepoBranch,
                provider: newAgentProvider.isEmpty ? nil : newAgentProvider,
                setupCommands: newAgentSetupCommands.isEmpty ? nil : newAgentSetupCommands
            )
        }

        let agent = AgentConfig(
            name: name,
            url: newAgentURL.isEmpty ? nil : newAgentURL,
            type: "acp_agent",
            command: newAgentCommand.isEmpty ? nil : newAgentCommand,
            args: args.isEmpty ? nil : args,
            env: nil,
            workdir: newAgentWorkdir.isEmpty ? nil : newAgentWorkdir,
            port: nil,
            subAgent: nil,
            enabled: true,
            description: newAgentDescription.isEmpty ? nil : newAgentDescription,
            tags: nil,
            workspaceConfig: workspaceConfig
        )
"""
    
    updated = updated.replacingOccurrences(of: oldAdd, with: newAdd)
    
    try! updated.write(toFile: path, atomically: true, encoding: .utf8)
    print("Patched AgentsViewModel.swift")
}

main()
