# Design: Comprehensive Test Suite

## Context

DianeMenu is a macOS SwiftUI application that provides a GUI for managing the Diane daemon. The app communicates with the daemon via a Unix socket at `~/.diane/diane.sock` using the `DianeClient` service. Currently, the application has:

- **No test infrastructure**: No test targets, no testing dependencies, no test coverage
- **Tightly coupled architecture**: Business logic embedded in SwiftUI views, making unit testing difficult
- **Concrete dependencies**: Direct instantiation of `DianeClient` prevents dependency injection and mocking
- **Active development**: Recent features like MCP server duplication need test coverage to prevent regressions

**Current constraints:**
- Must maintain existing UI/UX behavior during refactoring
- Cannot break existing functionality
- Must work with Swift 5.9+ and macOS 14+
- Test suite should run in under 2 minutes locally

**Stakeholders:**
- Developers: Need confidence when refactoring and adding features
- CI/CD: Need automated test execution for quality gates
- Users: Benefit from fewer bugs and more reliable releases

## Goals / Non-Goals

**Goals:**
- Establish comprehensive test infrastructure covering unit, integration, UI, and snapshot tests
- Enable fast, reliable testing of business logic through ViewModel extraction
- Provide mocking capabilities for DianeClient to test without running daemon
- Catch visual regressions automatically with snapshot tests
- Enable CI/CD integration for automated testing
- Create clear testing patterns and examples for future development

**Non-Goals:**
- Testing every single view component (focus on critical paths: MCP server management)
- Performance benchmarking or load testing
- Testing the Diane daemon itself (only the client interface)
- Refactoring unrelated code outside the test scope
- Adding new features beyond testing infrastructure

## Decisions

### Decision 1: MVVM Architecture with Observable ViewModels

**Choice:** Extract business logic into separate ViewModel classes conforming to Swift's `Observable` protocol.

**Rationale:**
- **Testability**: ViewModels can be instantiated and tested without rendering SwiftUI views
- **Separation of concerns**: Views focus purely on presentation, ViewModels handle state and business logic
- **Modern Swift**: `@Observable` macro (Swift 5.9+) provides simple reactivity without `@Published` boilerplate
- **Dependency injection**: ViewModels accept protocol dependencies via initializer, enabling mocking

**Architecture pattern:**
```swift
// ViewModel (testable)
@Observable
class MCPServersViewModel {
    private let client: DianeClientProtocol
    var servers: [MCPServerConfig] = []
    var isLoading = false
    var errorMessage: String?
    
    init(client: DianeClientProtocol = DianeClient.shared) {
        self.client = client
    }
    
    func loadServers() async { ... }
    func createServer(_ config: MCPServerConfig) async throws { ... }
    func duplicateServer(_ server: MCPServerConfig) async throws { ... }
}

// View (minimal, delegates to ViewModel)
struct MCPServersView: View {
    @State private var viewModel: MCPServersViewModel
    
    var body: some View {
        List(viewModel.servers) { server in
            ServerRow(server: server)
        }
        .task { await viewModel.loadServers() }
    }
}
```

**Alternatives considered:**
- **Keep logic in views**: Rejected because views are difficult to test; would require ViewInspector for all tests
- **Use ObservableObject**: Rejected because `@Observable` is more modern and requires less boilerplate
- **VIPER/Clean architecture**: Rejected as too heavyweight for this app's complexity

### Decision 2: Protocol-Based DianeClient Abstraction

**Choice:** Extract `DianeClientProtocol` from `DianeClient` class to enable dependency injection.

**Rationale:**
- **Mocking**: Tests can use `MockDianeClient` without Unix socket connections
- **Isolation**: Unit tests don't depend on daemon being running
- **Flexibility**: Future implementations (e.g., HTTP client, test fixtures) can conform to protocol
- **Non-breaking**: Existing code continues using `DianeClient.shared` singleton

