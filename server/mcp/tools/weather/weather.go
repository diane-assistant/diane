// Package weather provides yr.no weather API tools for the MCP server.
// Uses the Norwegian Meteorological Institute free API.
// No API key required, just User-Agent header.
package weather

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const userAgent = "diane github.com/Emergent-Comapny/diane"

// Tool represents an MCP tool definition
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// Provider implements weather tools
type Provider struct{}

// NewProvider creates a new weather provider
func NewProvider() *Provider {
	return &Provider{}
}

// CheckDependencies verifies weather API is accessible (no config needed)
func (p *Provider) CheckDependencies() error {
	// yr.no is a free API, no configuration needed
	return nil
}

// Tools returns the list of weather tools
func (p *Provider) Tools() []Tool {
	return []Tool{
		{
			Name:        "weather_get_weather",
			Description: "Get weather forecast from yr.no (Norwegian Meteorological Institute). Provides detailed weather data including temperature, precipitation, wind, and conditions. Supports any location worldwide using latitude/longitude coordinates.",
			InputSchema: map[string]interface{}{
				"type":     "object",
				"required": []string{"latitude", "longitude"},
				"properties": map[string]interface{}{
					"latitude": map[string]interface{}{
						"type":        "number",
						"description": "Latitude of the location (-90 to 90)",
					},
					"longitude": map[string]interface{}{
						"type":        "number",
						"description": "Longitude of the location (-180 to 180)",
					},
					"altitude": map[string]interface{}{
						"type":        "number",
						"description": "Altitude in meters above sea level (optional, improves accuracy)",
					},
				},
			},
		},
		{
			Name:        "weather_search_location_weather",
			Description: "Search for a location by name and get its weather forecast. This tool first geocodes the location name to coordinates, then fetches weather from yr.no. For cities, countries, or addresses.",
			InputSchema: map[string]interface{}{
				"type":     "object",
				"required": []string{"location"},
				"properties": map[string]interface{}{
					"location": map[string]interface{}{
						"type":        "string",
						"description": "Location name (city, address, landmark, etc.)",
					},
				},
			},
		},
	}
}

// HasTool checks if a tool name belongs to this provider
func (p *Provider) HasTool(name string) bool {
	switch name {
	case "weather_get_weather", "weather_search_location_weather":
		return true
	}
	return false
}

