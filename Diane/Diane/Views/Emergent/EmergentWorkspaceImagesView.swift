import SwiftUI

struct EmergentWorkspaceImagesView: View {
    @State private var viewModel = EmergentWorkspaceImagesViewModel()
    @State private var showingRegisterSheet = false
    @State private var imageToDelete: EmergentWorkspaceImageDTO?
    @State private var showingDeleteAlert = false
    
    var body: some View {
        Group {
            if viewModel.isLoading && viewModel.images.isEmpty {
                ProgressView()
                    .frame(maxWidth: .infinity, maxHeight: .infinity)
            } else if let error = viewModel.error, viewModel.images.isEmpty {
                VStack {
                    Text("Error loading images")
                        .font(.headline)
                    Text(error)
                        .foregroundStyle(.secondary)
                    Button("Retry") {
                        Task { await viewModel.loadImages() }
                    }
                    .padding()
                }
                .frame(maxWidth: .infinity, maxHeight: .infinity)
            } else if viewModel.images.isEmpty {
                EmptyStateView(
                    icon: "shippingbox",
                    title: "No Workspace Images",
                    description: "Register a custom docker image to use it as a sandbox environment.",
                    actionLabel: "Add Image",
                    action: {
                        showingRegisterSheet = true
                    }
                )
            } else {
                List(selection: $viewModel.selectedImage) {
                    ForEach(viewModel.images) { image in
                        imageRow(image)
                            .tag(image)
                            .contextMenu {
                                if image.type != "built-in" {
                                    Button("Delete", role: .destructive) {
                                        imageToDelete = image
                                        showingDeleteAlert = true
                                    }
                                }
                            }
                    }
                }
            }
        }
        .toolbar {
            ToolbarItem {
                Button {
                    showingRegisterSheet = true
                } label: {
                    Label("Add Image", systemImage: "plus")
                }
            }
            ToolbarItem {
                Button {
                    Task { await viewModel.loadImages() }
                } label: {
                    Label("Refresh", systemImage: "arrow.clockwise")
                }
            }
        }
        .sheet(isPresented: $showingRegisterSheet) {
            registerImageSheet
        }
        .alert("Delete Image", isPresented: $showingDeleteAlert) {
            Button("Cancel", role: .cancel) { }
            Button("Delete", role: .destructive) {
                if let id = imageToDelete?.id {
                    Task { await viewModel.deleteImage(id: id) }
                }
            }
        } message: {
            Text("Are you sure you want to delete '\(imageToDelete?.name ?? "")'? This cannot be undone.")
        }
        .task {
            await viewModel.loadImages()
        }
    }
    
    @ViewBuilder
    private func imageRow(_ image: EmergentWorkspaceImageDTO) -> some View {
        HStack {
            VStack(alignment: .leading, spacing: 2) {
                HStack(spacing: 8) {
                    Text(image.name)
                        .font(.headline)
                    
                    if let type = image.type {
                        Text(type.uppercased())
                            .font(.system(size: 9, weight: .bold))
                            .padding(.horizontal, 4)
                            .padding(.vertical, 2)
                            .background(type == "built-in" ? Color.blue.opacity(0.2) : Color.purple.opacity(0.2))
                            .foregroundColor(type == "built-in" ? .blue : .purple)
                            .cornerRadius(4)
                    }
                    
                    if let status = image.status {
                        Text(status)
                            .font(.system(size: 9, weight: .semibold))
                            .padding(.horizontal, 4)
                            .padding(.vertical, 2)
                            .background(status == "ready" ? Color.green.opacity(0.2) : Color.orange.opacity(0.2))
                            .foregroundColor(status == "ready" ? .green : .orange)
                            .cornerRadius(4)
                    }
                }
                
                if let ref = image.dockerRef {
                    Text(ref)
                        .font(.caption.monospaced())
                        .foregroundStyle(.secondary)
                }
                
                if let errorMsg = image.errorMsg, !errorMsg.isEmpty {
                    Text(errorMsg)
                        .font(.caption)
                        .foregroundStyle(.red)
                }
            }
            
            Spacer()
            
            if let provider = image.provider {
                ImageBadge(label: provider, icon: "shippingbox")
            }
        }
        .padding(.vertical, 4)
    }
    
    private var registerImageSheet: some View {
        VStack(spacing: 0) {
            HStack {
                Text("Register Custom Image")
                    .font(.headline)
                Spacer()
                Button("Cancel") {
                    showingRegisterSheet = false
                }
                .keyboardShortcut(.escape, modifiers: [])
            }
            .padding()
            
            Divider()
            
            VStack {
                if let err = viewModel.registerError {
                    Text(err).foregroundStyle(.red).font(.caption)
                }
                
                GroupBox("Details") {
                    VStack(alignment: .leading, spacing: 12) {
                        TextField("Name", text: $viewModel.newImageName)
                        TextField("Docker Ref", text: $viewModel.newImageDockerRef)
                            .help("e.g. ubuntu:latest, python:3.9-slim")
                        
                        Picker("Provider (Optional)", selection: $viewModel.newImageProvider) {
                            Text("Auto").tag("")
                            Text("Firecracker").tag("firecracker")
                            Text("gVisor").tag("gvisor")
                            Text("E2B").tag("e2b")
                        }
                    }
                    .padding()
                }
                
                Button(action: {
                    Task {
                        if await viewModel.registerImage() {
                            showingRegisterSheet = false
                        }
                    }
                }) {
                    if viewModel.isRegistering {
                        ProgressView().scaleEffect(0.5).frame(width: 40)
                    } else {
                        Text("Register Image")
                            .frame(maxWidth: .infinity)
                    }
                }
                .buttonStyle(.borderedProminent)
                .keyboardShortcut(.defaultAction)
                .disabled(viewModel.newImageName.isEmpty || viewModel.newImageDockerRef.isEmpty || viewModel.isRegistering)
                .padding(.top)
            }
            .padding()
            .frame(width: 400, height: 300)
        }
    }
}

// A simple local badge component for the list
fileprivate struct ImageBadge: View {
    let label: String
    let icon: String
    var body: some View {
        HStack(spacing: 4) {
            Image(systemName: icon)
            Text(label)
        }
        .font(.system(size: 10, weight: .medium))
        .padding(.horizontal, 6)
        .padding(.vertical, 3)
        .background(Color(NSColor.controlColor))
        .cornerRadius(4)
    }
}
