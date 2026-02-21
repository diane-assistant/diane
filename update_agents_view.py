import re

with open('Diane/Diane/Views/AgentsView.swift', 'r') as f:
    content = f.read()

# Add sheet for Edit
content = re.sub(
    r'(\.sheet\(isPresented: \$viewModel\.showAddAgent\) \{\n\s+addAgentSheet\n\s+\})',
    r'\1\n        .sheet(isPresented: $viewModel.showEditAgent) {\n            editAgentSheet\n        }',
    content
)

# Add Edit button to detailView header
edit_button = """
                        Button {
                            viewModel.startEditing()
                        } label: {
                            Image(systemName: "pencil")
                        }
                        .buttonStyle(.plain)
                        .help("Edit Agent")
                        
"""
content = re.sub(
    r'(                        // Status badge\n                        if let result = viewModel\.testResults\[agent\.name\] \{\n                            statusBadge\(for: result\)\n                        \}\n                    \})',
    edit_button + r'\1',
    content
)

# Add editAgentSheet view
edit_sheet = """
    // MARK: - Edit Agent Sheet
    
    private var editAgentSheet: some View {
        VStack(spacing: 0) {
            HStack {
                Text("Edit Agent")
                    .font(.headline)
                Spacer()
                Button {
                    viewModel.showEditAgent = false
                } label: {
                    Image(systemName: "xmark.circle.fill")
                        .foregroundStyle(.secondary)
                }
                .buttonStyle(.plain)
            }
            .padding()
            
            Divider()
            
            VStack(alignment: .leading, spacing: 16) {
                if let agent = viewModel.selectedAgent {
                    Text("Editing: \\(agent.displayName)")
                        .font(.subheadline.weight(.medium))
                }
                
                VStack(alignment: .leading, spacing: 4) {
                    Text("Description")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                    TextField("Optional description", text: $viewModel.editAgentDescription)
                        .textFieldStyle(.roundedBorder)
                }
                
                VStack(alignment: .leading, spacing: 4) {
                    Text("Working Directory")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                    TextField("Optional absolute path", text: $viewModel.editAgentWorkdir)
                        .textFieldStyle(.roundedBorder)
                }
                
                if let error = viewModel.editError {
                    Text(error)
                        .font(.caption)
                        .foregroundStyle(.red)
                }
                
                Spacer()
            }
            .padding()
            
            Divider()
            
            HStack {
                Button("Cancel") {
                    viewModel.showEditAgent = false
                }
                .keyboardShortcut(.escape, modifiers: [])
                
                Spacer()
                
                Button {
                    Task { await viewModel.saveEdit() }
                } label: {
                    if viewModel.isEditing {
                        ProgressView().scaleEffect(0.6)
                    } else {
                        Text("Save")
                    }
                }
                .buttonStyle(.borderedProminent)
                .keyboardShortcut(.defaultAction)
                .disabled(viewModel.isEditing)
            }
            .padding()
        }
        .frame(width: 400, height: 300)
    }
"""

content = re.sub(
    r'(    // MARK: - Add Agent Sheet)',
    edit_sheet + r'\n\1',
    content
)

with open('Diane/Diane/Views/AgentsView.swift', 'w') as f:
    f.write(content)

