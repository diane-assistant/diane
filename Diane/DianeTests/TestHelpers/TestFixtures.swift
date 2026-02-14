import Foundation
#if !COMPONENT_CATALOG
@testable import Diane
#endif

/// Factory functions for creating test data. All dates use a fixed reference
/// to keep snapshot/assertion output deterministic.
enum TestFixtures {

    /// A fixed reference date: 2025-01-15T12:00:00Z
    static let referenceDate = ISO8601DateFormatter().date(from: "2025-01-15T12:00:00Z")!

    // MARK: - MCPServer Factories

    /// Create a stdio-type MCP server with sensible defaults.
    static func makeStdioServer(
        id: Int64 = 1,
        name: String = "test-server",
        enabled: Bool = true,
        command: String = "/usr/local/bin/mcp-server",
        args: [String]? = ["--verbose"],
        env: [String: String]? = nil,
        createdAt: Date? = nil,
        updatedAt: Date? = nil
    ) -> MCPServer {
        MCPServer(
            id: id,
            name: name,
            enabled: enabled,
            type: "stdio",
            command: command,
            args: args,
            env: env,
            url: nil,
            headers: nil,
            oauth: nil,
            createdAt: createdAt ?? referenceDate,
            updatedAt: updatedAt ?? referenceDate
        )
    }

    /// Create an SSE-type MCP server.
    static func makeSSEServer(
        id: Int64 = 2,
        name: String = "sse-server",
        enabled: Bool = true,
        url: String = "https://example.com/mcp",
        headers: [String: String]? = nil,
        createdAt: Date? = nil,
        updatedAt: Date? = nil
    ) -> MCPServer {
        MCPServer(
            id: id,
            name: name,
            enabled: enabled,
            type: "sse",
            command: nil,
            args: nil,
            env: nil,
            url: url,
            headers: headers,
            oauth: nil,
            createdAt: createdAt ?? referenceDate,
            updatedAt: updatedAt ?? referenceDate
        )
    }

    /// Create an HTTP-type MCP server.
    static func makeHTTPServer(
        id: Int64 = 3,
        name: String = "http-server",
        enabled: Bool = true,
        url: String = "https://example.com/mcp/http",
        headers: [String: String]? = ["Authorization": "Bearer test"],
        createdAt: Date? = nil,
        updatedAt: Date? = nil
    ) -> MCPServer {
        MCPServer(
            id: id,
            name: name,
            enabled: enabled,
            type: "http",
            command: nil,
            args: nil,
            env: nil,
            url: url,
            headers: headers,
            oauth: nil,
            createdAt: createdAt ?? referenceDate,
            updatedAt: updatedAt ?? referenceDate
        )
    }

    /// Create a builtin-type MCP server.
    static func makeBuiltinServer(
        id: Int64 = 4,
        name: String = "builtin-server",
        enabled: Bool = true,
        createdAt: Date? = nil,
        updatedAt: Date? = nil
    ) -> MCPServer {
        MCPServer(
            id: id,
            name: name,
            enabled: enabled,
            type: "builtin",
            command: nil,
            args: nil,
            env: nil,
            url: nil,
            headers: nil,
            oauth: nil,
            createdAt: createdAt ?? referenceDate,
            updatedAt: updatedAt ?? referenceDate
        )
    }

    /// A standard set of mixed-type servers for list tests.
    static func makeMixedServerList() -> [MCPServer] {
        [
            makeStdioServer(id: 1, name: "node-mcp"),
            makeSSEServer(id: 2, name: "cloud-mcp"),
            makeHTTPServer(id: 3, name: "api-mcp"),
            makeBuiltinServer(id: 4, name: "builtin-tools"),
            makeStdioServer(id: 5, name: "disabled-server", enabled: false),
        ]
    }

    // MARK: - Job Factories

