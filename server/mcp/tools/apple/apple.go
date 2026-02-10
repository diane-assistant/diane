// Package apple provides MCP tools for Apple Reminders and Contacts
package apple

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// --- Helper Functions ---

func runCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		stderrStr := strings.TrimSpace(stderr.String())
		if stderrStr != "" {
			return "", fmt.Errorf("%s: %s", err, stderrStr)
		}
		return "", err
	}

	return strings.TrimSpace(stdout.String()), nil
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func getString(args map[string]interface{}, key string) string {
	if val, ok := args[key].(string); ok {
		return val
	}
	return ""
}

func getStringRequired(args map[string]interface{}, key string) (string, error) {
	if val, ok := args[key].(string); ok && val != "" {
		return val, nil
	}
	return "", fmt.Errorf("missing required argument: %s", key)
}

func getInt(args map[string]interface{}, key string, defaultVal int) int {
	if val, ok := args[key].(float64); ok {
		return int(val)
	}
	return defaultVal
}

func textContent(text string) map[string]interface{} {
	return map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": text,
			},
		},
	}
}

func objectSchema(properties map[string]interface{}, required []string) map[string]interface{} {
	schema := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func stringProperty(description string) map[string]interface{} {
	return map[string]interface{}{
		"type":        "string",
		"description": description,
	}
}

func numberProperty(description string) map[string]interface{} {
	return map[string]interface{}{
		"type":        "number",
		"description": description,
	}
}

// --- Tool Definition ---

type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

type Provider struct {
	contactsScriptPath string
}

func NewProvider() *Provider {
	return &Provider{}
}

func (p *Provider) Name() string {
	return "apple"
}

func (p *Provider) CheckDependencies() error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("Apple tools are only available on macOS")
	}

	if !commandExists("swift") {
		return fmt.Errorf("swift not found. Install Xcode Command Line Tools")
	}

	return nil
}

func (p *Provider) Tools() []Tool {
	return []Tool{
		// Reminders tools
		{
			Name:        "apple_list_reminders",
			Description: "List Apple Reminders from a specific list or all lists.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"listName": stringProperty("The name of the reminder list (optional, lists all if omitted)"),
				},
				nil,
			),
		},
		{
			Name:        "apple_add_reminder",
			Description: "Add a new Apple Reminder. Dates must be in YYYY-MM-DD HH:MM format.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"title":    stringProperty("The title of the reminder"),
					"listName": stringProperty("The list to add the reminder to (optional)"),
					"due":      stringProperty("Due date/time in 'YYYY-MM-DD HH:MM' format (optional)"),
				},
				[]string{"title"},
			),
		},
		// Contacts tools
		{
			Name:        "apple_search_contacts",
			Description: "Search Apple Contacts by name, email, phone, or organization. Returns matching contacts with summary info (id, name, email, phone, organization).",
			InputSchema: objectSchema(
				map[string]interface{}{
					"query": stringProperty("Search text to find contacts"),
					"field": stringProperty("Field to search: 'name' (default), 'email', 'phone', or 'all'"),
					"limit": numberProperty("Maximum number of results (default: 50)"),
				},
				[]string{"query"},
			),
		},
		{
			Name:        "apple_get_contact",
			Description: "Get complete details for a specific contact by ID. Returns all available fields including phones, emails, addresses, birthday, organization, notes, and social profiles.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"id": stringProperty("Contact identifier (from search results)"),
				},
				[]string{"id"},
			),
		},
		{
			Name:        "apple_create_contact",
			Description: "Create a new contact in Apple Contacts. Returns the new contact's ID.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"givenName":    stringProperty("First name (required)"),
					"familyName":   stringProperty("Last name"),
					"organization": stringProperty("Company/organization name"),
					"jobTitle":     stringProperty("Job title"),
					"department":   stringProperty("Department name"),
					"phones":       stringProperty("JSON array of phones: [{\"label\": \"mobile\", \"value\": \"+1234567890\"}]"),
					"emails":       stringProperty("JSON array of emails: [{\"label\": \"work\", \"value\": \"john@example.com\"}]"),
					"addresses":    stringProperty("JSON array of addresses: [{\"label\": \"work\", \"street\": \"123 Main St\", \"city\": \"SF\", \"state\": \"CA\", \"postalCode\": \"94102\", \"country\": \"USA\"}]"),
					"birthday":     stringProperty("Birthday in YYYY-MM-DD format"),
					"note":         stringProperty("Notes about the contact"),
					"urls":         stringProperty("JSON array of URLs: [{\"label\": \"homepage\", \"value\": \"https://example.com\"}]"),
				},
				[]string{"givenName"},
			),
		},
		{
			Name:        "apple_update_contact",
			Description: "Update an existing contact. Only specified fields are modified; omitted fields remain unchanged.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"id":           stringProperty("Contact identifier (required)"),
					"givenName":    stringProperty("First name"),
					"familyName":   stringProperty("Last name"),
					"organization": stringProperty("Company/organization name"),
					"jobTitle":     stringProperty("Job title"),
					"department":   stringProperty("Department name"),
					"phones":       stringProperty("JSON array of phones (replaces all existing)"),
					"emails":       stringProperty("JSON array of emails (replaces all existing)"),
					"addresses":    stringProperty("JSON array of addresses (replaces all existing)"),
					"birthday":     stringProperty("Birthday in YYYY-MM-DD format"),
					"note":         stringProperty("Notes about the contact"),
					"urls":         stringProperty("JSON array of URLs (replaces all existing)"),
				},
				[]string{"id"},
			),
		},
		{
			Name:        "apple_delete_contact",
			Description: "Delete a contact from Apple Contacts. This action cannot be undone.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"id": stringProperty("Contact identifier to delete"),
				},
				[]string{"id"},
			),
		},
		{
			Name:        "apple_list_contact_groups",
			Description: "List all contact groups/containers in Apple Contacts.",
			InputSchema: objectSchema(
				map[string]interface{}{},
				nil,
			),
		},
	}
}