**Implementation approach:**
```swift
// Protocol defining the interface
protocol DianeClientProtocol {
    func getMCPServerConfigs() async throws -> [MCPServerConfig]
    func createMCPServerConfig(_ config: MCPServerConfig) async throws -> MCPServerConfig
    func updateMCPServerConfig(_ config: MCPServerConfig) async throws -> MCPServerConfig
    func deleteMCPServerConfig(_ id: String) async throws
    // ... other methods
}

// Existing class conforms to protocol
extension DianeClient: DianeClientProtocol {}

// Mock for testing
class MockDianeClient: DianeClientProtocol {
    var serverConfigs: [MCPServerConfig] = []
    var shouldThrowError: Error?
    var methodCallCounts: [String: Int] = [:]
    
    func getMCPServerConfigs() async throws -> [MCPServerConfig] {
        methodCallCounts["getMCPServerConfigs", default: 0] += 1
        if let error = shouldThrowError { throw error }
        return serverConfigs
    }
    // ... implement other methods
}
```

**Alternatives considered:**
- **Dependency injection container**: Rejected as too heavyweight; simple protocol injection is sufficient
- **Subclassing DianeClient**: Rejected because it's harder to control behavior and verify calls
- **Global mock flag**: Rejected because it creates global state and isn't thread-safe

### Decision 3: Four-Tier Testing Strategy

**Choice:** Implement four distinct testing layers with clear responsibilities.

**Test tiers:**

1. **Unit tests** (DianeMenuTests target)
   - Test ViewModels with MockDianeClient
   - Test pure functions (duplicate name generation, filtering, validation)
   - Fast execution (< 1 second per test)
   - High coverage of business logic

2. **Integration tests** (DianeMenuTests target)
   - Test DianeClient protocol conformance
   - Test ViewModel + MockClient integration
   - Test async/await behavior, error handling
   - Medium execution speed (< 5 seconds per test)

3. **Snapshot tests** (DianeMenuTests target)
   - Test visual appearance of views with SnapshotTesting
   - Capture empty state, loaded state, error state
   - Test light/dark mode, different window sizes
   - Medium execution speed (1-2 seconds per snapshot)

4. **UI tests** (DianeMenuUITests target)
   - Test end-to-end user flows with XCUITest
   - Create, edit, duplicate, delete MCP servers
   - Slow execution (5-15 seconds per test)
   - Lower coverage, focus on critical paths

**Rationale:**
- **Speed**: Fast unit tests provide quick feedback; slow UI tests run less frequently
- **Reliability**: Unit tests are deterministic; UI tests can be flaky
- **Coverage**: Unit tests cover edge cases; UI tests verify integration
- **Maintenance**: Unit tests are easy to maintain; UI tests break with UI changes

**Alternatives considered:**
- **Only UI tests**: Rejected because too slow and flaky for comprehensive coverage
- **Only unit tests**: Rejected because doesn't verify actual user workflows
- **Contract testing**: Rejected because we control both client and server

### Decision 4: SnapshotTesting Library for Visual Regression

**Choice:** Use SnapshotTesting (~1.12.0) from Point-Free for snapshot tests.

**Rationale:**
- **SwiftUI support**: Works well with SwiftUI view rendering
- **Diff generation**: Automatically generates diff images on failure
- **Multiple strategies**: Supports image, text, JSON snapshots
- **Industry standard**: Widely used in Swift community
- **Active maintenance**: Regularly updated for new iOS/macOS versions

**Configuration:**
- Snapshots stored in `DianeMenuTests/__Snapshots__/` directory
- Committed to version control (not git LFS)
- Light mode default, dark mode as separate snapshots
- Fixed window sizes (800x600, 1024x768, 1920x1080)

**Usage pattern:**
```swift
func testMCPServersView_EmptyState() {
    let mockClient = MockDianeClient()
    mockClient.serverConfigs = []
    let viewModel = MCPServersViewModel(client: mockClient)
    let view = MCPServersView(viewModel: viewModel)
        .frame(width: 1024, height: 768)
    
    assertSnapshot(matching: view, as: .image)
}
```

**Alternatives considered:**
- **iOSSnapshotTestCase** (FBSnapshotTestCase): Rejected because less actively maintained
- **Manual screenshot comparison**: Rejected because too manual and error-prone
- **Xcode UI testing screenshots**: Rejected because slower and less precise