    /// Create a Job with sensible defaults.
    static func makeJob(
        id: Int64 = 1,
        name: String = "backup-db",
        command: String = "/usr/local/bin/backup",
        schedule: String = "0 2 * * *",
        enabled: Bool = true,
        actionType: String? = nil,
        agentName: String? = nil,
        createdAt: Date? = nil,
        updatedAt: Date? = nil
    ) -> Job {
        Job(
            id: id,
            name: name,
            command: command,
            schedule: schedule,
            enabled: enabled,
            actionType: actionType,
            agentName: agentName,
            createdAt: createdAt ?? referenceDate,
            updatedAt: updatedAt ?? referenceDate
        )
    }

    /// Create an agent-type Job.
    static func makeAgentJob(
        id: Int64 = 10,
        name: String = "daily-report",
        command: String = "Generate daily report",
        schedule: String = "0 9 * * 1-5",
        enabled: Bool = true,
        agentName: String = "report-agent"
    ) -> Job {
        makeJob(
            id: id,
            name: name,
            command: command,
            schedule: schedule,
            enabled: enabled,
            actionType: "agent",
            agentName: agentName
        )
    }

    /// Create a JobExecution with sensible defaults.
    static func makeJobExecution(
        id: Int64 = 1,
        jobID: Int64 = 1,
        jobName: String? = "backup-db",
        startedAt: Date? = nil,
        endedAt: Date? = nil,
        exitCode: Int? = 0,
        stdout: String? = nil,
        stderr: String? = nil,
        error: String? = nil
    ) -> JobExecution {
        let start = startedAt ?? referenceDate
        let end = endedAt ?? start.addingTimeInterval(5.0)
        return JobExecution(
            id: id,
            jobID: jobID,
            jobName: jobName,
            startedAt: start,
            endedAt: end,
            exitCode: exitCode,
            stdout: stdout,
            stderr: stderr,
            error: error
        )
    }

    /// A standard set of jobs for list tests.
    static func makeJobList() -> [Job] {
        [
            makeJob(id: 1, name: "backup-db", command: "/usr/local/bin/backup", schedule: "0 2 * * *"),
            makeJob(id: 2, name: "cleanup-logs", command: "/usr/local/bin/cleanup", schedule: "0 0 * * 0"),
            makeAgentJob(id: 3, name: "daily-report", schedule: "0 9 * * 1-5"),
            makeJob(id: 4, name: "disabled-job", command: "/usr/local/bin/noop", enabled: false),
        ]
    }

    /// A standard set of executions for log tests.
    static func makeExecutionList() -> [JobExecution] {
        let base = referenceDate
        return [
            makeJobExecution(id: 1, jobID: 1, jobName: "backup-db", startedAt: base, exitCode: 0),
            makeJobExecution(id: 2, jobID: 1, jobName: "backup-db", startedAt: base.addingTimeInterval(-3600), exitCode: 0),
            makeJobExecution(id: 3, jobID: 2, jobName: "cleanup-logs", startedAt: base.addingTimeInterval(-7200), exitCode: 1, error: "Permission denied"),
            makeJobExecution(id: 4, jobID: 3, jobName: "daily-report", startedAt: base.addingTimeInterval(-86400), exitCode: 0),
        ]
    }

    // MARK: - Usage Factories

    /// Create a UsageSummaryRecord with sensible defaults.
    static func makeUsageSummaryRecord(
        providerID: Int64 = 1,
        providerName: String = "openai",
        service: String = "openai",
        model: String = "gpt-4o",
        totalRequests: Int = 10,
        totalInput: Int = 5000,
        totalOutput: Int = 2000,
        totalCached: Int = 0,
        totalCost: Double = 0.50
    ) -> UsageSummaryRecord {
        UsageSummaryRecord(
            providerID: providerID,
            providerName: providerName,
            service: service,
            model: model,
            totalRequests: totalRequests,
            totalInput: totalInput,
            totalOutput: totalOutput,
            totalCached: totalCached,
            totalCost: totalCost
        )
    }

