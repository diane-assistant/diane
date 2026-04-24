import SwiftUI

struct EmergentMCPServerRegistryView: View {
    @State private var viewModel = EmergentMCPRegistryViewModel()
    @State private var showingCreateSheet = false
    @State private var showingDeleteAlert = false
    @State private var serverToDelete: EmergentMCPServerDTO?
    
    var body: some View {
        MasterDetailView(
            master: { masterList },
            detail: { detailPane }
        )
        .task {
            await viewModel.loadServers()
        }
        .sheet(isPresented: $showingCreateSheet) {
            EmergentMCPServerFormSheet(
                onSubmit: { dto in
                    try await viewModel.createServer(dto: dto)
                },
                onCancel: {
                    showingCreateSheet = false
                }
            )
        }
        .alert("Delete MCP Server", isPresented: $showingDeleteAlert) {
            Button("Cancel", role: .cancel) { }
            Button("Delete", role: .destructive) {
                if let id = serverToDelete?.id {
                    Task { await viewModel.deleteServer(id: id) }
                }
            }
        } message: {
            Text("Are you sure you want to delete '\(serverToDelete?.name ?? "")'? This cannot be undone.")
        }
    }
    
    private var masterList: some View {
        VStack(spacing: 0) {
            MasterListHeader(icon: "server.rack", title: "MCP Servers", count: viewModel.servers.count)
            
            if viewModel.isLoading && viewModel.servers.isEmpty {
                Spacer()
                ProgressView()
                Spacer()
            } else if viewModel.servers.isEmpty {
                EmptyStateView(
                    icon: "server.rack",
                    title: "No MCP Servers",
                    description: "Register an MCP server to expose tools to your agents.",
                    actionLabel: "Add Server",
                    action: {
                        showingCreateSheet = true
                    }
                )
            } else {
                ScrollView {
                    LazyVStack(spacing: 0) {
                        ForEach(viewModel.servers) { server in
                            serverRow(server)
                                .contentShape(Rectangle())
                                .background(viewModel.selectedServerId == server.id ? Color.accentColor.opacity(0.1) : Color.clear)
                                .onTapGesture {
                                    viewModel.selectedServerId = server.id
                                }
                                .contextMenu {
                                    if server.type != .builtin {
                                        Button("Delete", role: .destructive) {
                                            serverToDelete = server
                                            showingDeleteAlert = true
                                        }
                                    }
                                }
                            Divider().padding(.leading, 16)
                        }
                    }
                }
            }
        }
        .background(Color(NSColor.controlBackgroundColor))
    }
    
