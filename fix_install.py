import re

with open('Diane/Diane/Services/DianeHTTPClient.swift', 'r') as f:
    content = f.read()

# Fix installGalleryAgent
install_agent = """
    func installGalleryAgent(id: String, name: String?, workdir: String?, port: Int?) async throws -> GalleryInstallResponse {
        var body: [String: Any] = [:]
        if let name = name {
            body["name"] = name
        }
        if let workdir = workdir {
            body["workdir"] = workdir
        }
        if let port = port, port > 0 {
            body["port"] = port
            body["type"] = "acp"
        }
        
        let bodyData = body.isEmpty ? nil : try JSONSerialization.data(withJSONObject: body)
        let data = try await requestWithBody("/gallery/\\(id)/install", method: "POST", body: bodyData)
        return try decode(GalleryInstallResponse.self, from: data)
    }
"""
content = re.sub(
    r'    func installGalleryAgent\(id: String, name: String\?, workdir: String\?, port: Int\?\) async throws -> GalleryInstallResponse \{\n        throw DianeHTTPClientError\.readOnlyMode\n    \}',
    install_agent.strip('\n'),
    content
)

with open('Diane/Diane/Services/DianeHTTPClient.swift', 'w') as f:
    f.write(content)

