/*
Copyright 2026 Microsoft Corporation.

Licensed under the MIT License.
See LICENSE file in the project root for full license information.
*/

package validation

import (
	"regexp"
	"strings"
	"unicode"
)

// PasswordComplexityResult contains detailed password validation results
type PasswordComplexityResult struct {
	Valid          bool
	HasMinLength   bool
	HasUppercase   bool
	HasLowercase   bool
	HasDigit       bool
	HasSpecialChar bool
	Message        string
}

// ValidatePasswordComplexity checks if a password meets SQL Server complexity requirements
// SQL Server requires:
// - Minimum 8 characters
// - Contains characters from at least 3 of these categories:
//   - Uppercase letters (A-Z)
//   - Lowercase letters (a-z)
//   - Digits (0-9)
//   - Special characters (!@#$%^&*()_+-=[]{}|;:,.<>?)
func ValidatePasswordComplexity(password string) *PasswordComplexityResult {
	result := &PasswordComplexityResult{
		Valid: true,
	}

	// Minimum length check
	if len(password) >= 8 {
		result.HasMinLength = true
	}

	// Character category checks
	for _, char := range password {
		if unicode.IsUpper(char) {
			result.HasUppercase = true
		}
		if unicode.IsLower(char) {
			result.HasLowercase = true
		}
		if unicode.IsDigit(char) {
			result.HasDigit = true
		}
		if unicode.IsPunct(char) || unicode.IsSymbol(char) {
			result.HasSpecialChar = true
		}
	}

	// Count how many categories are satisfied
	categories := 0
	if result.HasUppercase {
		categories++
	}
	if result.HasLowercase {
		categories++
	}
	if result.HasDigit {
		categories++
	}
	if result.HasSpecialChar {
		categories++
	}

	// Build validation message
	var issues []string

	if !result.HasMinLength {
		result.Valid = false
		issues = append(issues, "minimum 8 characters required")
	}

	if categories < 3 {
		result.Valid = false
		missing := []string{}
		if !result.HasUppercase {
			missing = append(missing, "uppercase letter")
		}
		if !result.HasLowercase {
			missing = append(missing, "lowercase letter")
		}
		if !result.HasDigit {
			missing = append(missing, "digit")
		}
		if !result.HasSpecialChar {
			missing = append(missing, "special character")
		}
		issues = append(issues, "needs at least 3 of: uppercase, lowercase, digit, special char. Missing: "+strings.Join(missing, ", "))
	}

	if len(issues) > 0 {
		result.Message = "Password does not meet SQL Server complexity requirements: " + strings.Join(issues, "; ")
	} else {
		result.Message = "Password meets complexity requirements"
	}

	return result
}

// SQL Identifier Validation
// SQL Server allows identifiers that:
// - Start with a letter (a-z, A-Z), underscore (_), at sign (@), or number sign (#)
// - Subsequent characters can include letters, digits, @, $, #, or _
// - Maximum length is 128 characters
// - Cannot be a reserved keyword (we'll check common ones)

var sqlIdentifierPattern = regexp.MustCompile(`^[a-zA-Z_@#][a-zA-Z0-9_@#$]{0,127}$`)