    /// Create a UsageRecord with sensible defaults.
    static func makeUsageRecord(
        id: Int64 = 1,
        providerID: Int64 = 1,
        providerName: String = "openai",
        service: String = "openai",
        model: String = "gpt-4o",
        inputTokens: Int = 500,
        outputTokens: Int = 200,
        cachedTokens: Int = 0,
        cost: Double = 0.05,
        createdAt: Date? = nil
    ) -> UsageRecord {
        UsageRecord(
            id: id,
            providerID: providerID,
            providerName: providerName,
            service: service,
            model: model,
            inputTokens: inputTokens,
            outputTokens: outputTokens,
            cachedTokens: cachedTokens,
            cost: cost,
            createdAt: createdAt ?? referenceDate
        )
    }

    /// A standard set of summary records for usage tests.
    static func makeUsageSummaryList() -> [UsageSummaryRecord] {
        [
            makeUsageSummaryRecord(providerID: 1, providerName: "openai", model: "gpt-4o", totalRequests: 10, totalInput: 5000, totalOutput: 2000, totalCost: 0.50),
            makeUsageSummaryRecord(providerID: 1, providerName: "openai", model: "gpt-4o-mini", totalRequests: 25, totalInput: 12000, totalOutput: 4000, totalCost: 0.10),
            makeUsageSummaryRecord(providerID: 2, providerName: "anthropic", service: "anthropic", model: "claude-sonnet-4-20250514", totalRequests: 5, totalInput: 3000, totalOutput: 1500, totalCost: 0.30),
        ]
    }

    /// A standard UsageSummaryResponse for usage tests.
    static func makeUsageSummaryResponse() -> UsageSummaryResponse {
        let records = makeUsageSummaryList()
        return UsageSummaryResponse(
            summary: records,
            totalCost: records.reduce(0) { $0 + $1.totalCost },
            from: referenceDate.addingTimeInterval(-86400 * 30),
            to: referenceDate
        )
    }

    /// A standard set of usage records for recent activity tests.
    static func makeUsageRecordList() -> [UsageRecord] {
        let base = referenceDate
        return [
            makeUsageRecord(id: 1, providerID: 1, providerName: "openai", model: "gpt-4o", inputTokens: 500, outputTokens: 200, cost: 0.05, createdAt: base),
            makeUsageRecord(id: 2, providerID: 1, providerName: "openai", model: "gpt-4o-mini", inputTokens: 1000, outputTokens: 300, cost: 0.01, createdAt: base.addingTimeInterval(-60)),
            makeUsageRecord(id: 3, providerID: 2, providerName: "anthropic", service: "anthropic", model: "claude-sonnet-4-20250514", inputTokens: 300, outputTokens: 150, cost: 0.03, createdAt: base.addingTimeInterval(-120)),
        ]
    }

    /// A standard UsageResponse for usage tests.
    static func makeUsageResponse() -> UsageResponse {
        let records = makeUsageRecordList()
        return UsageResponse(
            records: records,
            totalCost: records.reduce(0) { $0 + $1.cost },
            from: referenceDate.addingTimeInterval(-86400 * 30),
            to: referenceDate
        )
    }

    // MARK: - ToolInfo Factories

    /// Create a ToolInfo with sensible defaults.
    static func makeTool(
        name: String = "read_file",
        description: String = "Read the contents of a file",
        server: String = "filesystem",
        builtin: Bool = false
    ) -> ToolInfo {
        ToolInfo(name: name, description: description, server: server, builtin: builtin)
    }

