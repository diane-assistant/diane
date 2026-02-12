import Foundation

/// Represents a provider type (embedding, llm, storage)
enum ProviderType: String, Codable, CaseIterable {
    case embedding
    case llm
    case storage
    
    var displayName: String {
        switch self {
        case .embedding: return "Embedding"
        case .llm: return "LLM"
        case .storage: return "Storage"
        }
    }
    
    var icon: String {
        switch self {
        case .embedding: return "cube.transparent"
        case .llm: return "brain.head.profile"
        case .storage: return "externaldrive"
        }
    }
}

/// Authentication type for a provider
enum AuthType: String, Codable, CaseIterable {
    case none
    case apiKey = "api_key"
    case oauth
    
    var displayName: String {
        switch self {
        case .none: return "None"
        case .apiKey: return "API Key"
        case .oauth: return "OAuth"
        }
    }
}

/// Represents a configured service provider
struct Provider: Codable, Identifiable {
    let id: Int64
    var name: String
    let type: ProviderType
    let service: String
    var enabled: Bool
    var isDefault: Bool
    let authType: AuthType
    var authConfig: [String: AnyCodable]?
    var config: [String: AnyCodable]
    let createdAt: Date
    let updatedAt: Date
    
    enum CodingKeys: String, CodingKey {
        case id
        case name
        case type
        case service
        case enabled
        case isDefault = "is_default"
        case authType = "auth_type"
        case authConfig = "auth_config"
        case config
        case createdAt = "created_at"
        case updatedAt = "updated_at"
    }
    
    /// Memberwise initializer for programmatic construction (e.g. tests, mocks).
    init(
        id: Int64,
        name: String,
        type: ProviderType,
        service: String,
        enabled: Bool = true,
        isDefault: Bool = false,
        authType: AuthType = .none,
        authConfig: [String: AnyCodable]? = nil,
        config: [String: AnyCodable] = [:],
        createdAt: Date,
        updatedAt: Date
    ) {
        self.id = id
        self.name = name
        self.type = type
        self.service = service
        self.enabled = enabled
        self.isDefault = isDefault
        self.authType = authType
        self.authConfig = authConfig
        self.config = config
        self.createdAt = createdAt
        self.updatedAt = updatedAt
    }

    init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        id = try container.decode(Int64.self, forKey: .id)
        name = try container.decode(String.self, forKey: .name)
        
        // Decode type from string
        let typeStr = try container.decode(String.self, forKey: .type)
        type = ProviderType(rawValue: typeStr) ?? .embedding
        
        service = try container.decode(String.self, forKey: .service)
        enabled = try container.decodeIfPresent(Bool.self, forKey: .enabled) ?? true
        isDefault = try container.decodeIfPresent(Bool.self, forKey: .isDefault) ?? false
        
        // Decode auth type from string
        let authTypeStr = try container.decodeIfPresent(String.self, forKey: .authType) ?? "none"
        authType = AuthType(rawValue: authTypeStr) ?? .none
        
        authConfig = try container.decodeIfPresent([String: AnyCodable].self, forKey: .authConfig)
        config = try container.decodeIfPresent([String: AnyCodable].self, forKey: .config) ?? [:]
        createdAt = try container.decode(Date.self, forKey: .createdAt)
        updatedAt = try container.decode(Date.self, forKey: .updatedAt)
    }
    
    /// Get a config value as String
    func getConfigString(_ key: String) -> String {
        guard let value = config[key] else { return "" }
        if let str = value.value as? String {
            return str
        }
        return ""
    }
    
    /// Get an auth config value as String
    func getAuthString(_ key: String) -> String {
        guard let authConfig = authConfig, let value = authConfig[key] else { return "" }
        if let str = value.value as? String {
            return str
        }
        return ""
    }
    
    /// Human-readable service name
    var serviceName: String {
        switch service {
        case "vertex_ai": return "Vertex AI"
        case "openai": return "OpenAI"
        case "ollama": return "Ollama"
        default: return service.capitalized
        }
    }
    
    /// Icon for the service
    var serviceIcon: String {
        switch service {
        case "vertex_ai": return "cloud"
        case "openai": return "bolt.circle"
        case "ollama": return "desktopcomputer"
        default: return "gear"
        }
    }
    
    /// Status color
    var statusColor: String {
        if !enabled {
            return "gray"
        }
        return isDefault ? "green" : "blue"
    }
}