### Decision 5: ViewInspector for SwiftUI Testing

**Choice:** Use ViewInspector (~0.9.0) for inspecting SwiftUI view hierarchies in unit tests.

**Rationale:**
- **View introspection**: Can query view hierarchy without rendering
- **Faster than UI tests**: No need to launch app or render to screen
- **Complementary to snapshots**: Verify structure, not appearance
- **Good for logic**: Test button states, text content, conditional rendering

**Use cases:**
- Verify button enabled/disabled state based on validation
- Check that correct views are rendered conditionally
- Inspect view modifiers and bindings
- Test navigation and sheet presentation

**Limitations (and mitigations):**
- Not all SwiftUI APIs supported → Use snapshot tests for unsupported views
- Can be brittle with SwiftUI changes → Focus on public API, not internal implementation
- Requires view to be fully initialized → Use proper test fixtures

**Alternatives considered:**
- **Only XCUITest**: Rejected because too slow for comprehensive view testing
- **Only snapshot tests**: Rejected because can't verify non-visual behavior
- **Custom view testing framework**: Rejected because reinventing the wheel

### Decision 6: Test Data Isolation Strategy

**Choice:** Implement complete isolation between tests and between test/production environments.

**Isolation mechanisms:**

1. **MockDianeClient for unit/integration tests**
   - Never connects to real Unix socket
   - State reset between tests
   - Configurable responses per test

2. **UI tests detect test environment**
   - Check for "UI-Testing" launch argument
   - Use MockDianeClient in test mode
   - Separate app container directory

3. **Test fixtures in separate files**
   - `TestFixtures.swift`: Sample MCPServerConfig instances
   - `TestHelpers.swift`: Reusable test utilities
   - Each test creates fresh fixtures

**Implementation:**
```swift
// In app code (e.g., DianeMenuApp.swift)
@main
struct DianeMenuApp: App {
    let client: DianeClientProtocol
    
    init() {
        if ProcessInfo.processInfo.arguments.contains("UI-Testing") {
            self.client = MockDianeClient()
        } else {
            self.client = DianeClient.shared
        }
    }
}

// In UI tests
func testCreateServer() {
    let app = XCUIApplication()
    app.launchArguments = ["UI-Testing"]
    app.launch()
    // ... test steps
}
```

**Alternatives considered:**
- **Shared test database**: Rejected because causes test interdependencies
- **Environment variable for socket path**: Rejected because more complex and fragile
- **No isolation (use real daemon)**: Rejected because too slow and unreliable

### Decision 7: Xcode Target and Scheme Configuration

**Choice:** Create two test targets with separate schemes for different test types.

**Target structure:**
- **DianeMenuTests**: Unit, integration, and snapshot tests
  - Product type: Unit Test Bundle
  - Host application: DianeMenu
  - Dependencies: ViewInspector, SnapshotTesting
  - Scheme: Run on every test action (Cmd+U)
  - Code coverage: Enabled

- **DianeMenuUITests**: End-to-end UI automation tests
  - Product type: UI Test Bundle
  - Target application: DianeMenu
  - Dependencies: None (uses XCUITest built-in)
  - Scheme: Separate scheme or disabled by default
  - Parallel execution: Disabled (sequential for stability)

**Scheme configuration:**
```
DianeMenu scheme:
  - Test action: Runs DianeMenuTests (not UI tests)
  - Code coverage: Enabled
  - Parallelization: Enabled for unit tests
  
DianeMenuUITests scheme:
  - Test action: Runs only UI tests
  - Code coverage: Disabled (not useful for UI tests)
  - Parallelization: Disabled
```

**Rationale:**
- **Fast feedback**: Cmd+U runs only fast unit tests, not slow UI tests
- **Selective execution**: Developers can run UI tests separately when needed
- **CI flexibility**: CI can run unit tests first, then UI tests if unit tests pass
- **Coverage accuracy**: Code coverage on unit tests is more meaningful