    private func serverRow(_ server: EmergentMCPServerDTO) -> some View {
        VStack(alignment: .leading, spacing: Spacing.xSmall) {
            HStack {
                Text(server.name)
                    .font(.system(.body, weight: .medium))
                    .foregroundColor(server.enabled ? .primary : .secondary)
                
                Spacer()
                
                Text(server.type.rawValue.uppercased())
                    .font(.system(size: 9, weight: .bold))
                    .padding(.horizontal, 4)
                    .padding(.vertical, 2)
                    .background(Color.blue.opacity(0.2))
                    .foregroundColor(.blue)
                    .cornerRadius(4)
            }
            
            HStack {
                Text(server.enabled ? "Enabled" : "Disabled")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                
                Spacer()
                
                if let count = server.toolCount {
                    Text("\(count) tools")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
            }
        }
        .padding(.horizontal, 16)
        .padding(.vertical, 10)
    }
    
    @ViewBuilder
    private var detailPane: some View {
        if let detail = viewModel.selectedServerDetail {
            VStack(spacing: 0) {
                detailHeader(detail: detail)
                Divider()
                
                if let err = viewModel.error ?? viewModel.editError {
                    Text(err)
                        .font(.caption)
                        .foregroundStyle(.red)
                        .padding()
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .background(Color.red.opacity(0.1))
                }
                
                ScrollView {
                    VStack(spacing: Spacing.large) {
                        DetailSection(title: "Configuration") {
                            if viewModel.isInEditMode {
                                TextField("Name", text: $viewModel.editName)
                                Toggle("Enabled", isOn: $viewModel.editEnabled)
                                InfoRow(label: "Type", value: detail.type.rawValue.uppercased())
                                
                                if detail.type == .stdio {
                                    TextField("Command", text: $viewModel.editCommand)
                                    StringArrayEditor(items: $viewModel.editArgs, title: "Arguments", placeholder: "e.g. --port 8080")
                                    KeyValueEditor(items: $viewModel.editEnv, title: "Environment", keyPlaceholder: "Key", valuePlaceholder: "Value")
                                } else if detail.type == .sse || detail.type == .http {
                                    TextField("URL", text: $viewModel.editUrl)
                                    KeyValueEditor(items: $viewModel.editHeaders, title: "Headers", keyPlaceholder: "Header", valuePlaceholder: "Value")
                                    KeyValueEditor(items: $viewModel.editEnv, title: "Environment", keyPlaceholder: "Key", valuePlaceholder: "Value")
                                }
                            } else {
                                InfoRow(label: "Type", value: detail.type.rawValue.uppercased())
                                InfoRow(label: "Enabled", value: detail.enabled ? "Yes" : "No")
                                
                                if detail.type == .stdio {
                                    InfoRow(label: "Command", value: detail.command ?? "-")
                                    InfoRow(label: "Args", value: detail.args?.joined(separator: " ") ?? "-")
                                } else if detail.type == .sse || detail.type == .http {
                                    InfoRow(label: "URL", value: detail.url ?? "-")
                                }
                                
                                if let env = detail.env, !env.isEmpty {
                                    InfoRow(label: "Environment", value: "\(env.count) variables")
                                }
                            }
                        }
                        
                        if !viewModel.isInEditMode {
                            DetailSection(title: "Tools (\(detail.tools?.count ?? 0))") {
                                if let tools = detail.tools, !tools.isEmpty {
                                    ForEach(tools) { tool in
                                        VStack(alignment: .leading, spacing: 4) {
                                            HStack {
                                                Text(tool.toolName).font(.system(.body, design: .monospaced, weight: .semibold))
                                                if !tool.enabled {
                                                    Text("Disabled")
                                                        .font(.system(size: 9))
                                                        .padding(.horizontal, 4).padding(.vertical, 2)
                                                        .background(Color.red.opacity(0.2))
                                                        .foregroundColor(.red).cornerRadius(4)
                                                }
                                            }
                                            if let desc = tool.description {
                                                Text(desc).font(.caption).foregroundStyle(.secondary)
                                            }
                                        }
                                        .padding(.vertical, 4)
                                    }
                                } else {
                                    Text("No tools associated with this server.")
                                        .font(.caption)
                                        .foregroundStyle(.secondary)
                                }
                            }
                        }
                    }
                    .padding(.bottom, Padding.large)
                }
            }
        } else {
            EmptyStateView(
                icon: "server.rack",
                title: "No Server Selected",
                description: "Select an MCP server from the list to view its configuration and tools.",
                actionLabel: "Add Server",
                action: {
                    showingCreateSheet = true
                }
            )
        }
    }
    
    private func detailHeader(detail: EmergentMCPServerDetailDTO) -> some View {
        HStack {
            Text(detail.name)
                .font(.title2)
                .fontWeight(.semibold)
            
            Spacer()
            
            if viewModel.isInEditMode {
                Button("Discard") {
                    viewModel.discardInlineEdit()
                }
                .keyboardShortcut(.escape, modifiers: [])
                
                Button("Save") {
                    Task { await viewModel.saveInlineEdit() }
                }
                .buttonStyle(.borderedProminent)
                .disabled(!viewModel.hasChanges)
                .keyboardShortcut(.defaultAction)
            } else {
                Button {
                    showingCreateSheet = true
                } label: {
                    Label("Add Server", systemImage: "plus")
                }
                
                if detail.type != .builtin {
                    Button {
                        viewModel.enterEditMode()
                    } label: {
                        Label("Edit", systemImage: "pencil")
                    }
                    
                    Menu {
                        Button("Delete", role: .destructive) {
                            serverToDelete = EmergentMCPServerDTO(
                                id: detail.id, projectId: detail.projectId, name: detail.name,
                                type: detail.type, command: detail.command, args: detail.args,
                                env: detail.env, url: detail.url, headers: detail.headers,
                                enabled: detail.enabled, toolCount: detail.toolCount,
                                createdAt: detail.createdAt, updatedAt: detail.updatedAt
                            )
                            showingDeleteAlert = true
                        }
                    } label: {
                        Image(systemName: "ellipsis.circle")
                    }
                    .menuStyle(.borderlessButton)
                    .frame(width: 30)
                }
            }
        }
        .padding()
    }
}
