package export

// Column defines one column in the exported file.
type Column struct {
	Field     string           // Key in the data map
	Header    string           // Display header (Chinese)
	Transform func(any) string // Optional value transformer
}

// Config holds the full export configuration.
type Config struct {
	Format   string   // "csv" or "xlsx"
	Filename string   // Without extension
	Columns  []Column // Ordered column definitions
	MaxRows  int      // Safety cap for Excel (default 100000)
}

// GetMaxRows returns the configured max rows or the default.
func (c Config) GetMaxRows() int {
	if c.MaxRows <= 0 {
		return 100000
	}
	return c.MaxRows
}
