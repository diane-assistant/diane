import re

path = "Diane/Diane/Views/AgentsView.swift"
with open(path, "r") as f:
    content = f.read()

# 1. Update text fields
old_fields = """            Form {
                TextField("Name (required)", text: $viewModel.newAgentName)
                TextField("URL (required)", text: $viewModel.newAgentURL)
                TextField("Description", text: $viewModel.newAgentDescription)
                TextField("Working Directory", text: $viewModel.newAgentWorkdir)
            }"""

new_fields = """            Form {
                Section("Basic Settings") {
                    TextField("Name (required)", text: $viewModel.newAgentName)
                    TextField("URL (optional)", text: $viewModel.newAgentURL)
                    TextField("Description", text: $viewModel.newAgentDescription)
                    TextField("Working Directory", text: $viewModel.newAgentWorkdir)
                }
                
                Section("Workspace Configuration (Emergent Agents)") {
                    TextField("Base Image", text: $viewModel.newAgentBaseImage)
                    TextField("Repository URL", text: $viewModel.newAgentRepoURL)
                    TextField("Repository Branch", text: $viewModel.newAgentRepoBranch)
                    TextField("Provider (e.g. firecracker, e2b)", text: $viewModel.newAgentProvider)
                    // We simplify the setup commands here since StringArrayEditor requires more setup
                    // For now, emergent engine supports them but we don't block the UI update on it
                }
            }"""

if old_fields in content:
    content = content.replace(old_fields, new_fields)
else:
    print("Could not find fields block")

# 2. Update disabled state
old_disabled = ".disabled(viewModel.newAgentName.isEmpty || viewModel.newAgentURL.isEmpty || viewModel.isInstalling)"
new_disabled = ".disabled(viewModel.newAgentName.isEmpty || viewModel.isInstalling)"

if old_disabled in content:
    content = content.replace(old_disabled, new_disabled)
else:
    print("Could not find disabled block")

with open(path, "w") as f:
    f.write(content)

print("Python patch AgentsView complete")