// Call executes a weather tool
func (p *Provider) Call(name string, args map[string]interface{}) (interface{}, error) {
	switch name {
	case "weather_get_weather":
		return p.getWeather(args)
	case "weather_search_location_weather":
		return p.searchLocationWeather(args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

// getWeather fetches weather for given coordinates
func (p *Provider) getWeather(args map[string]interface{}) (interface{}, error) {
	lat, ok := args["latitude"].(float64)
	if !ok {
		return nil, fmt.Errorf("latitude is required")
	}
	lon, ok := args["longitude"].(float64)
	if !ok {
		return nil, fmt.Errorf("longitude is required")
	}

	// Validate coordinates
	if lat < -90 || lat > 90 {
		return nil, fmt.Errorf("latitude must be between -90 and 90")
	}
	if lon < -180 || lon > 180 {
		return nil, fmt.Errorf("longitude must be between -180 and 180")
	}

	// Build API URL
	apiURL := fmt.Sprintf("https://api.met.no/weatherapi/locationforecast/2.0/compact?lat=%f&lon=%f", lat, lon)
	if alt, ok := args["altitude"].(float64); ok {
		apiURL += fmt.Sprintf("&altitude=%d", int(alt))
	}

	// Fetch weather data
	weatherData, err := p.fetchWeather(apiURL)
	if err != nil {
		return nil, err
	}

	// Build response
	result := map[string]interface{}{
		"location": map[string]interface{}{
			"latitude":  lat,
			"longitude": lon,
		},
	}
	if alt, ok := args["altitude"].(float64); ok {
		result["location"].(map[string]interface{})["altitude"] = alt
	}

	p.formatWeatherResponse(result, weatherData)
	return textContent(result), nil
}

// searchLocationWeather geocodes location and fetches weather
func (p *Provider) searchLocationWeather(args map[string]interface{}) (interface{}, error) {
	location, ok := args["location"].(string)
	if !ok || location == "" {
		return nil, fmt.Errorf("location is required")
	}

	// Geocode using Nominatim (OpenStreetMap)
	geocodeURL := fmt.Sprintf("https://nominatim.openstreetmap.org/search?q=%s&format=json&limit=1",
		url.QueryEscape(location))

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", geocodeURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("geocoding failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("geocoding failed: %s", resp.Status)
	}

	var geocodeData []struct {
		Lat         string `json:"lat"`
		Lon         string `json:"lon"`
		DisplayName string `json:"display_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&geocodeData); err != nil {
		return nil, fmt.Errorf("failed to parse geocode response: %w", err)
	}

	if len(geocodeData) == 0 {
		return nil, fmt.Errorf("location not found: %s", location)
	}

	// Parse coordinates
	var lat, lon float64
	fmt.Sscanf(geocodeData[0].Lat, "%f", &lat)
	fmt.Sscanf(geocodeData[0].Lon, "%f", &lon)

	// Fetch weather
	apiURL := fmt.Sprintf("https://api.met.no/weatherapi/locationforecast/2.0/compact?lat=%f&lon=%f", lat, lon)
	weatherData, err := p.fetchWeather(apiURL)
	if err != nil {
		return nil, err
	}

	// Build response
	result := map[string]interface{}{
		"location": map[string]interface{}{
			"name":      geocodeData[0].DisplayName,
			"latitude":  lat,
			"longitude": lon,
		},
	}

	p.formatWeatherResponse(result, weatherData)
	return textContent(result), nil
}

// fetchWeather makes HTTP request to yr.no API
func (p *Provider) fetchWeather(apiURL string) (map[string]interface{}, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch weather: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("yr.no API returned %s", resp.Status)
	}

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to parse weather response: %w", err)
	}

	return data, nil
}

// formatWeatherResponse formats yr.no data into readable output
func (p *Provider) formatWeatherResponse(result map[string]interface{}, data map[string]interface{}) {
	props, ok := data["properties"].(map[string]interface{})
	if !ok {
		return
	}

	// Extract metadata
	if meta, ok := props["meta"].(map[string]interface{}); ok {
		if updated, ok := meta["updated_at"].(string); ok {
			result["updated"] = updated
		}
		if units, ok := meta["units"].(map[string]interface{}); ok {
			result["units"] = units
		}
	}

	// Extract timeseries
	timeseries, ok := props["timeseries"].([]interface{})
	if !ok || len(timeseries) == 0 {
		return
	}

	// Current weather (first entry)
	if current := p.formatTimeseries(timeseries[0]); current != nil {
		result["current"] = current
	}

	// Hourly forecast (next 24 entries)
	var hourly []interface{}
	end := 25
	if len(timeseries) < end {
		end = len(timeseries)
	}
	for i := 1; i < end; i++ {
		if entry := p.formatTimeseries(timeseries[i]); entry != nil {
			hourly = append(hourly, entry)
		}
	}
	result["hourly_forecast"] = hourly
}

// formatTimeseries formats a single timeseries entry
func (p *Provider) formatTimeseries(entry interface{}) map[string]interface{} {
	e, ok := entry.(map[string]interface{})
	if !ok {
		return nil
	}

	result := map[string]interface{}{}

	if t, ok := e["time"].(string); ok {
		result["time"] = t
	}

	data, ok := e["data"].(map[string]interface{})
	if !ok {
		return result
	}

	// Instant details
	if instant, ok := data["instant"].(map[string]interface{}); ok {
		if details, ok := instant["details"].(map[string]interface{}); ok {
			if v, ok := details["air_temperature"].(float64); ok {
				result["temperature"] = v
			}
			if v, ok := details["wind_speed"].(float64); ok {
				result["wind_speed"] = v
			}
			if v, ok := details["wind_from_direction"].(float64); ok {
				result["wind_direction"] = v
			}
			if v, ok := details["relative_humidity"].(float64); ok {
				result["humidity"] = v
			}
			if v, ok := details["air_pressure_at_sea_level"].(float64); ok {
				result["pressure"] = v
			}
			if v, ok := details["cloud_area_fraction"].(float64); ok {
				result["cloud_coverage"] = v
			}
		}
	}

	// Precipitation from next_1_hours or next_6_hours
	precipitation := 0.0
	summary := "unknown"

	for _, period := range []string{"next_1_hours", "next_6_hours", "next_12_hours"} {
		if next, ok := data[period].(map[string]interface{}); ok {
			if details, ok := next["details"].(map[string]interface{}); ok {
				if v, ok := details["precipitation_amount"].(float64); ok && precipitation == 0 {
					precipitation = v
				}
			}
			if s, ok := next["summary"].(map[string]interface{}); ok {
				if code, ok := s["symbol_code"].(string); ok && summary == "unknown" {
					summary = code
				}
			}
		}
	}

	result["precipitation"] = precipitation
	result["summary"] = summary

	return result
}

// textContent formats result as MCP text content
func textContent(data interface{}) map[string]interface{} {
	jsonBytes, _ := json.MarshalIndent(data, "", "  ")
	return map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": string(jsonBytes),
			},
		},
	}
}
