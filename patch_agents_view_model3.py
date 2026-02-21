import re

path = "Diane/Diane/ViewModels/AgentsViewModel.swift"
with open(path, "r") as f:
    content = f.read()

old_reset = """            // Reset
            newAgentName = ""
            newAgentURL = ""
            newAgentDescription = ""
            newAgentWorkdir = ""
        } catch {"""

new_reset = """            // Reset
            newAgentName = ""
            newAgentURL = ""
            newAgentDescription = ""
            newAgentWorkdir = ""
            newAgentBaseImage = ""
            newAgentRepoURL = ""
            newAgentRepoBranch = ""
            newAgentProvider = ""
            newAgentSetupCommands = []
        } catch {"""

if old_reset in content:
    content = content.replace(old_reset, new_reset)
else:
    print("Could not find reset block")

with open(path, "w") as f:
    f.write(content)

print("Python patch 3 complete")