    /// A standard set of tools for list/filter tests.
    static func makeToolList() -> [ToolInfo] {
        [
            makeTool(name: "read_file", description: "Read the contents of a file", server: "filesystem"),
            makeTool(name: "write_file", description: "Write content to a file", server: "filesystem"),
            makeTool(name: "list_directory", description: "List directory contents", server: "filesystem"),
            makeTool(name: "web_search", description: "Search the web for information", server: "search-engine"),
            makeTool(name: "fetch_url", description: "Fetch content from a URL", server: "search-engine"),
            makeTool(name: "get_time", description: "Get current time and date", server: "builtin-tools", builtin: true),
            makeTool(name: "calculator", description: "Perform math calculations", server: "builtin-tools", builtin: true),
        ]
    }

    // MARK: - Provider Factories

    /// Create a Provider with sensible defaults.
    static func makeProvider(
        id: Int64 = 1,
        name: String = "my-openai",
        type: ProviderType = .llm,
        service: String = "openai",
        enabled: Bool = true,
        isDefault: Bool = false,
        authType: AuthType = .apiKey,
        authConfig: [String: AnyCodable]? = nil,
        config: [String: AnyCodable] = ["model": AnyCodable("gpt-4o")],
        createdAt: Date? = nil,
        updatedAt: Date? = nil
    ) -> Provider {
        Provider(
            id: id,
            name: name,
            type: type,
            service: service,
            enabled: enabled,
            isDefault: isDefault,
            authType: authType,
            authConfig: authConfig,
            config: config,
            createdAt: createdAt ?? referenceDate,
            updatedAt: updatedAt ?? referenceDate
        )
    }

    /// Create an LLM provider (OpenAI).
    static func makeLLMProvider(
        id: Int64 = 1,
        name: String = "my-openai",
        enabled: Bool = true,
        isDefault: Bool = false
    ) -> Provider {
        makeProvider(id: id, name: name, type: .llm, service: "openai", enabled: enabled, isDefault: isDefault, authType: .apiKey, config: ["model": AnyCodable("gpt-4o")])
    }

    /// Create an embedding provider (Vertex AI).
    static func makeEmbeddingProvider(
        id: Int64 = 2,
        name: String = "my-vertex-embed",
        enabled: Bool = true,
        isDefault: Bool = false
    ) -> Provider {
        makeProvider(id: id, name: name, type: .embedding, service: "vertex_ai", enabled: enabled, isDefault: isDefault, authType: .oauth, config: ["project_id": AnyCodable("my-project"), "region": AnyCodable("us-central1")])
    }

    /// Create a storage provider.
    static func makeStorageProvider(
        id: Int64 = 3,
        name: String = "my-storage",
        enabled: Bool = true
    ) -> Provider {
        makeProvider(id: id, name: name, type: .storage, service: "local_storage", enabled: enabled, authType: .none, config: [:])
    }

    /// A standard set of mixed providers for list tests.
    static func makeProviderList() -> [Provider] {
        [
            makeLLMProvider(id: 1, name: "openai-main", isDefault: true),
            makeLLMProvider(id: 2, name: "openai-backup"),
            makeEmbeddingProvider(id: 3, name: "vertex-embed"),
            makeStorageProvider(id: 4, name: "local-store"),
            makeLLMProvider(id: 5, name: "disabled-llm", enabled: false),
        ]
    }

    // MARK: - ConfigField / ProviderTemplate Factories

    /// Create a ConfigField with sensible defaults.
    static func makeConfigField(
        key: String = "model",
        label: String = "Model",
        type: String = "string",
        required: Bool = true,
        defaultValue: AnyCodable? = nil,
        options: [String]? = nil,
        description: String? = nil
    ) -> ConfigField {
        ConfigField(
            key: key,
            label: label,
            type: type,
            required: required,
            defaultValue: defaultValue,
            options: options,
            description: description
        )
    }

