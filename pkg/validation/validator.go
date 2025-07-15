package validation

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
)

var (
	// Custom validator instance
	validate = validator.New()

	// Regex patterns for validation
	tickerPattern = regexp.MustCompile(`^[A-Z0-9]{1,10}$`)
	sectorPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,50}$`)
	sourcePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,100}$`)
)

// ValidationError represents a validation error with field and message
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Value   interface{} `json:"value,omitempty"`
}

// ValidationErrors is a collection of validation errors
type ValidationErrors []ValidationError

// Error implements the error interface
func (ve ValidationErrors) Error() string {
	if len(ve) == 0 {
		return ""
	}
	
	var messages []string
	for _, err := range ve {
		messages = append(messages, fmt.Sprintf("%s: %s", err.Field, err.Message))
	}
	return strings.Join(messages, "; ")
}

// Register custom validators
func init() {
	// Register custom validators
	validate.RegisterValidation("ticker", validateTicker)
	validate.RegisterValidation("sector", validateSector)
	validate.RegisterValidation("source", validateSource)
	validate.RegisterValidation("price", validatePrice)
	validate.RegisterValidation("timestamp", validateTimestamp)
	validate.RegisterValidation("zscore", validateZScore)
}

// validateTicker validates ticker symbol format
func validateTicker(fl validator.FieldLevel) bool {
	ticker, ok := fl.Field().Interface().(string)
	if !ok {
		return false
	}
	return tickerPattern.MatchString(ticker)
}

// validateSector validates sector name format
func validateSector(fl validator.FieldLevel) bool {
	sector, ok := fl.Field().Interface().(string)
	if !ok {
		return false
	}
	return sectorPattern.MatchString(sector)
}

// validateSource validates source name format
func validateSource(fl validator.FieldLevel) bool {
	source, ok := fl.Field().Interface().(string)
	if !ok {
		return false
	}
	return sourcePattern.MatchString(source)
}

// validatePrice validates price is positive and reasonable
func validatePrice(fl validator.FieldLevel) bool {
	price, ok := fl.Field().Interface().(float64)
	if !ok {
		return false
	}
	// Price must be positive and less than 1 million
	return price > 0 && price < 1000000
}

// validateTimestamp validates timestamp is recent and reasonable
func validateTimestamp(fl validator.FieldLevel) bool {
	timestamp, ok := fl.Field().Interface().(int64)
	if !ok {
		return false
	}
	
	// Convert to time
	t := time.UnixMilli(timestamp)
	now := time.Now()
	
	// Timestamp should be within last 24 hours and not in the future
	return t.After(now.Add(-24*time.Hour)) && !t.After(now)
}

// validateZScore validates z-score is reasonable
func validateZScore(fl validator.FieldLevel) bool {
	zscore, ok := fl.Field().Interface().(float64)
	if !ok {
		return false
	}
	// Z-score should be positive and reasonable (not astronomical)
	return zscore >= 0 && zscore < 100
}

// ValidateStruct validates a struct using tags
func ValidateStruct(s interface{}) ValidationErrors {
	err := validate.Struct(s)
	if err == nil {
		return nil
	}

	var errors ValidationErrors
	for _, err := range err.(validator.ValidationErrors) {
		field := err.Field()
		tag := err.Tag()
		value := err.Value()
		
		message := getErrorMessage(field, tag, value)
		errors = append(errors, ValidationError{
			Field:   field,
			Message: message,
			Value:   value,
		})
	}
	
	return errors
}

// getErrorMessage returns a user-friendly error message
func getErrorMessage(field, tag string, value interface{}) string {
	switch tag {
	case "required":
		return fmt.Sprintf("%s is required", field)
	case "ticker":
		return fmt.Sprintf("%s must be a valid ticker symbol (1-10 uppercase letters/numbers)", field)
	case "sector":
		return fmt.Sprintf("%s must be a valid sector name (1-50 alphanumeric characters)", field)
	case "source":
		return fmt.Sprintf("%s must be a valid source name (1-100 alphanumeric characters)", field)
	case "price":
		return fmt.Sprintf("%s must be a positive price less than 1,000,000", field)
	case "timestamp":
		return fmt.Sprintf("%s must be a recent timestamp within the last 24 hours", field)
	case "zscore":
		return fmt.Sprintf("%s must be a positive z-score less than 100", field)
	case "min":
		return fmt.Sprintf("%s must be at least %v", field, value)
	case "max":
		return fmt.Sprintf("%s must be at most %v", field, value)
	case "email":
		return fmt.Sprintf("%s must be a valid email address", field)
	case "url":
		return fmt.Sprintf("%s must be a valid URL", field)
	default:
		return fmt.Sprintf("%s failed validation: %s", field, tag)
	}
}

