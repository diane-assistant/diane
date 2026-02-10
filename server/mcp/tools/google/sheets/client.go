// Package sheets provides native Google Sheets API client
package sheets

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/diane-assistant/diane/mcp/tools/google/auth"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

// Client wraps the Google Sheets API service
type Client struct {
	srv     *sheets.Service
	account string
}

// SheetMetadata represents spreadsheet metadata
type SheetMetadata struct {
	SpreadsheetID string      `json:"spreadsheetId"`
	Title         string      `json:"title"`
	Locale        string      `json:"locale,omitempty"`
	TimeZone      string      `json:"timeZone,omitempty"`
	Sheets        []SheetInfo `json:"sheets"`
}

// SheetInfo represents individual sheet tab info
type SheetInfo struct {
	SheetID     int64  `json:"sheetId"`
	Title       string `json:"title"`
	Index       int64  `json:"index"`
	RowCount    int64  `json:"rowCount,omitempty"`
	ColCount    int64  `json:"columnCount,omitempty"`
	Hidden      bool   `json:"hidden,omitempty"`
	RightToLeft bool   `json:"rightToLeft,omitempty"`
}

// NewClient creates a new Google Sheets API client
func NewClient(account string) (*Client, error) {
	if account == "" {
		account = "default"
	}

	ctx := context.Background()

	tokenSource, err := auth.GetTokenSource(ctx, account, sheets.SpreadsheetsScope)
	if err != nil {
		return nil, fmt.Errorf("failed to get token source: %w", err)
	}

	srv, err := sheets.NewService(ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return nil, fmt.Errorf("failed to create Sheets service: %w", err)
	}

	return &Client{srv: srv, account: account}, nil
}

// GetRange gets data from a spreadsheet range
func (c *Client) GetRange(spreadsheetID, rangeA1 string) ([][]interface{}, error) {
	resp, err := c.srv.Spreadsheets.Values.Get(spreadsheetID, rangeA1).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get range: %w", err)
	}

	return resp.Values, nil
}

// UpdateRange updates data in a spreadsheet range
func (c *Client) UpdateRange(spreadsheetID, rangeA1 string, values [][]interface{}) (*UpdateResult, error) {
	vr := &sheets.ValueRange{
		Values: values,
	}

	resp, err := c.srv.Spreadsheets.Values.Update(spreadsheetID, rangeA1, vr).
		ValueInputOption("USER_ENTERED").
		Do()
	if err != nil {
		return nil, fmt.Errorf("failed to update range: %w", err)
	}

	return &UpdateResult{
		SpreadsheetID:  resp.SpreadsheetId,
		UpdatedRange:   resp.UpdatedRange,
		UpdatedRows:    resp.UpdatedRows,
		UpdatedColumns: resp.UpdatedColumns,
		UpdatedCells:   resp.UpdatedCells,
	}, nil
}

// UpdateResult represents the result of an update operation
type UpdateResult struct {
	SpreadsheetID  string `json:"spreadsheetId"`
	UpdatedRange   string `json:"updatedRange"`
	UpdatedRows    int64  `json:"updatedRows"`
	UpdatedColumns int64  `json:"updatedColumns"`
	UpdatedCells   int64  `json:"updatedCells"`
}

// AppendRows appends rows to a spreadsheet
func (c *Client) AppendRows(spreadsheetID, rangeA1 string, values [][]interface{}) (*AppendResult, error) {
	vr := &sheets.ValueRange{
		Values: values,
	}

	resp, err := c.srv.Spreadsheets.Values.Append(spreadsheetID, rangeA1, vr).
		ValueInputOption("USER_ENTERED").
		InsertDataOption("INSERT_ROWS").
		Do()
	if err != nil {
		return nil, fmt.Errorf("failed to append rows: %w", err)
	}

	result := &AppendResult{
		SpreadsheetID: resp.SpreadsheetId,
		TableRange:    resp.TableRange,
	}

	if resp.Updates != nil {
		result.UpdatedRange = resp.Updates.UpdatedRange
		result.UpdatedRows = resp.Updates.UpdatedRows
		result.UpdatedColumns = resp.Updates.UpdatedColumns
		result.UpdatedCells = resp.Updates.UpdatedCells
	}

	return result, nil
}

// AppendResult represents the result of an append operation
type AppendResult struct {
	SpreadsheetID  string `json:"spreadsheetId"`
	TableRange     string `json:"tableRange,omitempty"`
	UpdatedRange   string `json:"updatedRange"`
	UpdatedRows    int64  `json:"updatedRows"`
	UpdatedColumns int64  `json:"updatedColumns"`
	UpdatedCells   int64  `json:"updatedCells"`
}

// ClearRange clears data from a spreadsheet range
func (c *Client) ClearRange(spreadsheetID, rangeA1 string) (*ClearResult, error) {
	resp, err := c.srv.Spreadsheets.Values.Clear(spreadsheetID, rangeA1, &sheets.ClearValuesRequest{}).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to clear range: %w", err)
	}

	return &ClearResult{
		SpreadsheetID: resp.SpreadsheetId,
		ClearedRange:  resp.ClearedRange,
	}, nil
}

// ClearResult represents the result of a clear operation
type ClearResult struct {
	SpreadsheetID string `json:"spreadsheetId"`
	ClearedRange  string `json:"clearedRange"`
}

// GetMetadata gets spreadsheet metadata including sheet tabs
func (c *Client) GetMetadata(spreadsheetID string) (*SheetMetadata, error) {
	resp, err := c.srv.Spreadsheets.Get(spreadsheetID).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata: %w", err)
	}

	metadata := &SheetMetadata{
		SpreadsheetID: resp.SpreadsheetId,
		Title:         resp.Properties.Title,
		Locale:        resp.Properties.Locale,
		TimeZone:      resp.Properties.TimeZone,
		Sheets:        make([]SheetInfo, len(resp.Sheets)),
	}

	for i, sheet := range resp.Sheets {
		props := sheet.Properties
		metadata.Sheets[i] = SheetInfo{
			SheetID:     props.SheetId,
			Title:       props.Title,
			Index:       props.Index,
			Hidden:      props.Hidden,
			RightToLeft: props.RightToLeft,
		}
		if props.GridProperties != nil {
			metadata.Sheets[i].RowCount = props.GridProperties.RowCount
			metadata.Sheets[i].ColCount = props.GridProperties.ColumnCount
		}
	}

	return metadata, nil
}

// ToJSON converts an object to JSON string
func ToJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}
	return string(b)
}
