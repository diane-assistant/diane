## ADDED Requirements

### Requirement: All ViewModel-backed views accept injectable ViewModel
Every SwiftUI view that uses a ViewModel SHALL accept an optional ViewModel parameter in its initializer, defaulting to a fresh instance, so that external code can inject a pre-configured ViewModel.

#### Scenario: MCPServersView already injectable
- **WHEN** MCPServersView is instantiated
- **THEN** it accepts `init(viewModel: MCPServersViewModel = MCPServersViewModel())` (already implemented)

#### Scenario: AgentsView becomes injectable
- **WHEN** AgentsView is instantiated with `AgentsView(viewModel: someViewModel)`
- **THEN** it uses the provided ViewModel instead of creating its own

#### Scenario: ProvidersView becomes injectable
- **WHEN** ProvidersView is instantiated with `ProvidersView(viewModel: someViewModel)`
- **THEN** it uses the provided ViewModel instead of creating its own

#### Scenario: ContextsView becomes injectable
- **WHEN** ContextsView is instantiated with `ContextsView(viewModel: someViewModel)`
- **THEN** it uses the provided ViewModel instead of creating its own

#### Scenario: SchedulerView becomes injectable
- **WHEN** SchedulerView is instantiated with `SchedulerView(viewModel: someViewModel)`
- **THEN** it uses the provided ViewModel instead of creating its own

#### Scenario: UsageView becomes injectable
- **WHEN** UsageView is instantiated with `UsageView(viewModel: someViewModel)`
- **THEN** it uses the provided ViewModel instead of creating its own

#### Scenario: ToolsBrowserView becomes injectable
- **WHEN** ToolsBrowserView is instantiated with `ToolsBrowserView(viewModel: someViewModel)`
- **THEN** it uses the provided ViewModel instead of creating its own

### Requirement: Default parameter preserves existing call sites
The default parameter value SHALL ensure that all existing call sites (e.g., `MCPServersView()` with no arguments) continue to compile and behave identically.

#### Scenario: No-argument construction unchanged
- **WHEN** a view is instantiated with no arguments (e.g., `AgentsView()`)
- **THEN** it creates a fresh ViewModel with a real DianeClient, identical to the current behavior

#### Scenario: MainWindowView unchanged
- **WHEN** MainWindowView instantiates its child views with no arguments
- **THEN** compilation succeeds without any changes to MainWindowView

### Requirement: Injectable init uses State wrapping pattern
The injectable init SHALL use the `_viewModel = State(initialValue: viewModel)` pattern to properly initialize the `@State` property from an external value.

#### Scenario: State initialization
- **WHEN** a view is created with `init(viewModel:)`
- **THEN** the `@State private var viewModel` property is initialized via `_viewModel = State(initialValue: viewModel)`

#### Scenario: ViewModel lifecycle
- **WHEN** the view is displayed and its body is evaluated
- **THEN** the injected ViewModel is used (not replaced by SwiftUI's state management) for the lifetime of the view