// ValidateMap validates a map[string]interface{} against expected schema
func ValidateMap(data map[string]interface{}, schema map[string]string) ValidationErrors {
	var errors ValidationErrors
	
	for field, expectedType := range schema {
		value, exists := data[field]
		if !exists {
			errors = append(errors, ValidationError{
				Field:   field,
				Message: fmt.Sprintf("%s is required", field),
			})
			continue
		}
		
		if err := validateFieldType(field, value, expectedType); err != nil {
			errors = append(errors, *err)
		}
	}
	
	return errors
}

// validateFieldType validates a field's type and value
func validateFieldType(field string, value interface{}, expectedType string) *ValidationError {
	switch expectedType {
	case "string":
		if _, ok := value.(string); !ok {
			return &ValidationError{
				Field:   field,
				Message: fmt.Sprintf("%s must be a string", field),
				Value:   value,
			}
		}
	case "float64":
		switch v := value.(type) {
		case float64:
			// Additional validation for specific fields
			if field == "price" && (v <= 0 || v >= 1000000) {
				return &ValidationError{
					Field:   field,
					Message: "price must be positive and less than 1,000,000",
					Value:   value,
				}
			}
		case string:
			if _, err := strconv.ParseFloat(v, 64); err != nil {
				return &ValidationError{
					Field:   field,
					Message: fmt.Sprintf("%s must be a valid number", field),
					Value:   value,
				}
			}
		default:
			return &ValidationError{
				Field:   field,
				Message: fmt.Sprintf("%s must be a number", field),
				Value:   value,
			}
		}
	case "int64":
		switch v := value.(type) {
		case int64:
			// Additional validation for timestamps
			if field == "timestamp" || field == "ts_ms" {
				t := time.UnixMilli(v)
				now := time.Now()
				if t.After(now) || t.Before(now.Add(-24*time.Hour)) {
					return &ValidationError{
						Field:   field,
						Message: "timestamp must be recent and not in the future",
						Value:   value,
					}
				}
			}
		case string:
			if _, err := strconv.ParseInt(v, 10, 64); err != nil {
				return &ValidationError{
					Field:   field,
					Message: fmt.Sprintf("%s must be a valid integer", field),
					Value:   value,
				}
			}
		case float64:
			// Allow float64 for timestamp fields (common in JSON)
			if field == "timestamp" || field == "ts_ms" {
				t := time.UnixMilli(int64(v))
				now := time.Now()
				if t.After(now) || t.Before(now.Add(-24*time.Hour)) {
					return &ValidationError{
						Field:   field,
						Message: "timestamp must be recent and not in the future",
						Value:   value,
					}
				}
			}
		default:
			return &ValidationError{
				Field:   field,
				Message: fmt.Sprintf("%s must be an integer", field),
				Value:   value,
			}
		}
	}
	
	return nil
}

// SanitizeString removes potentially dangerous characters
func SanitizeString(s string) string {
	// Remove null bytes and control characters
	s = strings.Map(func(r rune) rune {
		if r < 32 && r != 9 && r != 10 && r != 13 { // Keep tab, newline, carriage return
			return -1
		}
		return r
	}, s)
	
	// Trim whitespace
	return strings.TrimSpace(s)
}

// SanitizePrice ensures price is within reasonable bounds
func SanitizePrice(price float64) float64 {
	if price <= 0 {
		return 0.01 // Minimum valid price
	}
	if price > 1000000 {
		return 1000000 // Maximum reasonable price
	}
	return price
}

// SanitizeTimestamp ensures timestamp is recent and valid
func SanitizeTimestamp(timestamp int64) int64 {
	t := time.UnixMilli(timestamp)
	now := time.Now()
	
	// If timestamp is in the future, use current time
	if t.After(now) {
		return now.UnixMilli()
	}
	
	// If timestamp is too old (more than 24 hours), use current time
	if t.Before(now.Add(-24 * time.Hour)) {
		return now.UnixMilli()
	}
	
	return timestamp
} 