/// Configuration field schema for provider templates
struct ConfigField: Codable, Identifiable {
    let key: String
    let label: String
    let type: String  // string, int, bool, select
    let required: Bool
    let defaultValue: AnyCodable?
    let options: [String]?
    let description: String?
    
    var id: String { key }
    
    enum CodingKeys: String, CodingKey {
        case key
        case label
        case type
        case required
        case defaultValue = "default"
        case options
        case description
    }
    
    /// Memberwise initializer for programmatic construction (e.g. tests, mocks).
    init(
        key: String,
        label: String,
        type: String = "string",
        required: Bool = false,
        defaultValue: AnyCodable? = nil,
        options: [String]? = nil,
        description: String? = nil
    ) {
        self.key = key
        self.label = label
        self.type = type
        self.required = required
        self.defaultValue = defaultValue
        self.options = options
        self.description = description
    }

    init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        key = try container.decode(String.self, forKey: .key)
        label = try container.decode(String.self, forKey: .label)
        type = try container.decodeIfPresent(String.self, forKey: .type) ?? "string"
        required = try container.decodeIfPresent(Bool.self, forKey: .required) ?? false
        defaultValue = try container.decodeIfPresent(AnyCodable.self, forKey: .defaultValue)
        options = try container.decodeIfPresent([String].self, forKey: .options)
        description = try container.decodeIfPresent(String.self, forKey: .description)
    }
    
    /// Get default value as string
    var defaultString: String {
        guard let defaultValue = defaultValue else { return "" }
        if let str = defaultValue.value as? String {
            return str
        }
        if let num = defaultValue.value as? Int {
            return String(num)
        }
        return ""
    }
}

/// Template for creating a provider
struct ProviderTemplate: Codable, Identifiable {
    let service: String
    let name: String
    let type: ProviderType
    let authType: AuthType
    let oauthScopes: [String]?
    let configSchema: [ConfigField]
    let description: String
    
    var id: String { service }
    
    enum CodingKeys: String, CodingKey {
        case service
        case name
        case type
        case authType = "auth_type"
        case oauthScopes = "oauth_scopes"
        case configSchema = "config_schema"
        case description
    }
    
    /// Memberwise initializer for programmatic construction (e.g. tests, mocks).
    init(
        service: String,
        name: String,
        type: ProviderType,
        authType: AuthType = .none,
        oauthScopes: [String]? = nil,
        configSchema: [ConfigField] = [],
        description: String = ""
    ) {
        self.service = service
        self.name = name
        self.type = type
        self.authType = authType
        self.oauthScopes = oauthScopes
        self.configSchema = configSchema
        self.description = description
    }

    init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        service = try container.decode(String.self, forKey: .service)
        name = try container.decode(String.self, forKey: .name)
        
        let typeStr = try container.decode(String.self, forKey: .type)
        type = ProviderType(rawValue: typeStr) ?? .embedding
        
        let authTypeStr = try container.decodeIfPresent(String.self, forKey: .authType) ?? "none"
        authType = AuthType(rawValue: authTypeStr) ?? .none
        
        oauthScopes = try container.decodeIfPresent([String].self, forKey: .oauthScopes)
        configSchema = try container.decodeIfPresent([ConfigField].self, forKey: .configSchema) ?? []
        description = try container.decodeIfPresent(String.self, forKey: .description) ?? ""
    }
    
    /// Icon for the service
    var icon: String {
        switch service {
        case "vertex_ai": return "cloud"
        case "openai": return "bolt.circle"
        case "ollama": return "desktopcomputer"
        default: return "gear"
        }
    }
}

