package transform

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/edgeflare/pgo/pkg/pglogrepl"
)

// FilterConfig holds the configuration for table filtering
type FilterConfig struct {
	// IncludeTables is a list of table patterns to include (can use wildcards)
	IncludeTables []string `json:"includeTables,omitempty"`
	// ExcludeTables is a list of table patterns to exclude (can use wildcards)
	ExcludeTables []string `json:"excludeTables,omitempty"`
	// TablePattern is a regex pattern to match table names (for backward compatibility)
	TablePattern string `json:"tablePattern,omitempty"`
}

type tableRef struct {
	schema string
	table  string
	isGlob bool
}

func parseTableRef(ref string) tableRef {
	// Handle special case of *.*
	if ref == "*.*" {
		return tableRef{schema: "*", table: "*", isGlob: true}
	}

	parts := strings.Split(ref, ".")
	if len(parts) == 2 {
		return tableRef{
			schema: parts[0],
			table:  parts[1],
			isGlob: strings.Contains(parts[0], "*") || strings.Contains(parts[1], "*"),
		}
	}
	return tableRef{
		table:  ref,
		isGlob: strings.Contains(ref, "*"),
	}
}

func (c *FilterConfig) Validate() error {
	if len(c.IncludeTables) == 0 && len(c.ExcludeTables) == 0 && c.TablePattern == "" {
		return fmt.Errorf("at least one filter criteria (include, exclude, or pattern) is required")
	}

	if c.TablePattern != "" {
		_, err := regexp.Compile(c.TablePattern)
		if err != nil {
			return fmt.Errorf("invalid table pattern: %w", err)
		}
	}

	return nil
}

func (c *FilterConfig) Type() string {
	return "filter"
}

// matchesTableRef checks if a CDC event matches a table reference
func matchesTableRef(cdc *pglogrepl.CDC, ref tableRef) bool {
	if ref.isGlob {
		// Handle global wildcard *.* case
		if ref.schema == "*" && ref.table == "*" {
			return true
		}

		// Handle schema.* case
		if ref.schema != "*" && ref.table == "*" {
			matched, _ := filepath.Match(ref.schema, cdc.Payload.Source.Schema)
			return matched
		}

		// Handle *.table case
		if ref.schema == "*" && ref.table != "*" {
			matched, _ := filepath.Match(ref.table, cdc.Payload.Source.Table)
			return matched
		}

		// Handle patterns in both schema and table
		schemaMatched, _ := filepath.Match(ref.schema, cdc.Payload.Source.Schema)
		tableMatched, _ := filepath.Match(ref.table, cdc.Payload.Source.Table)
		return schemaMatched && tableMatched
	}

	// Non-glob exact matching
	if ref.schema != "" && ref.schema != cdc.Payload.Source.Schema {
		return false
	}
	return ref.table == cdc.Payload.Source.Table
}

// Filter creates a TransformFunc that filters CDC events based on table patterns
func Filter(config *FilterConfig) TransformFunc {
	var tableRegex *regexp.Regexp
	if config.TablePattern != "" {
		tableRegex = regexp.MustCompile(config.TablePattern)
	}

	// Parse table references once during initialization
	var includeRefs []tableRef
	var excludeRefs []tableRef

	for _, table := range config.IncludeTables {
		includeRefs = append(includeRefs, parseTableRef(table))
	}

	for _, table := range config.ExcludeTables {
		excludeRefs = append(excludeRefs, parseTableRef(table))
	}

	return func(cdc *pglogrepl.CDC) (*pglogrepl.CDC, error) {
		if err := config.Validate(); err != nil {
			return nil, fmt.Errorf("invalid filter configuration: %w", err)
		}

		// Check exclude list first
		for _, ref := range excludeRefs {
			if matchesTableRef(cdc, ref) {
				return nil, nil // Skip this event
			}
		}

		// If include list is specified, table must be in it
		if len(includeRefs) > 0 {
			included := false
			for _, ref := range includeRefs {
				if matchesTableRef(cdc, ref) {
					included = true
					break
				}
			}
			if !included {
				return nil, nil // Skip this event
			}
		}

		// Check regex pattern if specified (for backward compatibility)
		if tableRegex != nil {
			tableFullName := fmt.Sprintf("%s.%s", cdc.Payload.Source.Schema, cdc.Payload.Source.Table)
			if !tableRegex.MatchString(tableFullName) && !tableRegex.MatchString(cdc.Payload.Source.Table) {
				return nil, nil
			}
		}

		return cdc, nil
	}
}
