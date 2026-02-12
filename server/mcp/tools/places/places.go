// Package places provides MCP tools for Google Places API
package places

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// --- Configuration ---

type placesConfig struct {
	APIKey string `json:"api_key"`
}

var (
	config     *placesConfig
	secretsDir string
)

// --- Helper Functions ---

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

func getNumber(args map[string]interface{}, key string, defaultVal float64) float64 {
	if val, ok := args[key].(float64); ok {
		return val
	}
	return defaultVal
}

func getBool(args map[string]interface{}, key string) (bool, bool) {
	if val, ok := args[key].(bool); ok {
		return val, true
	}
	return false, false
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

func boolProperty(description string) map[string]interface{} {
	return map[string]interface{}{
		"type":        "boolean",
		"description": description,
	}
}

// --- Geocoding Helper ---

func geocodeLocation(location string) (lat, lng float64, err error) {
	// Check if already coordinates
	coordsRegex := regexp.MustCompile(`^(-?\d+\.?\d*),\s*(-?\d+\.?\d*)$`)
	if matches := coordsRegex.FindStringSubmatch(location); matches != nil {
		fmt.Sscanf(matches[1], "%f", &lat)
		fmt.Sscanf(matches[2], "%f", &lng)
		return lat, lng, nil
	}

	// Geocode the address
	geocodeURL := fmt.Sprintf("https://maps.googleapis.com/maps/api/geocode/json?address=%s&key=%s",
		url.QueryEscape(location), config.APIKey)

	resp, err := http.Get(geocodeURL)
	if err != nil {
		return 0, 0, fmt.Errorf("geocoding request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var data struct {
		Status  string `json:"status"`
		Results []struct {
			Geometry struct {
				Location struct {
					Lat float64 `json:"lat"`
					Lng float64 `json:"lng"`
				} `json:"location"`
			} `json:"geometry"`
		} `json:"results"`
	}

	if err := json.Unmarshal(body, &data); err != nil {
		return 0, 0, fmt.Errorf("failed to parse geocoding response: %w", err)
	}

	if data.Status != "OK" || len(data.Results) == 0 {
		return 0, 0, fmt.Errorf("could not geocode location: %s", location)
	}

	return data.Results[0].Geometry.Location.Lat, data.Results[0].Geometry.Location.Lng, nil
}

// --- Tool Definition ---

// Tool represents an MCP tool definition
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// Provider implements ToolProvider for Google Places tools
type Provider struct {
	available bool
}

// NewProvider creates a new places tools provider
func NewProvider() *Provider {
	return &Provider{}
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "places"
}

// CheckDependencies verifies required configurations exist
func (p *Provider) CheckDependencies() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	secretsDir = filepath.Join(home, ".diane", "secrets")
	configPath := filepath.Join(secretsDir, "google-places-config.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("Google Places config not found at %s. Please create it with your API key", configPath)
	}

	var cfg placesConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("failed to parse Google Places config: %w", err)
	}

	if cfg.APIKey == "" {
		return fmt.Errorf("Google Places config missing api_key field")
	}

	config = &cfg
	p.available = true
	return nil
}