func (p *Provider) HasTool(name string) bool {
	for _, tool := range p.Tools() {
		if tool.Name == name {
			return true
		}
	}
	return false
}

func (p *Provider) Call(name string, args map[string]interface{}) (interface{}, error) {
	switch name {
	// Reminders
	case "apple_list_reminders":
		return p.listReminders(args)
	case "apple_add_reminder":
		return p.addReminder(args)
	// Contacts
	case "apple_search_contacts":
		return p.searchContacts(args)
	case "apple_get_contact":
		return p.getContact(args)
	case "apple_create_contact":
		return p.createContact(args)
	case "apple_update_contact":
		return p.updateContact(args)
	case "apple_delete_contact":
		return p.deleteContact(args)
	case "apple_list_contact_groups":
		return p.listContactGroups(args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

// --- Reminders Tools ---

func (p *Provider) listReminders(args map[string]interface{}) (interface{}, error) {
	if !commandExists("remindctl") {
		return nil, fmt.Errorf("remindctl not found. Install with: brew install keith/formulae/remindctl")
	}

	listName := getString(args, "listName")

	var output string
	var err error

	if listName != "" {
		output, err = runCommand("remindctl", "list", listName, "--json")
	} else {
		output, err = runCommand("remindctl", "list", "--json")
	}

	if err != nil {
		return nil, fmt.Errorf("failed to list reminders: %w", err)
	}

	if output == "" {
		output = "No reminders found."
	}

	return textContent(output), nil
}

func (p *Provider) addReminder(args map[string]interface{}) (interface{}, error) {
	if !commandExists("remindctl") {
		return nil, fmt.Errorf("remindctl not found. Install with: brew install keith/formulae/remindctl")
	}

	title, err := getStringRequired(args, "title")
	if err != nil {
		return nil, err
	}

	listName := getString(args, "listName")
	due := getString(args, "due")

	cmdArgs := []string{"add", title}

	if listName != "" {
		cmdArgs = append(cmdArgs, "--list", listName)
	}

	if due != "" {
		cmdArgs = append(cmdArgs, "--due", due)
	}

	output, err := runCommand("remindctl", cmdArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to add reminder: %w", err)
	}

	return textContent(output), nil
}

// --- Contacts Tools ---

// Comprehensive Swift script for Apple Contacts operations
const contactsSwiftScript = `#!/usr/bin/env swift
import Contacts
import Foundation

// MARK: - JSON Encoding Helpers

struct LabeledValue: Codable {
    let label: String
    let value: String
}

struct PostalAddress: Codable {
    let label: String
    let street: String
    let city: String
    let state: String
    let postalCode: String
    let country: String
}

struct ContactSummary: Codable {
    let id: String
    let givenName: String
    let familyName: String
    let fullName: String
    let organization: String?
    let primaryEmail: String?
    let primaryPhone: String?
}

struct ContactDetails: Codable {
    let id: String
    let givenName: String
    let familyName: String
    let fullName: String
    let nickname: String?
    let organization: String?
    let jobTitle: String?
    let department: String?
    let phones: [LabeledValue]
    let emails: [LabeledValue]
    let addresses: [PostalAddress]
    let urls: [LabeledValue]
    let birthday: String?
    let note: String?
    let imageAvailable: Bool
}

struct ContactGroup: Codable {
    let id: String
    let name: String
}

struct SearchResult: Codable {
    let contacts: [ContactSummary]
    let count: Int
}

struct OperationResult: Codable {
    let success: Bool
    let message: String
    let id: String?
}

struct GroupsResult: Codable {
    let groups: [ContactGroup]
    let count: Int
}

// MARK: - Contact Store

let store = CNContactStore()

func requestAccess() -> Bool {
    var granted = false
    let semaphore = DispatchSemaphore(value: 0)
    store.requestAccess(for: .contacts) { success, error in
        granted = success
        semaphore.signal()
    }
    semaphore.wait()
    return granted
}

func outputJSON<T: Encodable>(_ value: T) {
    let encoder = JSONEncoder()
    encoder.outputFormatting = [.prettyPrinted, .sortedKeys]
    if let data = try? encoder.encode(value), let json = String(data: data, encoding: .utf8) {
        print(json)
    }
}

func outputError(_ message: String) -> Never {
    let result = OperationResult(success: false, message: message, id: nil)
    outputJSON(result)
    exit(1)
}

func labelToString(_ label: String?) -> String {
    guard let label = label else { return "other" }
    // Convert CNLabel constants to readable strings
    switch label {
    case CNLabelHome: return "home"
    case CNLabelWork: return "work"
    case CNLabelPhoneNumberMobile: return "mobile"
    case CNLabelPhoneNumberMain: return "main"
    case CNLabelPhoneNumberiPhone: return "iPhone"
    case CNLabelOther: return "other"
    default:
        // Strip the prefix if present
        if label.hasPrefix("_$!<") && label.hasSuffix(">!$_") {
            let start = label.index(label.startIndex, offsetBy: 4)
            let end = label.index(label.endIndex, offsetBy: -4)
            return String(label[start..<end]).lowercased()
        }
        return label.lowercased()
    }
}

func stringToLabel(_ string: String) -> String {
    switch string.lowercased() {
    case "home": return CNLabelHome
    case "work": return CNLabelWork
    case "mobile": return CNLabelPhoneNumberMobile
    case "main": return CNLabelPhoneNumberMain
    case "iphone": return CNLabelPhoneNumberiPhone
    case "other": return CNLabelOther
    default: return string
    }
}

// MARK: - Keys

let summaryKeys: [CNKeyDescriptor] = [
    CNContactIdentifierKey as CNKeyDescriptor,
    CNContactGivenNameKey as CNKeyDescriptor,
    CNContactFamilyNameKey as CNKeyDescriptor,
    CNContactOrganizationNameKey as CNKeyDescriptor,
    CNContactEmailAddressesKey as CNKeyDescriptor,
    CNContactPhoneNumbersKey as CNKeyDescriptor
]

let detailKeys: [CNKeyDescriptor] = [
    CNContactIdentifierKey as CNKeyDescriptor,
    CNContactGivenNameKey as CNKeyDescriptor,
    CNContactFamilyNameKey as CNKeyDescriptor,
    CNContactNicknameKey as CNKeyDescriptor,
    CNContactOrganizationNameKey as CNKeyDescriptor,
    CNContactJobTitleKey as CNKeyDescriptor,
    CNContactDepartmentNameKey as CNKeyDescriptor,
    CNContactPhoneNumbersKey as CNKeyDescriptor,
    CNContactEmailAddressesKey as CNKeyDescriptor,
    CNContactPostalAddressesKey as CNKeyDescriptor,
    CNContactUrlAddressesKey as CNKeyDescriptor,
    CNContactBirthdayKey as CNKeyDescriptor,
    CNContactNoteKey as CNKeyDescriptor,
    CNContactImageDataAvailableKey as CNKeyDescriptor
]

// MARK: - Operations

func searchContacts(query: String, field: String, limit: Int) {
    var contacts: [CNContact] = []
    
    do {
        if query.isEmpty {
            // Fetch all contacts
            let request = CNContactFetchRequest(keysToFetch: summaryKeys)
            request.sortOrder = .givenName
            var count = 0
            try store.enumerateContacts(with: request) { contact, stop in
                contacts.append(contact)
                count += 1
                if count >= limit {
                    stop.pointee = true
                }
            }
        } else {
            // Search based on field
            var predicate: NSPredicate?
            
            switch field {
            case "email":
                predicate = CNContact.predicateForContacts(matchingEmailAddress: query)
            case "phone":
                let phoneNumber = CNPhoneNumber(stringValue: query)
                predicate = CNContact.predicateForContacts(matching: phoneNumber)
            case "name", "all":
                predicate = CNContact.predicateForContacts(matchingName: query)
            default:
                predicate = CNContact.predicateForContacts(matchingName: query)
            }
            
            if let pred = predicate {
                contacts = try store.unifiedContacts(matching: pred, keysToFetch: summaryKeys)
            }
            
            // For "all" field, also search email if name didn't match
            if field == "all" && contacts.isEmpty {
                let emailPred = CNContact.predicateForContacts(matchingEmailAddress: query)
                contacts = try store.unifiedContacts(matching: emailPred, keysToFetch: summaryKeys)
            }
            
            // Limit results
            if contacts.count > limit {
                contacts = Array(contacts.prefix(limit))
            }
        }
    } catch {
        outputError("Failed to search contacts: \(error.localizedDescription)")
    }
    
    let summaries = contacts.map { contact -> ContactSummary in
        let fullName = "\(contact.givenName) \(contact.familyName)".trimmingCharacters(in: .whitespaces)
        return ContactSummary(
            id: contact.identifier,
            givenName: contact.givenName,
            familyName: contact.familyName,
            fullName: fullName.isEmpty ? contact.organizationName : fullName,
            organization: contact.organizationName.isEmpty ? nil : contact.organizationName,
            primaryEmail: contact.emailAddresses.first?.value as String?,
            primaryPhone: contact.phoneNumbers.first?.value.stringValue
        )
    }
    
    outputJSON(SearchResult(contacts: summaries, count: summaries.count))
}

func getContact(id: String) {
    do {
        let contact = try store.unifiedContact(withIdentifier: id, keysToFetch: detailKeys)
        
        let phones = contact.phoneNumbers.map { 
            LabeledValue(label: labelToString($0.label), value: $0.value.stringValue) 
        }
        let emails = contact.emailAddresses.map { 
            LabeledValue(label: labelToString($0.label), value: $0.value as String) 
        }
        let urls = contact.urlAddresses.map { 
            LabeledValue(label: labelToString($0.label), value: $0.value as String) 
        }
        let addresses = contact.postalAddresses.map { addr -> PostalAddress in
            let value = addr.value
            return PostalAddress(
                label: labelToString(addr.label),
                street: value.street,
                city: value.city,
                state: value.state,
                postalCode: value.postalCode,
                country: value.country
            )
        }
        
        var birthdayStr: String? = nil
        if let birthday = contact.birthday {
            let formatter = DateFormatter()
            formatter.dateFormat = "yyyy-MM-dd"
            if let date = Calendar.current.date(from: birthday) {
                birthdayStr = formatter.string(from: date)
            }
        }
        
        let fullName = "\(contact.givenName) \(contact.familyName)".trimmingCharacters(in: .whitespaces)
        
        let details = ContactDetails(
            id: contact.identifier,
            givenName: contact.givenName,
            familyName: contact.familyName,
            fullName: fullName.isEmpty ? contact.organizationName : fullName,
            nickname: contact.nickname.isEmpty ? nil : contact.nickname,
            organization: contact.organizationName.isEmpty ? nil : contact.organizationName,
            jobTitle: contact.jobTitle.isEmpty ? nil : contact.jobTitle,
            department: contact.departmentName.isEmpty ? nil : contact.departmentName,
            phones: phones,
            emails: emails,
            addresses: addresses,
            urls: urls,
            birthday: birthdayStr,
            note: contact.note.isEmpty ? nil : contact.note,
            imageAvailable: contact.imageDataAvailable
        )
        
        outputJSON(details)
    } catch {
        outputError("Failed to get contact: \(error.localizedDescription)")
    }
}

func createContact(jsonData: String) {
    guard let data = jsonData.data(using: .utf8),
          let params = try? JSONSerialization.jsonObject(with: data) as? [String: Any] else {
        outputError("Invalid JSON data")
    }
    
    let contact = CNMutableContact()
    
    // Required field
    contact.givenName = params["givenName"] as? String ?? ""
    
    // Optional fields
    if let familyName = params["familyName"] as? String { contact.familyName = familyName }
    if let organization = params["organization"] as? String { contact.organizationName = organization }
    if let jobTitle = params["jobTitle"] as? String { contact.jobTitle = jobTitle }
    if let department = params["department"] as? String { contact.departmentName = department }
    if let note = params["note"] as? String { contact.note = note }
    
    // Birthday
    if let birthdayStr = params["birthday"] as? String {
        let formatter = DateFormatter()
        formatter.dateFormat = "yyyy-MM-dd"
        if let date = formatter.date(from: birthdayStr) {
            contact.birthday = Calendar.current.dateComponents([.year, .month, .day], from: date)
        }
    }
    
    // Phones
    if let phonesJson = params["phones"] as? String,
       let phonesData = phonesJson.data(using: .utf8),
       let phones = try? JSONSerialization.jsonObject(with: phonesData) as? [[String: String]] {
        contact.phoneNumbers = phones.map { phone in
            CNLabeledValue(label: stringToLabel(phone["label"] ?? "mobile"),
                          value: CNPhoneNumber(stringValue: phone["value"] ?? ""))
        }
    }
    
    // Emails
    if let emailsJson = params["emails"] as? String,
       let emailsData = emailsJson.data(using: .utf8),
       let emails = try? JSONSerialization.jsonObject(with: emailsData) as? [[String: String]] {
        contact.emailAddresses = emails.map { email in
            CNLabeledValue(label: stringToLabel(email["label"] ?? "work"),
                          value: (email["value"] ?? "") as NSString)
        }
    }
    
    // URLs
    if let urlsJson = params["urls"] as? String,
       let urlsData = urlsJson.data(using: .utf8),
       let urls = try? JSONSerialization.jsonObject(with: urlsData) as? [[String: String]] {
        contact.urlAddresses = urls.map { url in
            CNLabeledValue(label: stringToLabel(url["label"] ?? "homepage"),
                          value: (url["value"] ?? "") as NSString)
        }
    }
    
    // Addresses
    if let addrsJson = params["addresses"] as? String,
       let addrsData = addrsJson.data(using: .utf8),
       let addrs = try? JSONSerialization.jsonObject(with: addrsData) as? [[String: String]] {
        contact.postalAddresses = addrs.map { addr in
            let postal = CNMutablePostalAddress()
            postal.street = addr["street"] ?? ""
            postal.city = addr["city"] ?? ""
            postal.state = addr["state"] ?? ""
            postal.postalCode = addr["postalCode"] ?? ""
            postal.country = addr["country"] ?? ""
            return CNLabeledValue(label: stringToLabel(addr["label"] ?? "work"), value: postal)
        }
    }
    
    // Save
    let saveRequest = CNSaveRequest()
    saveRequest.add(contact, toContainerWithIdentifier: nil)
    
    do {
        try store.execute(saveRequest)
        outputJSON(OperationResult(success: true, message: "Contact created successfully", id: contact.identifier))
    } catch {
        outputError("Failed to create contact: \(error.localizedDescription)")
    }
}

func updateContact(id: String, jsonData: String) {
    guard let data = jsonData.data(using: .utf8),
          let params = try? JSONSerialization.jsonObject(with: data) as? [String: Any] else {
        outputError("Invalid JSON data")
    }
    
    do {
        let contact = try store.unifiedContact(withIdentifier: id, keysToFetch: detailKeys)
        let mutableContact = contact.mutableCopy() as! CNMutableContact
        
        // Update only provided fields
        if let givenName = params["givenName"] as? String { mutableContact.givenName = givenName }
        if let familyName = params["familyName"] as? String { mutableContact.familyName = familyName }
        if let organization = params["organization"] as? String { mutableContact.organizationName = organization }
        if let jobTitle = params["jobTitle"] as? String { mutableContact.jobTitle = jobTitle }
        if let department = params["department"] as? String { mutableContact.departmentName = department }
        if let note = params["note"] as? String { mutableContact.note = note }
        
        // Birthday
        if let birthdayStr = params["birthday"] as? String {
            let formatter = DateFormatter()
            formatter.dateFormat = "yyyy-MM-dd"
            if let date = formatter.date(from: birthdayStr) {
                mutableContact.birthday = Calendar.current.dateComponents([.year, .month, .day], from: date)
            }
        }
        
        // Phones (replace all)
        if let phonesJson = params["phones"] as? String,
           let phonesData = phonesJson.data(using: .utf8),
           let phones = try? JSONSerialization.jsonObject(with: phonesData) as? [[String: String]] {
            mutableContact.phoneNumbers = phones.map { phone in
                CNLabeledValue(label: stringToLabel(phone["label"] ?? "mobile"),
                              value: CNPhoneNumber(stringValue: phone["value"] ?? ""))
            }
        }
        
        // Emails (replace all)
        if let emailsJson = params["emails"] as? String,
           let emailsData = emailsJson.data(using: .utf8),
           let emails = try? JSONSerialization.jsonObject(with: emailsData) as? [[String: String]] {
            mutableContact.emailAddresses = emails.map { email in
                CNLabeledValue(label: stringToLabel(email["label"] ?? "work"),
                              value: (email["value"] ?? "") as NSString)
            }
        }
        
        // URLs (replace all)
        if let urlsJson = params["urls"] as? String,
           let urlsData = urlsJson.data(using: .utf8),
           let urls = try? JSONSerialization.jsonObject(with: urlsData) as? [[String: String]] {
            mutableContact.urlAddresses = urls.map { url in
                CNLabeledValue(label: stringToLabel(url["label"] ?? "homepage"),
                              value: (url["value"] ?? "") as NSString)
            }
        }
        
        // Addresses (replace all)
        if let addrsJson = params["addresses"] as? String,
           let addrsData = addrsJson.data(using: .utf8),
           let addrs = try? JSONSerialization.jsonObject(with: addrsData) as? [[String: String]] {
            mutableContact.postalAddresses = addrs.map { addr in
                let postal = CNMutablePostalAddress()
                postal.street = addr["street"] ?? ""
                postal.city = addr["city"] ?? ""
                postal.state = addr["state"] ?? ""
                postal.postalCode = addr["postalCode"] ?? ""
                postal.country = addr["country"] ?? ""
                return CNLabeledValue(label: stringToLabel(addr["label"] ?? "work"), value: postal)
            }
        }
        
        // Save
        let saveRequest = CNSaveRequest()
        saveRequest.update(mutableContact)
        try store.execute(saveRequest)
        
        outputJSON(OperationResult(success: true, message: "Contact updated successfully", id: id))
    } catch {
        outputError("Failed to update contact: \(error.localizedDescription)")
    }
}

func deleteContact(id: String) {
    do {
        let keysToFetch: [CNKeyDescriptor] = [CNContactIdentifierKey as CNKeyDescriptor]
        let contact = try store.unifiedContact(withIdentifier: id, keysToFetch: keysToFetch)
        let mutableContact = contact.mutableCopy() as! CNMutableContact
        
        let saveRequest = CNSaveRequest()
        saveRequest.delete(mutableContact)
        try store.execute(saveRequest)
        
        outputJSON(OperationResult(success: true, message: "Contact deleted successfully", id: id))
    } catch {
        outputError("Failed to delete contact: \(error.localizedDescription)")
    }
}

func listGroups() {
    do {
        let containers = try store.containers(matching: nil)
        let groups = try store.groups(matching: nil)
        
        var result: [ContactGroup] = []
        
        // Add containers
        for container in containers {
            result.append(ContactGroup(id: container.identifier, name: container.name))
        }
        
        // Add groups
        for group in groups {
            result.append(ContactGroup(id: group.identifier, name: group.name))
        }
        
        outputJSON(GroupsResult(groups: result, count: result.count))
    } catch {
        outputError("Failed to list groups: \(error.localizedDescription)")
    }
}

// MARK: - Main

guard requestAccess() else {
    outputError("Contacts access denied. Please grant access in System Settings > Privacy & Security > Contacts.")
}

let args = CommandLine.arguments

guard args.count >= 2 else {
    outputError("Usage: contacts.swift <command> [options]")
}

let command = args[1]

switch command {
case "search":
    let query = args.count > 2 ? args[2] : ""
    let field = args.count > 3 ? args[3] : "name"
    let limit = args.count > 4 ? Int(args[4]) ?? 50 : 50
    searchContacts(query: query, field: field, limit: limit)
    
case "get":
    guard args.count > 2 else {
        outputError("Usage: contacts.swift get <id>")
    }
    getContact(id: args[2])
    
case "create":
    guard args.count > 2 else {
        outputError("Usage: contacts.swift create '<json>'")
    }
    createContact(jsonData: args[2])
    
case "update":
    guard args.count > 3 else {
        outputError("Usage: contacts.swift update <id> '<json>'")
    }
    updateContact(id: args[2], jsonData: args[3])
    
case "delete":
    guard args.count > 2 else {
        outputError("Usage: contacts.swift delete <id>")
    }
    deleteContact(id: args[2])
    
case "groups":
    listGroups()
    
default:
    outputError("Unknown command: \(command). Valid commands: search, get, create, update, delete, groups")
}
`

// scriptVersion is incremented when the Swift script changes to force recompilation
const scriptVersion = "1"

// ensureContactsBinary compiles the Swift script to a binary and returns its path.
// The binary is named "diane-contacts" so macOS permission dialogs show a friendly name.
func (p *Provider) ensureContactsBinary() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	toolsDir := filepath.Join(home, ".diane", "tools")
	scriptPath := filepath.Join(toolsDir, "contacts.swift")
	binaryPath := filepath.Join(toolsDir, "diane-contacts")
	versionPath := filepath.Join(toolsDir, ".contacts-version")

	// Check if we have a cached path and binary exists with correct version
	if p.contactsScriptPath != "" {
		if _, err := os.Stat(p.contactsScriptPath); err == nil {
			return p.contactsScriptPath, nil
		}
	}

	// Create directory if needed
	if err := os.MkdirAll(toolsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create tools directory: %w", err)
	}

	// Check if binary exists and version matches
	needsCompile := true
	if _, err := os.Stat(binaryPath); err == nil {
		// Binary exists, check version
		if versionData, err := os.ReadFile(versionPath); err == nil {
			if string(versionData) == scriptVersion {
				needsCompile = false
			}
		}
	}

	if needsCompile {
		// Write the Swift source
		if err := os.WriteFile(scriptPath, []byte(contactsSwiftScript), 0644); err != nil {
			return "", fmt.Errorf("failed to write Swift script: %w", err)
		}

		// Compile to binary with a friendly name
		// The binary name "diane-contacts" will appear in macOS permission dialogs
		_, err := runCommand("swiftc", "-O", "-o", binaryPath, scriptPath)
		if err != nil {
			return "", fmt.Errorf("failed to compile Swift script: %w", err)
		}

		// Write version file
		if err := os.WriteFile(versionPath, []byte(scriptVersion), 0644); err != nil {
			// Non-fatal, just means we might recompile unnecessarily next time
		}

		// Clean up source file after compilation
		os.Remove(scriptPath)
	}

	p.contactsScriptPath = binaryPath
	return binaryPath, nil
}

func (p *Provider) searchContacts(args map[string]interface{}) (interface{}, error) {
	query := getString(args, "query")
	field := getString(args, "field")
	if field == "" {
		field = "name"
	}
	limit := getInt(args, "limit", 50)

	binaryPath, err := p.ensureContactsBinary()
	if err != nil {
		return nil, fmt.Errorf("failed to setup contacts binary: %w", err)
	}

	output, err := runCommand(binaryPath, "search", query, field, fmt.Sprintf("%d", limit))
	if err != nil {
		return nil, fmt.Errorf("failed to search contacts: %w", err)
	}

	return textContent(output), nil
}

func (p *Provider) getContact(args map[string]interface{}) (interface{}, error) {
	id, err := getStringRequired(args, "id")
	if err != nil {
		return nil, err
	}

	binaryPath, err := p.ensureContactsBinary()
	if err != nil {
		return nil, fmt.Errorf("failed to setup contacts binary: %w", err)
	}

	output, err := runCommand(binaryPath, "get", id)
	if err != nil {
		return nil, fmt.Errorf("failed to get contact: %w", err)
	}

	return textContent(output), nil
}

func (p *Provider) createContact(args map[string]interface{}) (interface{}, error) {
	// Build JSON from arguments
	params := make(map[string]interface{})

	// Required
	givenName, err := getStringRequired(args, "givenName")
	if err != nil {
		return nil, err
	}
	params["givenName"] = givenName

	// Optional string fields
	for _, field := range []string{"familyName", "organization", "jobTitle", "department", "birthday", "note", "phones", "emails", "addresses", "urls"} {
		if val := getString(args, field); val != "" {
			params[field] = val
		}
	}

	jsonBytes, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize contact data: %w", err)
	}

	binaryPath, err := p.ensureContactsBinary()
	if err != nil {
		return nil, fmt.Errorf("failed to setup contacts binary: %w", err)
	}

	output, err := runCommand(binaryPath, "create", string(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create contact: %w", err)
	}

	return textContent(output), nil
}

func (p *Provider) updateContact(args map[string]interface{}) (interface{}, error) {
	id, err := getStringRequired(args, "id")
	if err != nil {
		return nil, err
	}

	// Build JSON from arguments (excluding id)
	params := make(map[string]interface{})

	for _, field := range []string{"givenName", "familyName", "organization", "jobTitle", "department", "birthday", "note", "phones", "emails", "addresses", "urls"} {
		if val := getString(args, field); val != "" {
			params[field] = val
		}
	}

	if len(params) == 0 {
		return nil, fmt.Errorf("no fields to update")
	}

	jsonBytes, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize contact data: %w", err)
	}

	binaryPath, err := p.ensureContactsBinary()
	if err != nil {
		return nil, fmt.Errorf("failed to setup contacts binary: %w", err)
	}

	output, err := runCommand(binaryPath, "update", id, string(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to update contact: %w", err)
	}

	return textContent(output), nil
}

func (p *Provider) deleteContact(args map[string]interface{}) (interface{}, error) {
	id, err := getStringRequired(args, "id")
	if err != nil {
		return nil, err
	}

	binaryPath, err := p.ensureContactsBinary()
	if err != nil {
		return nil, fmt.Errorf("failed to setup contacts binary: %w", err)
	}

	output, err := runCommand(binaryPath, "delete", id)
	if err != nil {
		return nil, fmt.Errorf("failed to delete contact: %w", err)
	}

	return textContent(output), nil
}

func (p *Provider) listContactGroups(args map[string]interface{}) (interface{}, error) {
	binaryPath, err := p.ensureContactsBinary()
	if err != nil {
		return nil, fmt.Errorf("failed to setup contacts binary: %w", err)
	}

	output, err := runCommand(binaryPath, "groups")
	if err != nil {
		return nil, fmt.Errorf("failed to list contact groups: %w", err)
	}

	return textContent(output), nil
}