/// Request to create a new provider
struct CreateProviderRequest: Encodable {
    let name: String
    let service: String
    let type: String?
    let authType: String?
    let authConfig: [String: AnyCodable]?
    let config: [String: AnyCodable]
    
    enum CodingKeys: String, CodingKey {
        case name
        case service
        case type
        case authType = "auth_type"
        case authConfig = "auth_config"
        case config
    }
}

/// Request to update a provider
struct UpdateProviderRequest: Encodable {
    let name: String?
    let enabled: Bool?
    let isDefault: Bool?
    let authConfig: [String: AnyCodable]?
    let config: [String: AnyCodable]?
    
    enum CodingKeys: String, CodingKey {
        case name
        case enabled
        case isDefault = "is_default"
        case authConfig = "auth_config"
        case config
    }
}

/// Result of testing a provider
struct ProviderTestResult: Codable {
    let success: Bool
    let message: String
    let responseTimeMs: Double
    let details: [String: AnyCodable]?
    
    enum CodingKeys: String, CodingKey {
        case success
        case message
        case responseTimeMs = "response_time_ms"
        case details
    }
    
    /// Memberwise initializer for programmatic construction (e.g. tests, mocks).
    init(
        success: Bool,
        message: String,
        responseTimeMs: Double = 0,
        details: [String: AnyCodable]? = nil
    ) {
        self.success = success
        self.message = message
        self.responseTimeMs = responseTimeMs
        self.details = details
    }

    init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        success = try container.decode(Bool.self, forKey: .success)
        message = try container.decode(String.self, forKey: .message)
        responseTimeMs = try container.decodeIfPresent(Double.self, forKey: .responseTimeMs) ?? 0
        details = try container.decodeIfPresent([String: AnyCodable].self, forKey: .details)
    }
    
    /// Get a detail value as string
    func getDetail(_ key: String) -> String {
        guard let details = details, let value = details[key] else { return "" }
        if let str = value.value as? String {
            return str
        }
        if let num = value.value as? Int {
            return String(num)
        }
        if let num = value.value as? Double {
            return String(format: "%.0f", num)
        }
        return ""
    }
}

/// Model cost info (per 1M tokens)
struct ModelCost: Codable, Hashable {
    let input: Double
    let output: Double
    let cacheRead: Double?
    let cacheWrite: Double?
    
    enum CodingKeys: String, CodingKey {
        case input
        case output
        case cacheRead = "cache_read"
        case cacheWrite = "cache_write"
    }
    
    /// Format cost as currency string
    func formatCost(_ tokens: Int, isOutput: Bool) -> String {
        let rate = isOutput ? output : input
        let cost = Double(tokens) / 1_000_000.0 * rate
        return String(format: "$%.4f", cost)
    }
}

/// Model context/output limits
struct ModelLimits: Codable, Hashable {
    let context: Int
    let output: Int
    
    /// Format context limit as human-readable string (e.g., "1M", "128K")
    var contextFormatted: String {
        formatTokens(context)
    }
    
    /// Format output limit as human-readable string (e.g., "64K", "8K")
    var outputFormatted: String {
        formatTokens(output)
    }
    
    /// Format both limits together (e.g., "1M in / 64K out")
    var limitsFormatted: String {
        "\(contextFormatted) in / \(outputFormatted) out"
    }
    
    private func formatTokens(_ count: Int) -> String {
        if count >= 1_000_000 {
            let value = Double(count) / 1_000_000.0
            if value == Double(Int(value)) {
                return "\(Int(value))M"
            }
            return String(format: "%.1fM", value)
        } else if count >= 1_000 {
            let value = Double(count) / 1_000.0
            if value == Double(Int(value)) {
                return "\(Int(value))K"
            }
            return String(format: "%.1fK", value)
        }
        return "\(count)"
    }
}