// Tools returns all Google Places tools
func (p *Provider) Tools() []Tool {
	if !p.available {
		return nil
	}

	return []Tool{
		{
			Name:        "places_search",
			Description: "Search for places using Google Places API. Find restaurants, cafes, attractions, hotels, shops, and more. Supports text search with optional location bias and filters.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"query":       stringProperty("Search query (e.g., 'Italian restaurants', 'coffee shops near me', 'Eiffel Tower')"),
					"location":    stringProperty("Location to center search (address, city, or coordinates as 'lat,lng')"),
					"radius":      numberProperty("Search radius in meters (max 50000). Only used with location parameter."),
					"type":        stringProperty("Place type filter (e.g., 'restaurant', 'cafe', 'hotel', 'tourist_attraction', 'museum'). See Google Places types."),
					"min_rating":  numberProperty("Minimum rating filter (0-5)"),
					"open_now":    boolProperty("Filter to only show places currently open"),
					"max_results": numberProperty("Maximum number of results to return (default: 20, max: 60)"),
				},
				[]string{"query"},
			),
		},
		{
			Name:        "places_get_details",
			Description: "Get detailed information about a specific place using its Place ID. Includes photos, reviews, opening hours, contact info, and more.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"place_id": stringProperty("Google Places ID (from search_places results)"),
					"fields":   stringProperty("Comma-separated list of fields to include (e.g., 'name,rating,reviews,opening_hours,photos,website,phone'). Default: all fields."),
				},
				[]string{"place_id"},
			),
		},
		{
			Name:        "places_find_nearby",
			Description: "Find places near a specific location or coordinates. Great for 'what's near me' queries. Returns places within a radius sorted by prominence or distance.",
			InputSchema: objectSchema(
				map[string]interface{}{
					"location":    stringProperty("Location as address/city or coordinates 'lat,lng'"),
					"radius":      numberProperty("Search radius in meters (max 50000)"),
					"type":        stringProperty("Place type (e.g., 'restaurant', 'cafe', 'atm', 'gas_station', 'pharmacy')"),
					"keyword":     stringProperty("Keyword to match in place name or type (e.g., 'pizza', 'vegan', 'luxury')"),
					"min_rating":  numberProperty("Minimum rating filter (0-5)"),
					"open_now":    boolProperty("Filter to only show places currently open"),
					"max_results": numberProperty("Maximum number of results (default: 20, max: 60)"),
				},
				[]string{"location", "radius"},
			),
		},
	}
}

// HasTool checks if a tool belongs to this provider
func (p *Provider) HasTool(name string) bool {
	if !p.available {
		return false
	}
	return strings.HasPrefix(name, "places_")
}

