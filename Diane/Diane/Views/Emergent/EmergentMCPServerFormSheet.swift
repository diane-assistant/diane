import SwiftUI

struct EmergentMCPServerFormSheet: View {
    let onSubmit: (EmergentCreateMCPServerDTO) async throws -> Void
    let onCancel: () -> Void
    
    @State private var name = ""
    @State private var type: EmergentMCPServerType = .stdio
    @State private var command = ""
    @State private var args: [String] = []
    @State private var env: [String: String] = [:]
    @State private var url = ""
    @State private var headers: [String: String] = [:]
    @State private var enabled = true
    
    @State private var isSubmitting = false
    @State private var errorMessage: String?
    
    var isValid: Bool {
        if name.isEmpty { return false }
        switch type {
        case .stdio:
            return !command.isEmpty
        case .sse, .http:
            return !url.isEmpty
        default:
            return false
        }
    }
    
    var body: some View {
        VStack(spacing: 0) {
            HStack {
                Text("Add MCP Server")
                    .font(.headline)
                Spacer()
                Button("Cancel", action: onCancel)
                    .keyboardShortcut(.escape, modifiers: [])
            }
            .padding()
            
            Divider()
            
            Form {
                if let err = errorMessage {
                    Text(err).foregroundStyle(.red).font(.caption)
                }
                
                Section("General") {
                    TextField("Name", text: $name)
                    Toggle("Enabled", isOn: $enabled)
                    Picker("Type", selection: $type) {
                        Text("STDIO").tag(EmergentMCPServerType.stdio)
                        Text("SSE").tag(EmergentMCPServerType.sse)
                        Text("HTTP").tag(EmergentMCPServerType.http)
                    }
                }
                
                Section("Configuration") {
                    if type == .stdio {
                        TextField("Command", text: $command)
                        StringArrayEditor(items: $args, title: "Arguments", placeholder: "e.g. --port 8080")
                        KeyValueEditor(items: $env, title: "Environment Variables", keyPlaceholder: "Key", valuePlaceholder: "Value")
                    } else if type == .sse || type == .http {
                        TextField("URL", text: $url)
                        KeyValueEditor(items: $headers, title: "Headers", keyPlaceholder: "Header", valuePlaceholder: "Value")
                        KeyValueEditor(items: $env, title: "Environment Variables", keyPlaceholder: "Key", valuePlaceholder: "Value")
                    }
                }
                
                Section {
                    Button(action: submit) {
                        if isSubmitting {
                            ProgressView().scaleEffect(0.5).frame(width: 40)
                        } else {
                            Text("Save Server")
                        }
                    }
                    .keyboardShortcut(.defaultAction)
                    .disabled(!isValid || isSubmitting)
                }
            }
            .formStyle(.grouped)
            .padding()
            .frame(width: 500, height: 600)
        }
    }
    
    private func submit() {
        isSubmitting = true
        errorMessage = nil
        
        let dto = EmergentCreateMCPServerDTO(
            name: name,
            type: type,
            command: type == .stdio ? command : nil,
            args: type == .stdio ? args : nil,
            env: env,
            url: (type == .sse || type == .http) ? url : nil,
            headers: (type == .sse || type == .http) ? headers : nil,
            enabled: enabled
        )
        
        Task {
            do {
                try await onSubmit(dto)
                onCancel()
            } catch {
                errorMessage = error.localizedDescription
                isSubmitting = false
            }
        }
    }
}