    /// Create a ProviderTemplate with sensible defaults.
    static func makeProviderTemplate(
        service: String = "openai",
        name: String = "OpenAI",
        type: ProviderType = .llm,
        authType: AuthType = .apiKey,
        oauthScopes: [String]? = nil,
        configSchema: [ConfigField]? = nil,
        description: String = "OpenAI LLM provider"
    ) -> ProviderTemplate {
        let schema = configSchema ?? [
            makeConfigField(key: "model", label: "Model", required: true, defaultValue: AnyCodable("gpt-4o"), options: ["gpt-4o", "gpt-4o-mini"]),
        ]
        return ProviderTemplate(
            service: service,
            name: name,
            type: type,
            authType: authType,
            oauthScopes: oauthScopes,
            configSchema: schema,
            description: description
        )
    }

    /// Create a Vertex AI template (OAuth, embedding).
    static func makeVertexAITemplate() -> ProviderTemplate {
        makeProviderTemplate(
            service: "vertex_ai",
            name: "Vertex AI",
            type: .embedding,
            authType: .oauth,
            oauthScopes: ["https://www.googleapis.com/auth/cloud-platform"],
            configSchema: [
                makeConfigField(key: "project_id", label: "Project ID", required: true),
                makeConfigField(key: "region", label: "Region", required: true, defaultValue: AnyCodable("us-central1")),
            ],
            description: "Google Vertex AI embedding provider"
        )
    }

    /// A standard set of templates for create-provider tests.
    static func makeProviderTemplateList() -> [ProviderTemplate] {
        [
            makeProviderTemplate(), // OpenAI (LLM, apiKey)
            makeVertexAITemplate(), // Vertex AI (Embedding, oauth)
            makeProviderTemplate(
                service: "ollama",
                name: "Ollama",
                type: .llm,
                authType: .none,
                configSchema: [
                    makeConfigField(key: "model", label: "Model", required: true, defaultValue: AnyCodable("llama3")),
                    makeConfigField(key: "base_url", label: "Base URL", required: false, defaultValue: AnyCodable("http://localhost:11434")),
                ],
                description: "Local Ollama LLM"
            ),
        ]
    }

    // MARK: - AvailableModel Factories

    /// Create an AvailableModel with sensible defaults.
    static func makeAvailableModel(
        id: String = "gpt-4o",
        name: String = "gpt-4o",
        displayName: String? = nil,
        launchStage: String? = "GA",
        family: String? = nil,
        toolCall: Bool? = true,
        reasoning: Bool? = false
    ) -> AvailableModel {
        AvailableModel(
            id: id,
            name: name,
            displayName: displayName,
            launchStage: launchStage,
            family: family,
            toolCall: toolCall,
            reasoning: reasoning
        )
    }

    /// A standard set of available models for model-discovery tests.
    static func makeAvailableModelList() -> [AvailableModel] {
        [
            makeAvailableModel(id: "gpt-4o", name: "gpt-4o"),
            makeAvailableModel(id: "gpt-4o-mini", name: "gpt-4o-mini"),
            makeAvailableModel(id: "gpt-3.5-turbo", name: "gpt-3.5-turbo", launchStage: "GA", toolCall: false),
        ]
    }

    // MARK: - AgentConfig Factories

    /// Create an AgentConfig with sensible defaults.
    static func makeAgentConfig(
        name: String = "test-agent",
        url: String? = "http://localhost:4321",
        type: String? = "acp",
        command: String? = nil,
        args: [String]? = nil,
        env: [String: String]? = nil,
        workdir: String? = nil,
        port: Int? = nil,
        subAgent: String? = nil,
        enabled: Bool = true,
        description: String? = "A test agent",
        tags: [String]? = nil
    ) -> AgentConfig {
        AgentConfig(
            name: name, url: url, type: type,
            command: command, args: args, env: env,
            workdir: workdir, port: port, subAgent: subAgent,
            enabled: enabled, description: description, tags: tags
        )
    }