**Alternatives considered:**
- **Single test target**: Rejected because can't easily separate fast/slow tests
- **Test tags for filtering**: Rejected because Xcode's tag support is limited
- **Always run all tests**: Rejected because too slow for quick iteration

### Decision 8: Test Organization and File Structure

**Choice:** Organize tests by feature area with clear naming conventions.

**Directory structure:**
```
DianeMenuTests/
├── README.md                          # Testing guide
├── TestHelpers/
│   ├── TestFixtures.swift             # Sample data
│   ├── MockDianeClient.swift          # Mock implementation
│   ├── AsyncTestHelpers.swift         # Async utilities
│   └── ViewTestHelpers.swift          # SwiftUI test utilities
├── UnitTests/
│   ├── ViewModels/
│   │   ├── MCPServersViewModelTests.swift
│   │   └── ServerFormViewModelTests.swift
│   └── Utilities/
│       ├── DuplicateNameGeneratorTests.swift
│       └── ValidationHelpersTests.swift
├── IntegrationTests/
│   ├── DianeClientProtocolTests.swift
│   └── MCPServerIntegrationTests.swift
├── SnapshotTests/
│   ├── MCPServersViewSnapshotTests.swift
│   └── ServerFormSnapshotTests.swift
└── __Snapshots__/                     # Generated by SnapshotTesting
    ├── MCPServersViewSnapshotTests/
    └── ServerFormSnapshotTests/

DianeMenuUITests/
├── README.md                          # UI testing guide
├── TestHelpers/
│   └── XCUIElementHelpers.swift       # UI test utilities
└── MCPServersUITests.swift
```

**Naming conventions:**
- Test class: `<Feature><Type>Tests` (e.g., `MCPServersViewModelTests`, `MCPServersUITests`)
- Test method: `test<Feature>_<Scenario>` (e.g., `testDuplicateServer_AddsNumberSuffix`)
- Fixtures: `make<Entity>()` (e.g., `makeMCPServerConfig()`)
- Mocks: `Mock<Protocol>` (e.g., `MockDianeClient`)

**Alternatives considered:**
- **Flat structure**: Rejected because hard to navigate with many tests
- **Mirror app structure exactly**: Rejected because tests have different grouping needs
- **Separate repos for tests**: Rejected because unnecessary complexity

## Risks / Trade-offs

### Risk: Refactoring to MVVM may introduce bugs
**Impact:** Medium  
**Likelihood:** Low  
**Mitigation:**
- Refactor incrementally, starting with MCP Servers view
- Run app manually after each refactoring step to verify behavior
- Use snapshot tests to catch visual regressions
- Keep original view structure and just extract logic

### Risk: Snapshot tests may be flaky across different environments
**Impact:** Medium  
**Likelihood:** Medium  
**Mitigation:**
- Use fixed window sizes and font settings
- Document required macOS version for consistent rendering
- Configure CI to use same OS version as development machines
- Accept minor rendering differences with tolerance threshold
- Regenerate snapshots when upgrading OS or dependencies

### Risk: UI tests may be slow and flaky
**Impact:** Low  
**Likelihood:** High  
**Mitigation:**
- Keep UI test count low (< 10 tests, only critical paths)
- Use explicit waits with generous timeouts
- Run UI tests in separate scheme, not on every test run
- Implement retry logic for flaky tests in CI
- Focus unit/integration tests on exhaustive coverage

### Risk: Protocol abstraction adds indirection
**Impact:** Low  
**Likelihood:** N/A (expected trade-off)  
**Mitigation:**
- Protocol is simple and mirrors existing DianeClient API
- Only one production implementation, no over-engineering
- Benefits (testability) outweigh cost (one extra protocol)

### Risk: Developers may skip writing tests
**Impact:** High  
**Likelihood:** Medium  
**Mitigation:**
- Provide clear examples and templates for common test types
- Document testing expectations in README
- Show value by catching bugs early with existing tests
- Consider CI enforcement (tests must pass before merge)