// Call executes a tool by name
func (p *Provider) Call(name string, args map[string]interface{}) (interface{}, error) {
	if !p.available {
		return nil, fmt.Errorf("Google Places tools not available")
	}

	switch name {
	case "places_search":
		return p.searchPlaces(args)
	case "places_get_details":
		return p.getPlaceDetails(args)
	case "places_find_nearby":
		return p.findNearbyPlaces(args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

// --- Tool Implementations ---

func (p *Provider) searchPlaces(args map[string]interface{}) (interface{}, error) {
	query, err := getStringRequired(args, "query")
	if err != nil {
		return nil, err
	}

	location := getString(args, "location")
	radius := getNumber(args, "radius", 0)
	placeType := getString(args, "type")
	minRating := getNumber(args, "min_rating", 0)
	openNow, _ := getBool(args, "open_now")
	maxResults := int(getNumber(args, "max_results", 20))
	if maxResults > 60 {
		maxResults = 60
	}

	// Build URL
	apiURL := fmt.Sprintf("https://maps.googleapis.com/maps/api/place/textsearch/json?query=%s&key=%s",
		url.QueryEscape(query), config.APIKey)

	// Add location bias if provided
	if location != "" {
		lat, lng, err := geocodeLocation(location)
		if err == nil {
			apiURL += fmt.Sprintf("&location=%f,%f", lat, lng)
			if radius > 0 {
				if radius > 50000 {
					radius = 50000
				}
				apiURL += fmt.Sprintf("&radius=%d", int(radius))
			}
		}
	}

	if placeType != "" {
		apiURL += "&type=" + placeType
	}
	if openNow {
		apiURL += "&opennow=true"
	}

	// Make request
	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("places search request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var data struct {
		Status       string `json:"status"`
		ErrorMessage string `json:"error_message"`
		Results      []struct {
			Name             string   `json:"name"`
			FormattedAddress string   `json:"formatted_address"`
			Rating           float64  `json:"rating"`
			UserRatingsTotal int      `json:"user_ratings_total"`
			PriceLevel       int      `json:"price_level"`
			Types            []string `json:"types"`
			PlaceID          string   `json:"place_id"`
			OpeningHours     *struct {
				OpenNow bool `json:"open_now"`
			} `json:"opening_hours"`
			Geometry struct {
				Location struct {
					Lat float64 `json:"lat"`
					Lng float64 `json:"lng"`
				} `json:"location"`
			} `json:"geometry"`
			Photos []struct{} `json:"photos"`
		} `json:"results"`
	}

	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("failed to parse places response: %w", err)
	}

	if data.Status != "OK" && data.Status != "ZERO_RESULTS" {
		return nil, fmt.Errorf("Google Places API error: %s - %s", data.Status, data.ErrorMessage)
	}

	if data.Status == "ZERO_RESULTS" {
		return textContent("No places found matching your criteria."), nil
	}

	// Filter by rating
	var places []map[string]interface{}
	for _, place := range data.Results {
		if minRating > 0 && place.Rating < minRating {
			continue
		}

		priceStr := "N/A"
		if place.PriceLevel > 0 {
			priceStr = strings.Repeat("$", place.PriceLevel)
		}

		var isOpen interface{}
		if place.OpeningHours != nil {
			isOpen = place.OpeningHours.OpenNow
		}

		places = append(places, map[string]interface{}{
			"name":               place.Name,
			"address":            place.FormattedAddress,
			"rating":             place.Rating,
			"user_ratings_total": place.UserRatingsTotal,
			"price_level":        priceStr,
			"types":              place.Types,
			"open_now":           isOpen,
			"location": map[string]float64{
				"lat": place.Geometry.Location.Lat,
				"lng": place.Geometry.Location.Lng,
			},
			"place_id": place.PlaceID,
			"photos":   len(place.Photos),
		})

		if len(places) >= maxResults {
			break
		}
	}

	result, _ := json.MarshalIndent(map[string]interface{}{
		"total_results": len(places),
		"places":        places,
	}, "", "  ")

	return textContent(string(result)), nil
}

func (p *Provider) getPlaceDetails(args map[string]interface{}) (interface{}, error) {
	placeID, err := getStringRequired(args, "place_id")
	if err != nil {
		return nil, err
	}

	fields := getString(args, "fields")
	if fields == "" {
		fields = "name,formatted_address,formatted_phone_number,website,rating,user_ratings_total,reviews,opening_hours,price_level,photos,geometry,types,url"
	}

	apiURL := fmt.Sprintf("https://maps.googleapis.com/maps/api/place/details/json?place_id=%s&fields=%s&key=%s",
		placeID, fields, config.APIKey)

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("place details request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var data struct {
		Status       string `json:"status"`
		ErrorMessage string `json:"error_message"`
		Result       struct {
			Name                 string   `json:"name"`
			FormattedAddress     string   `json:"formatted_address"`
			FormattedPhoneNumber string   `json:"formatted_phone_number"`
			Website              string   `json:"website"`
			URL                  string   `json:"url"`
			Rating               float64  `json:"rating"`
			UserRatingsTotal     int      `json:"user_ratings_total"`
			PriceLevel           int      `json:"price_level"`
			Types                []string `json:"types"`
			OpeningHours         *struct {
				OpenNow     bool     `json:"open_now"`
				WeekdayText []string `json:"weekday_text"`
			} `json:"opening_hours"`
			Geometry struct {
				Location struct {
					Lat float64 `json:"lat"`
					Lng float64 `json:"lng"`
				} `json:"location"`
			} `json:"geometry"`
			Reviews []struct {
				AuthorName string  `json:"author_name"`
				Rating     float64 `json:"rating"`
				Text       string  `json:"text"`
				Time       int64   `json:"time"`
			} `json:"reviews"`
			Photos []struct {
				PhotoReference string `json:"photo_reference"`
			} `json:"photos"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("failed to parse place details response: %w", err)
	}

	if data.Status != "OK" {
		return nil, fmt.Errorf("Google Places API error: %s - %s", data.Status, data.ErrorMessage)
	}

	place := data.Result
	priceStr := "N/A"
	if place.PriceLevel > 0 {
		priceStr = strings.Repeat("$", place.PriceLevel)
	}

	var openingHours interface{}
	if place.OpeningHours != nil {
		openingHours = map[string]interface{}{
			"open_now":     place.OpeningHours.OpenNow,
			"weekday_text": place.OpeningHours.WeekdayText,
		}
	}

	var reviews []map[string]interface{}
	for i, review := range place.Reviews {
		if i >= 5 {
			break
		}
		reviews = append(reviews, map[string]interface{}{
			"author": review.AuthorName,
			"rating": review.Rating,
			"text":   review.Text,
			"time":   review.Time,
		})
	}

	var photoRefs []string
	for i, photo := range place.Photos {
		if i >= 3 {
			break
		}
		photoRefs = append(photoRefs, photo.PhotoReference)
	}

	result, _ := json.MarshalIndent(map[string]interface{}{
		"name":               place.Name,
		"address":            place.FormattedAddress,
		"phone":              place.FormattedPhoneNumber,
		"website":            place.Website,
		"google_maps_url":    place.URL,
		"rating":             place.Rating,
		"user_ratings_total": place.UserRatingsTotal,
		"price_level":        priceStr,
		"types":              place.Types,
		"location": map[string]float64{
			"lat": place.Geometry.Location.Lat,
			"lng": place.Geometry.Location.Lng,
		},
		"opening_hours":    openingHours,
		"reviews":          reviews,
		"photos_count":     len(place.Photos),
		"photo_references": photoRefs,
	}, "", "  ")

	return textContent(string(result)), nil
}

func (p *Provider) findNearbyPlaces(args map[string]interface{}) (interface{}, error) {
	location, err := getStringRequired(args, "location")
	if err != nil {
		return nil, err
	}

	radius := getNumber(args, "radius", 1000)
	if radius > 50000 {
		radius = 50000
	}

	lat, lng, err := geocodeLocation(location)
	if err != nil {
		return nil, err
	}

	placeType := getString(args, "type")
	keyword := getString(args, "keyword")
	minRating := getNumber(args, "min_rating", 0)
	openNow, _ := getBool(args, "open_now")
	maxResults := int(getNumber(args, "max_results", 20))
	if maxResults > 60 {
		maxResults = 60
	}

	// Build URL
	apiURL := fmt.Sprintf("https://maps.googleapis.com/maps/api/place/nearbysearch/json?location=%f,%f&radius=%d&key=%s",
		lat, lng, int(radius), config.APIKey)

	if placeType != "" {
		apiURL += "&type=" + placeType
	}
	if keyword != "" {
		apiURL += "&keyword=" + url.QueryEscape(keyword)
	}
	if openNow {
		apiURL += "&opennow=true"
	}

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("nearby places request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var data struct {
		Status       string `json:"status"`
		ErrorMessage string `json:"error_message"`
		Results      []struct {
			Name             string   `json:"name"`
			Vicinity         string   `json:"vicinity"`
			Rating           float64  `json:"rating"`
			UserRatingsTotal int      `json:"user_ratings_total"`
			PriceLevel       int      `json:"price_level"`
			Types            []string `json:"types"`
			PlaceID          string   `json:"place_id"`
			OpeningHours     *struct {
				OpenNow bool `json:"open_now"`
			} `json:"opening_hours"`
			Geometry struct {
				Location struct {
					Lat float64 `json:"lat"`
					Lng float64 `json:"lng"`
				} `json:"location"`
			} `json:"geometry"`
		} `json:"results"`
	}

	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("failed to parse nearby places response: %w", err)
	}

	if data.Status != "OK" && data.Status != "ZERO_RESULTS" {
		return nil, fmt.Errorf("Google Places API error: %s - %s", data.Status, data.ErrorMessage)
	}

	if data.Status == "ZERO_RESULTS" {
		return textContent("No places found nearby."), nil
	}

	var places []map[string]interface{}
	for _, place := range data.Results {
		if minRating > 0 && place.Rating < minRating {
			continue
		}

		priceStr := "N/A"
		if place.PriceLevel > 0 {
			priceStr = strings.Repeat("$", place.PriceLevel)
		}

		var isOpen interface{}
		if place.OpeningHours != nil {
			isOpen = place.OpeningHours.OpenNow
		}

		places = append(places, map[string]interface{}{
			"name":               place.Name,
			"vicinity":           place.Vicinity,
			"rating":             place.Rating,
			"user_ratings_total": place.UserRatingsTotal,
			"price_level":        priceStr,
			"types":              place.Types,
			"open_now":           isOpen,
			"location": map[string]float64{
				"lat": place.Geometry.Location.Lat,
				"lng": place.Geometry.Location.Lng,
			},
			"place_id": place.PlaceID,
		})

		if len(places) >= maxResults {
			break
		}
	}

	result, _ := json.MarshalIndent(map[string]interface{}{
		"search_location": map[string]float64{"lat": lat, "lng": lng},
		"radius_meters":   int(radius),
		"total_results":   len(places),
		"places":          places,
	}, "", "  ")

	return textContent(string(result)), nil
}