    /// A standard set of agents for list tests.
    static func makeAgentList() -> [AgentConfig] {
        [
            makeAgentConfig(name: "gemini", url: "http://localhost:4321", enabled: true, description: "Gemini agent"),
            makeAgentConfig(name: "opencode", url: "http://localhost:4322", enabled: true, description: "OpenCode agent"),
            makeAgentConfig(name: "claude", url: "http://localhost:4323", enabled: false, description: "Claude agent"),
            makeAgentConfig(name: "worker@project", url: "http://localhost:4324", workdir: "/tmp/project", enabled: true, description: "Worker agent"),
        ]
    }

    // MARK: - AgentTestResult Factories

    /// Create an AgentTestResult with sensible defaults.
    static func makeAgentTestResult(
        name: String = "test-agent",
        url: String? = "http://localhost:4321",
        workdir: String? = nil,
        enabled: Bool = true,
        status: String = "connected",
        error: String? = nil,
        version: String? = "1.0.0",
        agentCount: Int? = 1,
        agents: [String]? = nil
    ) -> AgentTestResult {
        AgentTestResult(
            name: name, url: url, workdir: workdir,
            enabled: enabled, status: status, error: error,
            version: version, agentCount: agentCount, agents: agents
        )
    }

    // MARK: - AgentLog Factories

    /// Create an AgentLog with sensible defaults.
    static func makeAgentLog(
        id: Int64 = 1,
        agentName: String = "test-agent",
        direction: String = "request",
        messageType: String = "run",
        content: String? = "{\"prompt\": \"hello\"}",
        error: String? = nil,
        timestamp: Date? = nil,
        durationMs: Int? = 150
    ) -> AgentLog {
        AgentLog(
            id: id, agentName: agentName,
            direction: direction, messageType: messageType,
            content: content, error: error,
            timestamp: timestamp ?? referenceDate,
            durationMs: durationMs
        )
    }

    /// A standard set of logs for agent log tests.
    static func makeAgentLogList(agentName: String = "test-agent") -> [AgentLog] {
        let base = referenceDate
        return [
            makeAgentLog(id: 1, agentName: agentName, direction: "request", messageType: "run", timestamp: base, durationMs: nil),
            makeAgentLog(id: 2, agentName: agentName, direction: "response", messageType: "run", timestamp: base.addingTimeInterval(0.15), durationMs: 150),
            makeAgentLog(id: 3, agentName: agentName, direction: "request", messageType: "ping", timestamp: base.addingTimeInterval(-60), durationMs: nil),
            makeAgentLog(id: 4, agentName: agentName, direction: "response", messageType: "ping", timestamp: base.addingTimeInterval(-59.95), durationMs: 50),
        ]
    }

    // MARK: - AgentRunResult Factories

    /// Create a successful AgentRunResult.
    static func makeAgentRunResult(
        agentName: String = "test-agent",
        sessionId: String? = "session-123",
        runId: String = "run-456",
        status: String = "completed",
        awaitRequest: String? = nil,
        output: [AgentMessage] = [],
        error: AgentError? = nil,
        createdAt: Date? = nil,
        finishedAt: Date? = nil
    ) -> AgentRunResult {
        AgentRunResult(
            agentName: agentName, sessionId: sessionId,
            runId: runId, status: status, awaitRequest: awaitRequest,
            output: output, error: error,
            createdAt: createdAt ?? referenceDate,
            finishedAt: finishedAt ?? referenceDate.addingTimeInterval(1)
        )
    }

    /// Create an AgentRunResult with text output.
    static func makeAgentRunResultWithOutput(
        agentName: String = "test-agent",
        text: String = "Hello! I'm the agent."
    ) -> AgentRunResult {
        let part = AgentMessagePart(name: nil, contentType: "text/plain", content: text, contentEncoding: nil, contentUrl: nil)
        let message = AgentMessage(role: "agent", parts: [part], createdAt: referenceDate, completedAt: referenceDate.addingTimeInterval(1))
        return makeAgentRunResult(agentName: agentName, output: [message])
    }

