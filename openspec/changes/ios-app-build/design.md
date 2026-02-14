## Context

Diane is a macOS-native SwiftUI application that manages MCP servers, AI providers, agents, contexts, and scheduled jobs. It communicates with a local Go daemon via Unix socket using shelled-out `curl` commands (`Process` class). The Go server uses `http.ServeMux` on the Unix socket and exposes ~50 API endpoints. The macOS app uses AppKit-specific APIs extensively: `NSApplicationDelegateAdaptor`, `MenuBarExtra`, `NSApp.windows`, `Process`, `NSPasteboard`, and `HSplitView`.

The iOS companion app will be a read-only monitoring interface connecting to the same Go server over HTTP via Tailscale VPN, targeting iPhone and iPad.

## Goals / Non-Goals

**Goals:**
- Provide read-only visibility into all Diane configuration from iPhone and iPad
- Reuse existing `Codable` models, `DianeClientProtocol`, ViewModels, and DesignTokens across platforms
- Add an opt-in HTTP listener to the Go server with minimal changes to the existing architecture
- Support adaptive layouts: tab navigation on iPhone, sidebar on iPad

**Non-Goals:**
- No write operations from iOS in this version (create, edit, delete, toggle)
- No authentication on the HTTP listener (Tailscale VPN provides network-level security)
- No TLS/HTTPS (plain HTTP over trusted Tailscale network)
- No offline mode or data caching beyond in-memory state
- No push notifications or background refresh
- No Catalyst or "one binary for both platforms" approach

## Decisions

### 1. Separate iOS target, shared code via Xcode target membership

**Decision**: Create a new iOS app target (`DianeIOS`) in the existing `Diane.xcodeproj`, sharing source files via target membership rather than extracting a Swift package.

**Rationale**: The project is a pure Xcode project (no SPM for app code). Adding a second target with shared file membership is the simplest path. Models, protocol, DesignTokens, and portable components can be added to both targets. Platform-specific files (DianeApp.swift vs a new DianeIOSApp.swift, DianeClient.swift vs DianeHTTPClient.swift) belong to only one target.

**Alternative considered**: Extracting shared code into a local Swift package. This adds build complexity and refactoring effort for a v1 where the sharing boundary is straightforward. Can revisit if the shared surface grows.

**Shared files** (both targets):
- `Models/` — all 6 model files (already `Codable`, no platform dependencies)
- `Services/DianeClientProtocol.swift` — the protocol abstraction
- `Components/DesignTokens.swift` — already cross-platform (no `nsColor` references)
- `Components/EmptyStateView.swift`, `InfoRow.swift`, `TimeRangePicker.swift` — portable as-is

**iOS-only files**:
- `DianeIOSApp.swift` — iOS app entry point
- `Services/DianeHTTPClient.swift` — URLSession-based client
- `Views/iOS/` — iOS-specific views and navigation

**macOS-only files** (existing, unchanged):
- `App/DianeApp.swift`, `App/AppDelegate` functionality
- `Services/DianeClient.swift` — Unix socket client
- `Views/MainWindowView.swift`, `MenuBarView.swift`
- `Components/MasterDetailView.swift` — uses `HSplitView`

### 2. URLSession-based HTTP client implementing DianeClientProtocol

**Decision**: Create `DianeHTTPClient` that implements `DianeClientProtocol` using `URLSession` with a configurable base URL, replacing the Unix socket + `curl` transport.

**Rationale**: The existing `DianeClientProtocol` has ~60 methods covering all API operations. The iOS client only needs to implement the read (GET) methods for v1. Write methods can throw "not supported" errors. `URLSession` is the standard iOS networking API and handles HTTP natively.

**Implementation approach**:
- A private `request(_ path: String, method: String, body: Data?) async throws -> Data` method mirrors the macOS client's central method but uses `URLSession.data(for:)` instead of `Process`+`curl`
- Base URL is constructed from user-configured host and port (e.g., `http://my-mac.tail12345.ts.net:8080`)
- Reuse the same `makeGoCompatibleDecoder()` for date parsing
- Write methods throw a descriptive error (`DianeHTTPClientError.readOnlyMode`)
- 10-second timeout per request, matching the spec

### 3. Go server HTTP listener as opt-in secondary listener

**Decision**: Add an HTTP TCP listener alongside the existing Unix socket listener in the Go server, gated by a `--http-addr` flag or `http_addr` config field. The HTTP listener reuses the same `http.ServeMux` and handlers.

**Rationale**: The Go server already uses `http.ServeMux` for routing. The Unix socket is just `http.Serve(unixListener, mux)`. Adding a TCP listener is `http.Serve(tcpListener, mux)` with the same mux. A middleware wrapper on the TCP listener rejects non-GET requests with 405, enforcing read-only access without modifying individual handlers.

**Implementation approach**:
- In `api.go`, after creating the mux and registering routes, optionally start a second goroutine: `go http.Serve(tcpListener, readOnlyMiddleware(mux))`
- `readOnlyMiddleware` checks `r.Method != "GET"` and returns 405 with `Allow: GET` header
- Default off: if `--http-addr` is empty, no TCP listener starts
- Logging: `log.Printf("HTTP listener on %s", addr)` at startup

