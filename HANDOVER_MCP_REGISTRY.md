# Handover: MCP Registry & Node-Centric Deployment

## Status Summary
*   **Backend (Go):** ✅ **COMPLETE & VERIFIED**. The `mcp_server_placements` table, migration logic, and API endpoints (`/hosts`, `/mcp-placements`, `/mcp-registry`) are fully implemented and tested via `curl` on the Unix socket.
*   **Frontend (Swift):** ⚠️ **Build Failing**. The `DianeHTTPClient` was successfully updated, but the Mac app build fails due to missing file references in the Xcode project and some type errors in the ViewModel.

## Current Issues (Blocking Build)

### 1. Missing File Reference: `MCPRegistryView.swift`
**Error:** `/Diane/Diane/Views/MainWindowView.swift:189:13: error: cannot find 'MCPRegistryView' in scope`
**Diagnosis:** The file `Diane/Diane/Views/MCPRegistryView.swift` exists on disk but is **not** included in `Diane.xcodeproj`.
**Action Required:** You must add this file to the Xcode project. Since you cannot use the Xcode GUI, you have two options:
*   **Option A (Recommended):** Use a tool (if available) or script to manipulate the `.pbxproj`.
*   **Option B (Hack/Fallback):** If editing `.pbxproj` is too risky, copy the content of `MCPRegistryView.swift` and append it to the end of `MCPServersView.swift` (which *is* in the project). This makes the type available to the compiler without modifying the project file.

### 2. Type Logic Error: `MCPServersViewModel.swift`
**Error:** `Line 70: cannot call value of non-function type 'MCPServerPlacement?'`
**Code:**
```swift
let placement = placements.first { $0.server.id == server.id }
```
**Diagnosis:** The compiler is confused by the syntax or type inference here. `MCPServerPlacement` might contain a property named `server` that is also optional, or `placements` is not what we think.
**Action Required:**
*   Check the definition of `MCPServerPlacement`.
*   Rewrite the closure to be explicit: `placements.first(where: { $0.server?.id == server.id })`.

### 3. SwiftUI Identifier Error: `MCPServersViewModel.swift`
**Error:** `referencing instance method 'id' on 'Optional' requires that 'MCPServer' conform to 'View'`
**Diagnosis:** This usually happens in a `List` or `ForEach` where the ID selection is ambiguous or applied to an optional value incorrectly.
**Action Required:**
*   Examine how `List(serversWithPlacementStatus)` or similar is constructing rows.
*   Ensure `MCPServer` conforms to `Identifiable` or an explicit `id: \.self` (or `\.id`) is provided correctly.

## Next Steps for the Agent

1.  **Fix `MCPServersViewModel.swift`**:
    *   Read `Diane/Diane/Models/MCPServer.swift` to verify the `MCPServer` and `MCPServerPlacement` struct definitions.
    *   Edit `Diane/Diane/ViewModels/MCPServersViewModel.swift` to fix the closure syntax on line 70 and the iteration logic causing the "id" error.

2.  **Fix `MCPRegistryView` Missing Reference**:
    *   Read `Diane/Diane/Views/MCPRegistryView.swift` to capture its content.
    *   Append that content to the end of `Diane/Diane/Views/MCPServersView.swift` (or another existing view file) to bring it into the compilation scope.
    *   *Alternatively*, if you have a reliable way to add files to `project.pbxproj`, do that.

3.  **Rebuild & Verify**:
    *   Run `make install-app`.
    *   Verify the build succeeds.
    *   Launch the app and test `Cmd+1` (MCP Servers) and `Cmd+0` (MCP Registry).

## Verified Backend Data (Reference)
The backend returns placements in this format (verified via `curl`):
```json
[
  {
    "id": 8,
    "server_id": 8,
    "host_id": "master",
    "enabled": true,
    "server": {
      "id": 8,
      "name": "apple",
      "enabled": true,
      "type": "builtin",
      "node_mode": "master"
    }
  }
]
```
Ensure the Swift `Codable` structs match this nested structure.