    /// Create a failed AgentRunResult.
    static func makeAgentRunResultFailed(
        agentName: String = "test-agent",
        errorCode: String = "agent_error",
        errorMessage: String = "Something went wrong"
    ) -> AgentRunResult {
        let err = AgentError(code: errorCode, message: errorMessage, data: nil)
        return makeAgentRunResult(agentName: agentName, status: "failed", error: err)
    }

    // MARK: - GalleryEntry Factories

    /// Create a GalleryEntry with sensible defaults.
    static func makeGalleryEntry(
        entryId: String = "gemini",
        name: String = "Gemini",
        description: String = "Google Gemini agent",
        icon: String? = nil,
        category: String = "llm",
        provider: String = "google",
        installType: String = "acp",
        tags: [String]? = ["featured", "llm"],
        featured: Bool = false
    ) -> GalleryEntry {
        GalleryEntry(
            entryId: entryId, name: name, description: description,
            icon: icon, category: category, provider: provider,
            installType: installType, tags: tags, featured: featured
        )
    }

    /// A standard set of gallery entries for gallery tests.
    static func makeGalleryEntryList() -> [GalleryEntry] {
        [
            makeGalleryEntry(entryId: "gemini", name: "Gemini", description: "Google Gemini agent", provider: "google", featured: true),
            makeGalleryEntry(entryId: "opencode", name: "OpenCode", description: "OpenCode ACP agent", provider: "openai", installType: "acp"),
            makeGalleryEntry(entryId: "claude", name: "Claude", description: "Anthropic Claude agent", provider: "anthropic"),
        ]
    }

    // MARK: - GalleryInstallResponse Factories

    /// Create a GalleryInstallResponse with sensible defaults.
    static func makeGalleryInstallResponse(
        status: String = "installed",
        agent: String = "gemini",
        installCmd: String? = nil
    ) -> GalleryInstallResponse {
        GalleryInstallResponse(status: status, agent: agent, installCmd: installCmd)
    }

    // MARK: - RemoteAgentInfo Factories

    /// Create a RemoteAgentInfo with sensible defaults.
    static func makeRemoteAgentInfo(
        configId: String = "sub-1",
        name: String = "default-model",
        description: String? = "The default model configuration",
        options: [String]? = ["gpt-4o", "gpt-4o-mini", "claude-sonnet"]
    ) -> RemoteAgentInfo {
        RemoteAgentInfo(
            configId: configId, name: name,
            description: description, options: options
        )
    }

    /// A standard set of remote agents for sub-agent tests.
    static func makeRemoteAgentList() -> [RemoteAgentInfo] {
        [
            makeRemoteAgentInfo(configId: "sub-1", name: "default-model", options: ["gpt-4o", "gpt-4o-mini"]),
            makeRemoteAgentInfo(configId: "sub-2", name: "fallback-model", description: "Fallback model", options: ["claude-sonnet"]),
        ]
    }

    // MARK: - Context Factories

    /// Create a Context with sensible defaults.
    static func makeContext(
        id: Int64 = 1,
        name: String = "default",
        description: String? = "Default context",
        isDefault: Bool = true
    ) -> Context {
        Context(id: id, name: name, description: description, isDefault: isDefault)
    }

    /// A standard set of contexts for list tests.
    static func makeContextList() -> [Context] {
        [
            makeContext(id: 1, name: "default", description: "Default context", isDefault: true),
            makeContext(id: 2, name: "work", description: "Work projects", isDefault: false),
            makeContext(id: 3, name: "personal", description: nil, isDefault: false),
        ]
    }

    // MARK: - ContextServer Factories

    /// Create a ContextServer with sensible defaults.
    static func makeContextServer(
        id: Int64 = 1,
        name: String = "test-server",
        type: String = "stdio",
        enabled: Bool = true,
        toolsActive: Int = 3,
        toolsTotal: Int = 5,
        tools: [ContextTool]? = nil
    ) -> ContextServer {
        ContextServer(id: id, name: name, type: type, enabled: enabled, toolsActive: toolsActive, toolsTotal: toolsTotal, tools: tools)
    }