/// Available model info from provider discovery
struct AvailableModel: Codable, Identifiable, Hashable {
    let id: String
    let name: String
    let displayName: String
    let launchStage: String?
    let family: String?
    let cost: ModelCost?
    let limits: ModelLimits?
    let toolCall: Bool?
    let reasoning: Bool?
    
    enum CodingKeys: String, CodingKey {
        case id
        case name
        case displayName = "display_name"
        case launchStage = "launch_stage"
        case family
        case cost
        case limits
        case toolCall = "tool_call"
        case reasoning
    }
    
    /// Memberwise initializer for programmatic construction (e.g. tests, mocks).
    init(
        id: String,
        name: String,
        displayName: String? = nil,
        launchStage: String? = nil,
        family: String? = nil,
        cost: ModelCost? = nil,
        limits: ModelLimits? = nil,
        toolCall: Bool? = nil,
        reasoning: Bool? = nil
    ) {
        self.id = id
        self.name = name
        self.displayName = displayName ?? name
        self.launchStage = launchStage
        self.family = family
        self.cost = cost
        self.limits = limits
        self.toolCall = toolCall
        self.reasoning = reasoning
    }

    init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        id = try container.decode(String.self, forKey: .id)
        name = try container.decode(String.self, forKey: .name)
        displayName = try container.decodeIfPresent(String.self, forKey: .displayName) ?? name
        launchStage = try container.decodeIfPresent(String.self, forKey: .launchStage)
        family = try container.decodeIfPresent(String.self, forKey: .family)
        cost = try container.decodeIfPresent(ModelCost.self, forKey: .cost)
        limits = try container.decodeIfPresent(ModelLimits.self, forKey: .limits)
        toolCall = try container.decodeIfPresent(Bool.self, forKey: .toolCall)
        reasoning = try container.decodeIfPresent(Bool.self, forKey: .reasoning)
    }
    
    /// Format pricing info for display
    var pricingInfo: String? {
        guard let cost = cost else { return nil }
        return "$\(String(format: "%.2f", cost.input))/$\(String(format: "%.2f", cost.output)) per 1M"
    }
}

/// Response containing available models
struct ListModelsResponse: Codable {
    let models: [AvailableModel]
}

// MARK: - Google OAuth Models

/// Google authentication status
struct GoogleAuthStatus: Codable {
    let authenticated: Bool
    let account: String
    let hasToken: Bool
    let hasCredentials: Bool
    let hasADC: Bool
    let usingADC: Bool
    let tokenPath: String?
    
    enum CodingKeys: String, CodingKey {
        case authenticated
        case account
        case hasToken = "has_token"
        case hasCredentials = "has_credentials"
        case hasADC = "has_adc"
        case usingADC = "using_adc"
        case tokenPath = "token_path"
    }
}

/// Response from starting the Google device flow
struct GoogleDeviceCodeResponse: Codable {
    let userCode: String
    let verificationURL: String
    let expiresIn: Int
    let interval: Int
    let deviceCode: String
    
    enum CodingKeys: String, CodingKey {
        case userCode = "user_code"
        case verificationURL = "verification_url"
        case expiresIn = "expires_in"
        case interval
        case deviceCode = "device_code"
    }
}

/// Request to start Google OAuth
struct GoogleAuthStartRequest: Encodable {
    let account: String?
    let scopes: [String]?
}

/// Request to poll for Google OAuth token
struct GoogleAuthPollRequest: Encodable {
    let account: String?
    let deviceCode: String
    let interval: Int
    
    enum CodingKeys: String, CodingKey {
        case account
        case deviceCode = "device_code"
        case interval
    }
}

/// Response from polling for Google OAuth token
struct GoogleAuthPollResponse: Codable {
    let status: String
    let message: String
    let account: String?
    let expires: Date?
    
    /// Whether authorization is still pending
    var isPending: Bool {
        status == "pending"
    }
    
    /// Whether authorization was successful
    var isSuccess: Bool {
        status == "success"
    }
    
    /// Whether we need to slow down polling
    var shouldSlowDown: Bool {
        status == "slow_down"
    }
    