// Common SQL Server reserved keywords
var sqlReservedKeywords = map[string]bool{
	"ADD": true, "ALL": true, "ALTER": true, "AND": true, "ANY": true, "AS": true,
	"ASC": true, "AUTHORIZATION": true, "BACKUP": true, "BEGIN": true, "BETWEEN": true,
	"BREAK": true, "BROWSE": true, "BULK": true, "BY": true, "CASCADE": true,
	"CASE": true, "CHECK": true, "CHECKPOINT": true, "CLOSE": true, "CLUSTERED": true,
	"COALESCE": true, "COLLATE": true, "COLUMN": true, "COMMIT": true, "COMPUTE": true,
	"CONSTRAINT": true, "CONTAINS": true, "CONTAINSTABLE": true, "CONTINUE": true,
	"CONVERT": true, "CREATE": true, "CROSS": true, "CURRENT": true, "CURRENT_DATE": true,
	"CURRENT_TIME": true, "CURRENT_TIMESTAMP": true, "CURRENT_USER": true, "CURSOR": true,
	"DATABASE": true, "DBCC": true, "DEALLOCATE": true, "DECLARE": true, "DEFAULT": true,
	"DELETE": true, "DENY": true, "DESC": true, "DISK": true, "DISTINCT": true,
	"DISTRIBUTED": true, "DOUBLE": true, "DROP": true, "DUMP": true, "ELSE": true,
	"END": true, "ERRLVL": true, "ESCAPE": true, "EXCEPT": true, "EXEC": true,
	"EXECUTE": true, "EXISTS": true, "EXIT": true, "EXTERNAL": true, "FETCH": true,
	"FILE": true, "FILLFACTOR": true, "FOR": true, "FOREIGN": true, "FREETEXT": true,
	"FREETEXTTABLE": true, "FROM": true, "FULL": true, "FUNCTION": true, "GOTO": true,
	"GRANT": true, "GROUP": true, "HAVING": true, "HOLDLOCK": true, "IDENTITY": true,
	"IDENTITY_INSERT": true, "IDENTITYCOL": true, "IF": true, "IN": true, "INDEX": true,
	"INNER": true, "INSERT": true, "INTERSECT": true, "INTO": true, "IS": true,
	"JOIN": true, "KEY": true, "KILL": true, "LEFT": true, "LIKE": true, "LINENO": true,
	"LOAD": true, "MERGE": true, "NATIONAL": true, "NOCHECK": true, "NONCLUSTERED": true,
	"NOT": true, "NULL": true, "NULLIF": true, "OF": true, "OFF": true, "OFFSETS": true,
	"ON": true, "OPEN": true, "OPENDATASOURCE": true, "OPENQUERY": true, "OPENROWSET": true,
	"OPENXML": true, "OPTION": true, "OR": true, "ORDER": true, "OUTER": true, "OVER": true,
	"PERCENT": true, "PIVOT": true, "PLAN": true, "PRECISION": true, "PRIMARY": true,
	"PRINT": true, "PROC": true, "PROCEDURE": true, "PUBLIC": true, "RAISERROR": true,
	"READ": true, "READTEXT": true, "RECONFIGURE": true, "REFERENCES": true, "REPLICATION": true,
	"RESTORE": true, "RESTRICT": true, "RETURN": true, "REVERT": true, "REVOKE": true,
	"RIGHT": true, "ROLLBACK": true, "ROWCOUNT": true, "ROWGUIDCOL": true, "RULE": true,
	"SAVE": true, "SCHEMA": true, "SECURITYAUDIT": true, "SELECT": true, "SEMANTICKEYPHRASETABLE": true,
	"SEMANTICSIMILARITYDETAILSTABLE": true, "SEMANTICSIMILARITYTABLE": true, "SESSION_USER": true,
	"SET": true, "SETUSER": true, "SHUTDOWN": true, "SOME": true, "STATISTICS": true,
	"SYSTEM_USER": true, "TABLE": true, "TABLESAMPLE": true, "TEXTSIZE": true, "THEN": true,
	"TO": true, "TOP": true, "TRAN": true, "TRANSACTION": true, "TRIGGER": true,
	"TRUNCATE": true, "TRY_CONVERT": true, "TSEQUAL": true, "UNION": true, "UNIQUE": true,
	"UNPIVOT": true, "UPDATE": true, "UPDATETEXT": true, "USE": true, "USER": true,
	"VALUES": true, "VARYING": true, "VIEW": true, "WAITFOR": true, "WHEN": true,
	"WHERE": true, "WHILE": true, "WITH": true, "WITHIN": true, "WRITETEXT": true,
}

// ValidateSQLIdentifier validates a SQL Server identifier (database name, AG name, etc.)
func ValidateSQLIdentifier(name string, fieldName string) *ValidationResult {
	result := NewValidationResult()

	if name == "" {
		result.AddError("%s is required", fieldName)
		return result
	}

	// Length check
	if len(name) > 128 {
		result.AddError("%s '%s' exceeds maximum length of 128 characters (has %d)",
			fieldName, truncate(name, 20), len(name))
	}

	// Pattern check
	if !sqlIdentifierPattern.MatchString(name) {
		result.AddError("%s '%s' contains invalid characters. SQL identifiers must start with a letter, _, @, or # "+
			"and contain only letters, digits, @, $, #, or _", fieldName, truncate(name, 20))
	}

	// Reserved keyword check (warning only)
	if sqlReservedKeywords[strings.ToUpper(name)] {
		result.AddWarning("%s '%s' is a SQL Server reserved keyword. Consider using a different name", fieldName, name)
	}

	return result
}

// ValidateDatabaseName validates a SQL Server database name
func ValidateDatabaseName(name string) *ValidationResult {
	return ValidateSQLIdentifier(name, "Database name")
}

// ValidateAGName validates a SQL Server Availability Group name
func ValidateAGName(name string) *ValidationResult {
	return ValidateSQLIdentifier(name, "Availability Group name")
}

