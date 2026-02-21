import re

path = "Diane/Diane/ViewModels/AgentsViewModel.swift"
with open(path, "r") as f:
    content = f.read()

# 1. Add properties
old_props = """    var showGallerySheet = false
    var newAgentURL = ""
    var newAgentDescription = ""
    var newAgentWorkdir = ""
    var newAgentCommand = ""
    var newAgentArgs = \"\""""

new_props = """    var showGallerySheet = false
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
    var newAgentSetupCommands = [String]()"""

if old_props in content:
    content = content.replace(old_props, new_props)
else:
    print("Could not find properties block")

# 2. Add to AgentConfig creation
old_add = """            let agent = AgentConfig(
                name: newAgentName,
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
            )"""

new_add = """            var workspaceConfig: WorkspaceConfig? = nil
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
                name: newAgentName,
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
            )"""

if old_add in content:
    content = content.replace(old_add, new_add)
else:
    print("Could not find agent creation block")

with open(path, "w") as f:
    f.write(content)

print("Python patch complete")