### Risk: Test maintenance burden
**Impact:** Medium  
**Likelihood:** Medium  
**Mitigation:**
- Focus on testing behavior, not implementation details
- Use fixtures and helpers to reduce duplication
- Snapshot tests require re-recording on intentional UI changes (this is expected)
- Keep UI tests minimal to reduce brittleness

### Trade-off: Code coverage vs. test execution speed
**Choice:** Prioritize fast execution for unit tests, accept slower UI tests for critical paths  
**Rationale:** Fast tests encourage frequent execution; slow but comprehensive UI tests run less often but catch integration issues

### Trade-off: Test isolation vs. realistic scenarios
**Choice:** Prioritize isolation with mocks, supplement with limited UI tests for realism  
**Rationale:** Isolated tests are reliable and fast; realistic scenarios covered by UI tests and manual QA

## Migration Plan

### Phase 1: Infrastructure Setup (Low risk)
1. Add DianeMenuTests and DianeMenuUITests targets to Xcode project
2. Add Swift Package Manager dependencies: ViewInspector, SnapshotTesting
3. Configure test schemes with code coverage enabled
4. Create test helper files: TestFixtures.swift, MockDianeClient.swift
5. Write README documentation for testing

**Validation:** Build succeeds, empty test targets run successfully

### Phase 2: Protocol Extraction (Low risk, non-breaking)
1. Define DianeClientProtocol with all existing DianeClient methods
2. Make DianeClient conform to protocol (no implementation changes)
3. Update ViewModels to accept DianeClientProtocol instead of concrete client
4. Default to DianeClient.shared to maintain existing behavior
5. Implement MockDianeClient with configurable responses

**Validation:** App builds and runs without changes, existing functionality works

### Phase 3: ViewModel Extraction (Medium risk)
1. Extract MCPServersViewModel from MCPServersView
2. Move state (@State properties) to ViewModel
3. Move methods (loadServers, createServer, etc.) to ViewModel
4. Update view to use @State var viewModel
5. Inject DianeClientProtocol into ViewModel
6. Test manually to verify behavior unchanged

**Validation:** MCP Servers view works identically to before, snapshot test captures baseline

### Phase 4: Write Tests (Low risk)
1. Write unit tests for MCPServersViewModel with MockDianeClient
2. Write unit tests for pure functions (duplicate name generation, validation)
3. Write integration tests for DianeClient protocol conformance
4. Write snapshot tests for MCPServersView (empty, loaded, error states)
5. Write UI tests for create, edit, duplicate, delete flows
6. Achieve >80% code coverage for ViewModels and business logic

**Validation:** All tests pass, coverage meets target

### Phase 5: CI Integration (Low risk)
1. Add test execution to CI pipeline (if exists)
2. Configure snapshot test artifact upload on failure
3. Enable code coverage reporting
4. Document CI test execution in README

**Validation:** CI runs tests successfully, failures block merges

### Rollback Strategy
- Each phase is independently reversible
- Phase 2 (protocol): Remove protocol, revert to concrete DianeClient
- Phase 3 (ViewModels): Move logic back into views, delete ViewModels
- Phase 4 (tests): Delete test targets (no impact on app)
- Git history preserves pre-refactoring state

## Open Questions

1. **Should we add ViewModels to all views or just MCP Servers view?**
   - Recommendation: Start with MCP Servers (most complex), expand later if valuable
   - Decision needed before implementation begins

2. **What code coverage threshold should we enforce?**
   - Recommendation: 80% for ViewModels, 60% overall (excluding UI code)
   - Decision needed for CI configuration

3. **Should UI tests run on every PR or only before releases?**
   - Recommendation: Run on every PR but don't block merge if flaky; investigate and fix flakes
   - Decision needed for CI configuration

4. **Do we need screenshot comparison testing in addition to snapshot tests?**
   - Recommendation: No, SnapshotTesting provides sufficient visual regression coverage
   - Can revisit if SnapshotTesting proves inadequate

5. **Should we test other features (Agents, Contexts, Providers) or focus on MCP Servers?**
   - Recommendation: Focus on MCP Servers initially, expand to other features after pattern is proven
   - Decision needed before claiming "comprehensive" test suite