    /// Whether the device code has expired
    var isExpired: Bool {
        status == "expired"
    }
    
    /// Whether authorization was denied
    var isDenied: Bool {
        status == "denied"
    }
}

// MARK: - Usage Tracking Models

/// A single usage record
struct UsageRecord: Codable, Identifiable {
    let id: Int64
    let providerID: Int64
    let providerName: String
    let service: String
    let model: String
    let inputTokens: Int
    let outputTokens: Int
    let cachedTokens: Int
    let cost: Double
    let createdAt: Date
    
    enum CodingKeys: String, CodingKey {
        case id
        case providerID = "provider_id"
        case providerName = "provider_name"
        case service
        case model
        case inputTokens = "input_tokens"
        case outputTokens = "output_tokens"
        case cachedTokens = "cached_tokens"
        case cost
        case createdAt = "created_at"
    }
    
    /// Format cost as currency
    var formattedCost: String {
        formatCost(cost)
    }
    
    /// Total tokens
    var totalTokens: Int {
        inputTokens + outputTokens
    }
}

/// Aggregated usage summary record
struct UsageSummaryRecord: Codable, Identifiable {
    let providerID: Int64
    let providerName: String
    let service: String
    let model: String
    let totalRequests: Int
    let totalInput: Int
    let totalOutput: Int
    let totalCached: Int
    let totalCost: Double
    
    var id: String { "\(providerID)-\(service)-\(model)" }
    
    enum CodingKeys: String, CodingKey {
        case providerID = "provider_id"
        case providerName = "provider_name"
        case service
        case model
        case totalRequests = "total_requests"
        case totalInput = "total_input"
        case totalOutput = "total_output"
        case totalCached = "total_cached"
        case totalCost = "total_cost"
    }
    
    /// Format cost as currency
    var formattedCost: String {
        formatCost(totalCost)
    }
    
    /// Total tokens
    var totalTokens: Int {
        totalInput + totalOutput
    }
    
    /// Format token count with K/M suffix
    var formattedTokens: String {
        let total = totalTokens
        if total >= 1_000_000 {
            return String(format: "%.1fM", Double(total) / 1_000_000.0)
        } else if total >= 1_000 {
            return String(format: "%.1fK", Double(total) / 1_000.0)
        }
        return "\(total)"
    }
}

/// Response from usage API
struct UsageResponse: Codable {
    let records: [UsageRecord]
    let totalCost: Double
    let from: Date
    let to: Date
    
    enum CodingKeys: String, CodingKey {
        case records
        case totalCost = "total_cost"
        case from
        case to
    }
}

/// Response from usage summary API
struct UsageSummaryResponse: Codable {
    let summary: [UsageSummaryRecord]
    let totalCost: Double
    let from: Date
    let to: Date
    
    enum CodingKeys: String, CodingKey {
        case summary
        case totalCost = "total_cost"
        case from
        case to
    }
    
    /// Format total cost as currency
    var formattedTotalCost: String {
        formatCost(totalCost)
    }
}

/// Helper function to format costs intelligently
/// - For costs >= $0.01: Shows 2 decimal places (e.g., "$1.23")
/// - For costs < $0.01: Shows 4 decimal places (e.g., "$0.0012", "$0.0000")
/// - For costs < $0.0001 but > 0: Shows in fractional cents (e.g., "0.05¢", "<0.01¢")
fileprivate func formatCost(_ cost: Double) -> String {
    if cost >= 0.01 {
        // Regular amounts: show 2 decimals
        return String(format: "$%.2f", cost)
    } else if cost >= 0.0001 {
        // Small amounts (including zero when displayed in context): show 4 decimals
        return String(format: "$%.4f", cost)
    } else if cost > 0 {
        // Tiny amounts: show in fractional cents
        let cents = cost * 100
        if cents >= 0.01 {
            return String(format: "%.2f¢", cents)
        } else {
            return "<0.01¢"
        }
    } else {
        // Zero: show with 4 decimals to maintain precision context
        return "$0.0000"
    }
}