### 4. iOS navigation: TabView + NavigationStack (iPhone), NavigationSplitView (iPad)

**Decision**: Use `@Environment(\.horizontalSizeClass)` to switch between `TabView` (compact) and `NavigationSplitView` (regular) at the root level.

**Rationale**: This is the standard SwiftUI adaptive pattern for universal iOS apps. iPhone always gets tabs; iPad gets sidebar in landscape and tabs in compact split view. The macOS app already uses `NavigationSplitView` with a sidebar, so iPad users get a familiar layout.

**Tab structure** (compact/iPhone):
- Status (house icon)
- Servers (server.rack icon)
- Agents (cpu icon)
- More (ellipsis.circle icon) → list of: Providers, Contexts, Jobs, Usage, Settings

**Sidebar sections** (regular/iPad):
- All 8 sections listed directly, matching the macOS sidebar

### 5. iOS views: new read-only views, not adapted macOS views

**Decision**: Create new iOS view files rather than adding `#if os()` conditionals throughout the existing macOS views.

**Rationale**: The macOS views contain significant write functionality (forms, editors, toggle switches, delete buttons, OAuth flows) that would need to be conditionally removed for iOS. The read-only iOS views are simpler: list → detail with `InfoRow` display. Writing fresh iOS views is less error-prone and keeps the macOS views clean. Shared components (DesignTokens, InfoRow, EmptyStateView) are still reused.

**View structure per section**:
- `{Section}ListView.swift` — list view with search/filter
- `{Section}DetailView.swift` — read-only detail with sections

### 6. Components with Color(nsColor:) — centralize in DesignTokens

**Decision**: Add semantic color properties to `DesignTokens.swift` with `#if os(macOS)` / `#if os(iOS)` guards, then update affected components to use these properties instead of inline `Color(nsColor:)`.

**Rationale**: `DesignTokens.swift` itself has no `nsColor` references (confirmed), but several components do: `SummaryCard`, `DetailSection`, `KeyValueEditor`, `StringArrayEditor`. Adding `static var windowBackground: Color`, `static var controlBackground: Color`, etc. to DesignTokens centralizes the platform branching in one place. Components then use `DesignTokens.Colors.windowBackground` and become cross-platform.

**Colors to add** (mapped from macOS components):
- `windowBackground` → macOS: `Color(nsColor: .windowBackgroundColor)`, iOS: `Color(.systemBackground)`
- `controlBackground` → macOS: `Color(nsColor: .controlBackgroundColor)`, iOS: `Color(.secondarySystemBackground)`
- `textBackground` → macOS: `Color(nsColor: .textBackgroundColor)`, iOS: `Color(.systemBackground)`
- `separatorColor` → macOS: `Color(nsColor: .separatorColor)`, iOS: `Color(.separator)`

### 7. StatusMonitor adapted for iOS lifecycle

**Decision**: Create an `IOSStatusMonitor` that wraps `DianeHTTPClient`, polls on a timer when the app is foregrounded, and pauses when backgrounded. Connection state is published as `@Published` for SwiftUI binding.

**Rationale**: The macOS `StatusMonitor` uses the Unix socket client and doesn't need to handle app backgrounding. The iOS version needs `UIApplication` lifecycle observation (`scenePhase` in SwiftUI) to start/stop polling. The polling interval (5 seconds) and connection state enum (`connected`, `disconnected`, `connecting`, `error`) can mirror the macOS implementation.

### 8. iOS deployment target: iOS 17.0

**Decision**: Target iOS 17.0 minimum.

**Rationale**: The macOS app targets macOS 15.0 (released fall 2024). iOS 17.0 (released fall 2023) provides `NavigationSplitView` improvements, `@Observable` macro support, and modern SwiftUI APIs. iOS 17 adoption is high enough that targeting it is reasonable for a developer tool.

## Risks / Trade-offs

- **DianeClientProtocol size**: The protocol has ~60 methods. The iOS client must implement all of them (even if just to throw for write methods). If the protocol grows, maintenance burden increases. Mitigation: consider splitting into `DianeReadClient` and `DianeWriteClient` protocols in a future iteration.
- **API compatibility**: The Go server's JSON response format is the contract between platforms. Changes to the Go API affect both macOS and iOS clients. Mitigation: the shared `Codable` models serve as a single source of truth for JSON shapes.
- **No authentication**: Plain HTTP without auth is acceptable only because Tailscale provides network-level security. If the HTTP listener is ever exposed on a non-VPN network, authentication must be added before that happens.
- **Polling overhead**: 5-second polling from multiple iOS devices could add load to the Go server. Mitigation: the Go server is lightweight and already handles frequent Unix socket polling from the macOS client. Can add rate limiting to the HTTP listener if needed.
- **View duplication**: Separate iOS views mean changes to data display logic must be applied in two places. Mitigation: keep views thin (display only), put logic in shared ViewModels.

## Open Questions

- Should the iOS app support multiple saved server connections, or just one at a time?
- Should we implement Bonjour/mDNS discovery so the iOS app can auto-discover Diane servers on the Tailscale network instead of manual address entry?
- For v2, should write operations go through the HTTP listener or should we implement a different transport (e.g., WebSocket for bidirectional communication)?
