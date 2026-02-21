import re

with open('Diane/Diane/Views/AgentsView.swift', 'r') as f:
    content = f.read()

# Add standard custom agent fields
custom_agent = """
    // MARK: - Custom Agent Setup
    
    private var customAgentSheet: some View {
        VStack(spacing: 0) {
            HStack {
                Text("Add Custom Agent")
                    .font(.headline)
                Spacer()
                Button {
                    viewModel.showAddAgent = false
                } label: {
                    Image(systemName: "xmark.circle.fill")
                        .foregroundStyle(.secondary)
                }
                .buttonStyle(.plain)
            }
            .padding()
            
            Divider()
            
            Form {
                TextField("Name (required)", text: $viewModel.newAgentName)
                TextField("URL (required)", text: $viewModel.newAgentURL)
                TextField("Description", text: $viewModel.newAgentDescription)
                TextField("Working Directory", text: $viewModel.newAgentWorkdir)
            }
            .padding()
            
            if let error = viewModel.installError {
                Text(error)
                    .foregroundColor(.red)
                    .font(.caption)
                    .padding()
            }
            
            Divider()
            
            HStack {
                Button("Cancel") {
                    viewModel.showAddAgent = false
                }
                Spacer()
                Button("Add Agent") {
                    Task { await viewModel.addCustomAgent() }
                }
                .buttonStyle(.borderedProminent)
                .disabled(viewModel.newAgentName.isEmpty || viewModel.newAgentURL.isEmpty || viewModel.isInstalling)
            }
            .padding()
        }
        .frame(width: 450, height: 350)
    }
"""

content = content.replace("    // MARK: - Error View", custom_agent + "\n    // MARK: - Error View")

# Replace header Add Agent button
add_btn = """
            // Add Agent button
            Menu {
                Button("From Gallery") {
                    viewModel.showGallerySheet = true
                    Task { await viewModel.loadGallery() }
                }
                Button("Custom Agent") {
                    viewModel.showAddAgent = true
                }
            } label: {
                Label("Add Agent", systemImage: "plus")
            }
"""
content = re.sub(
    r'(\s+// Add Agent button\n\s+Button \{\n\s+viewModel\.showAddAgent = true\n\s+Task \{ await viewModel\.loadGallery\(\) \}\n\s+\} label: \{\n\s+Label\("Add Agent", systemImage: "plus"\)\n\s+\})',
    add_btn,
    content
)

# Update the sheets
sheets = """
        .sheet(isPresented: $viewModel.showAddAgent) {
            customAgentSheet
        }
        .sheet(isPresented: $viewModel.showGallerySheet) {
            addAgentSheet
        }
"""
content = re.sub(
    r'(\s+\.sheet\(isPresented: \$viewModel\.showAddAgent\) \{\n\s+addAgentSheet\n\s+\})',
    sheets,
    content
)

with open('Diane/Diane/Views/AgentsView.swift', 'w') as f:
    f.write(content)