// SanitizeSQLIdentifier escapes a SQL identifier for safe use in dynamic SQL
// Uses QUOTENAME-style escaping: wraps in brackets and escapes internal brackets
func SanitizeSQLIdentifier(name string) string {
	// First validate
	if !sqlIdentifierPattern.MatchString(name) {
		// If invalid, return empty to prevent any injection
		return ""
	}

	// Escape any existing brackets (though pattern shouldn't allow them)
	escaped := strings.ReplaceAll(name, "]", "]]")

	// Wrap in brackets
	return "[" + escaped + "]"
}

// ValidatePath validates a file path for security issues
func ValidatePath(path string, fieldName string) *ValidationResult {
	result := NewValidationResult()

	if path == "" {
		return result // Empty path might be valid depending on context
	}

	// Check for path traversal
	if strings.Contains(path, "..") {
		result.AddError("%s '%s' contains path traversal sequence '..' which is not allowed",
			fieldName, truncate(path, 30))
	}

	// Check for null bytes (path injection)
	if strings.Contains(path, "\x00") {
		result.AddError("%s contains null byte which is not allowed", fieldName)
	}

	// Check for shell metacharacters
	dangerousChars := []string{";", "|", "&", "$", "`", "(", ")", "<", ">", "\n", "\r"}
	for _, char := range dangerousChars {
		if strings.Contains(path, char) {
			result.AddError("%s contains potentially dangerous character '%s'", fieldName, char)
		}
	}

	return result
}

// ValidateEnvironmentVariableName validates an environment variable name
func ValidateEnvironmentVariableName(name string) *ValidationResult {
	result := NewValidationResult()

	if name == "" {
		result.AddError("Environment variable name is required")
		return result
	}

	// Check for valid env var name pattern
	pattern := regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
	if !pattern.MatchString(name) {
		result.AddError("Environment variable name '%s' is invalid. Must start with letter or underscore, "+
			"followed by letters, digits, or underscores", truncate(name, 20))
	}

	return result
}

// ValidateEnvironmentVariableValue checks for potential injection in env var values
func ValidateEnvironmentVariableValue(value string, name string) *ValidationResult {
	result := NewValidationResult()

	// Check for command substitution patterns
	if strings.Contains(value, "$(") || strings.Contains(value, "`") {
		result.AddWarning("Environment variable '%s' contains command substitution pattern, ensure this is intentional",
			name)
	}

	// Check for excessive length
	if len(value) > 32768 {
		result.AddError("Environment variable '%s' value exceeds maximum length of 32KB", name)
	}

	return result
}

// DetectSQLInjection checks a string for common SQL injection patterns
func DetectSQLInjection(input string, fieldName string) *ValidationResult {
	result := NewValidationResult()

	if input == "" {
		return result
	}

	// Convert to lowercase for pattern matching
	lower := strings.ToLower(input)

	// Common SQL injection patterns
	injectionPatterns := []struct {
		pattern string
		desc    string
	}{
		{"';", "single quote followed by semicolon"},
		{"--", "SQL comment"},
		{"/*", "SQL block comment start"},
		{"*/", "SQL block comment end"},
		{"xp_", "extended stored procedure prefix"},
		{"sp_", "system stored procedure prefix"},
		{"exec(", "EXEC call"},
		{"execute(", "EXECUTE call"},
		{"drop ", "DROP statement"},
		{"delete ", "DELETE statement"},
		{"insert ", "INSERT statement"},
		{"update ", "UPDATE statement"},
		{"union ", "UNION statement"},
		{"select ", "SELECT statement"},
		{"1=1", "tautology"},
		{"1 = 1", "tautology with spaces"},
		{"or 1=1", "OR tautology"},
		{"' or '", "OR injection"},
		{"<script", "script tag"},
	}

	for _, p := range injectionPatterns {
		if strings.Contains(lower, p.pattern) {
			result.AddError("%s contains potential SQL injection pattern (%s): '%s'",
				fieldName, p.desc, p.pattern)
		}
	}

	return result
}

// ValidateNoShellMetacharacters checks for shell metacharacters that could enable injection
func ValidateNoShellMetacharacters(input string, fieldName string) *ValidationResult {
	result := NewValidationResult()

	dangerousChars := []string{";", "|", "&", "$", "`", "(", ")", "{", "}", "<", ">", "\\", "\n", "\r", "'", "\""}

	for _, char := range dangerousChars {
		if strings.Contains(input, char) {
			result.AddError("%s contains shell metacharacter '%s' which could enable injection", fieldName, char)
		}
	}

	return result
}