    /// A standard set of context servers for detail tests.
    static func makeContextServerList() -> [ContextServer] {
        [
            makeContextServer(id: 1, name: "filesystem", type: "stdio", enabled: true, toolsActive: 3, toolsTotal: 3),
            makeContextServer(id: 2, name: "search-engine", type: "sse", enabled: true, toolsActive: 1, toolsTotal: 2),
            makeContextServer(id: 3, name: "disabled-server", type: "stdio", enabled: false, toolsActive: 0, toolsTotal: 4),
        ]
    }

    // MARK: - ContextTool Factories

    /// Create a ContextTool with sensible defaults.
    static func makeContextTool(
        name: String = "read_file",
        description: String? = "Read file contents",
        enabled: Bool = true
    ) -> ContextTool {
        ContextTool(name: name, description: description, enabled: enabled)
    }

    /// A standard set of context tools for tool tests.
    static func makeContextToolList() -> [ContextTool] {
        [
            makeContextTool(name: "read_file", description: "Read file contents", enabled: true),
            makeContextTool(name: "write_file", description: "Write to a file", enabled: true),
            makeContextTool(name: "delete_file", description: "Delete a file", enabled: false),
        ]
    }

    // MARK: - ContextSummary Factories

    /// Create a ContextSummary with sensible defaults.
    static func makeContextSummary(
        serversEnabled: Int = 2,
        serversTotal: Int = 3,
        toolsActive: Int = 4,
        toolsTotal: Int = 9
    ) -> ContextSummary {
        ContextSummary(serversEnabled: serversEnabled, serversTotal: serversTotal, toolsActive: toolsActive, toolsTotal: toolsTotal)
    }

    // MARK: - ContextDetail Factories

    /// Create a ContextDetail with sensible defaults.
    static func makeContextDetail(
        context: Context? = nil,
        servers: [ContextServer]? = nil,
        summary: ContextSummary? = nil
    ) -> ContextDetail {
        ContextDetail(
            context: context ?? makeContext(),
            servers: servers ?? makeContextServerList(),
            summary: summary ?? makeContextSummary()
        )
    }

    // MARK: - ContextConnectInfo Factories

    /// Create a ContextConnectInfo with sensible defaults.
    static func makeContextConnectInfo(
        context: String = "default",
        sseUrl: String = "http://localhost:8080/sse/default",
        sseExample: String? = "curl http://localhost:8080/sse/default",
        streamableUrl: String = "http://localhost:8080/mcp/default",
        streamableExample: String? = "curl -X POST http://localhost:8080/mcp/default",
        description: String? = "Default context connection"
    ) -> ContextConnectInfo {
        ContextConnectInfo(
            context: context,
            sse: ConnectionDetails(url: sseUrl, example: sseExample),
            streamable: ConnectionDetails(url: streamableUrl, example: streamableExample),
            description: description
        )
    }

    // MARK: - AvailableServer Factories

    /// Create an AvailableServer with sensible defaults.
    static func makeAvailableServer(
        name: String = "test-server",
        toolCount: Int = 5,
        inContext: Bool = false,
        builtin: Bool? = nil
    ) -> AvailableServer {
        AvailableServer(name: name, toolCount: toolCount, inContext: inContext, builtin: builtin)
    }

    /// A standard set of available servers for add-server tests.
    static func makeAvailableServerList() -> [AvailableServer] {
        [
            makeAvailableServer(name: "filesystem", toolCount: 3, inContext: true),
            makeAvailableServer(name: "search-engine", toolCount: 2, inContext: false),
            makeAvailableServer(name: "builtin-tools", toolCount: 5, inContext: false, builtin: true),
            makeAvailableServer(name: "database", toolCount: 4, inContext: false),
        ]
    }
}
