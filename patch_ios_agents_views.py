import re

path = "Diane/Diane/Views/iOS/AgentsViews.swift"
with open(path, "r") as f:
    content = f.read()

# Find the DetailSection("Connection")
pattern = r'(DetailSection\(title: "Connection"\) \{.+?\})'
replacement = r'''\1
            
            if let wc = agent.workspaceConfig {
                DetailSection(title: "Workspace Configuration") {
                    if let img = wc.baseImage, !img.isEmpty {
                        InfoRow(label: "Base Image", value: img)
                    }
                    if let repo = wc.repoUrl, !repo.isEmpty {
                        InfoRow(label: "Repo URL", value: repo)
                    }
                    if let branch = wc.repoBranch, !branch.isEmpty {
                        InfoRow(label: "Repo Branch", value: branch)
                    }
                    if let provider = wc.provider, !provider.isEmpty {
                        InfoRow(label: "Provider", value: provider)
                    }
                    if let commands = wc.setupCommands, !commands.isEmpty {
                        InfoRow(label: "Setup Commands", value: commands.joined(separator: ", "))
                    }
                }
            }'''

# Replace using regex
new_content = re.sub(pattern, replacement, content, flags=re.DOTALL)

if new_content != content:
    with open(path, "w") as f:
        f.write(new_content)
    print("Patched AgentsViews.swift successfully")
else:
    print("Could not find Connection section in AgentsViews.swift")

