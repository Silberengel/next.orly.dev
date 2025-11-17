package find

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	ErrInvalidName         = errors.New("invalid name format")
	ErrNameTooLong         = errors.New("name exceeds 253 characters")
	ErrLabelTooLong        = errors.New("label exceeds 63 characters")
	ErrLabelEmpty          = errors.New("label is empty")
	ErrInvalidCharacter    = errors.New("invalid character in name")
	ErrInvalidHyphen       = errors.New("label cannot start or end with hyphen")
	ErrAllNumericLabel     = errors.New("label cannot be all numeric")
	ErrInvalidRecordValue  = errors.New("invalid record value")
	ErrRecordLimitExceeded = errors.New("record limit exceeded")
	ErrNotOwner            = errors.New("not the name owner")
	ErrNameExpired         = errors.New("name registration expired")
	ErrInRenewalWindow     = errors.New("name is in renewal window")
	ErrNotRenewalWindow    = errors.New("not in renewal window")
)

// Name format validation regex
var (
	labelRegex = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)
	allNumeric = regexp.MustCompile(`^[0-9]+$`)
)

// NormalizeName converts a name to lowercase
func NormalizeName(name string) string {
	return strings.ToLower(name)
}

// ValidateName validates a name according to DNS naming rules
func ValidateName(name string) error {
	// Normalize to lowercase
	name = NormalizeName(name)

	// Check total length
	if len(name) > 253 {
		return fmt.Errorf("%w: %d > 253", ErrNameTooLong, len(name))
	}

	if len(name) == 0 {
		return fmt.Errorf("%w: name is empty", ErrInvalidName)
	}

	// Split into labels
	labels := strings.Split(name, ".")

	for i, label := range labels {
		if err := validateLabel(label); err != nil {
			return fmt.Errorf("invalid label %d (%s): %w", i, label, err)
		}
	}

	return nil
}

// validateLabel validates a single label according to DNS rules
func validateLabel(label string) error {
	// Check length
	if len(label) == 0 {
		return ErrLabelEmpty
	}
	if len(label) > 63 {
		return fmt.Errorf("%w: %d > 63", ErrLabelTooLong, len(label))
	}

	// Check character set and hyphen placement
	if !labelRegex.MatchString(label) {
		if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return ErrInvalidHyphen
		}
		return ErrInvalidCharacter
	}

	// Check not all numeric
	if allNumeric.MatchString(label) {
		return ErrAllNumericLabel
	}

	return nil
}

// GetParentDomain returns the parent domain of a name
// e.g., "www.example.com" -> "example.com", "example.com" -> "com", "com" -> ""
func GetParentDomain(name string) string {
	name = NormalizeName(name)
	parts := strings.Split(name, ".")
	if len(parts) <= 1 {
		return "" // TLD has no parent
	}
	return strings.Join(parts[1:], ".")
}

// IsTLD returns true if the name is a top-level domain (single label)
func IsTLD(name string) bool {
	name = NormalizeName(name)
	return !strings.Contains(name, ".")
}

// ValidateIPv4 validates an IPv4 address format
func ValidateIPv4(ip string) error {
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return fmt.Errorf("%w: invalid IPv4 format", ErrInvalidRecordValue)
	}

	for _, part := range parts {
		var octet int
		if _, err := fmt.Sscanf(part, "%d", &octet); err != nil {
			return fmt.Errorf("%w: invalid IPv4 octet: %v", ErrInvalidRecordValue, err)
		}
		if octet < 0 || octet > 255 {
			return fmt.Errorf("%w: IPv4 octet out of range: %d", ErrInvalidRecordValue, octet)
		}
	}

	return nil
}

// ValidateIPv6 validates an IPv6 address format (simplified check)
func ValidateIPv6(ip string) error {
	// Basic validation - contains colons and valid hex characters
	if !strings.Contains(ip, ":") {
		return fmt.Errorf("%w: invalid IPv6 format", ErrInvalidRecordValue)
	}

	// Split by colons
	parts := strings.Split(ip, ":")
	if len(parts) < 3 || len(parts) > 8 {
		return fmt.Errorf("%w: invalid IPv6 segment count", ErrInvalidRecordValue)
	}

	// Check for valid hex characters
	validHex := regexp.MustCompile(`^[0-9a-fA-F]*$`)
	for _, part := range parts {
		if part == "" {
			continue // Allow :: notation
		}
		if len(part) > 4 {
			return fmt.Errorf("%w: IPv6 segment too long", ErrInvalidRecordValue)
		}
		if !validHex.MatchString(part) {
			return fmt.Errorf("%w: invalid IPv6 hex", ErrInvalidRecordValue)
		}
	}

	return nil
}

// ValidateRecordValue validates a record value based on its type
func ValidateRecordValue(recordType, value string) error {
	switch recordType {
	case RecordTypeA:
		return ValidateIPv4(value)
	case RecordTypeAAAA:
		return ValidateIPv6(value)
	case RecordTypeCNAME, RecordTypeMX, RecordTypeNS:
		return ValidateName(value)
	case RecordTypeTXT:
		if len(value) > 1024 {
			return fmt.Errorf("%w: TXT record exceeds 1024 characters", ErrInvalidRecordValue)
		}
		return nil
	case RecordTypeSRV:
		return ValidateName(value) // Hostname for SRV
	default:
		return fmt.Errorf("%w: unknown record type: %s", ErrInvalidRecordValue, recordType)
	}
}

// ValidateRecordLimit checks if adding a record would exceed type limits
func ValidateRecordLimit(recordType string, currentCount int) error {
	limit, ok := RecordLimits[recordType]
	if !ok {
		return fmt.Errorf("%w: unknown record type: %s", ErrInvalidRecordValue, recordType)
	}

	if currentCount >= limit {
		return fmt.Errorf("%w: %s records limited to %d", ErrRecordLimitExceeded, recordType, limit)
	}

	return nil
}

// ValidatePriority validates priority value (0-65535)
func ValidatePriority(priority int) error {
	if priority < 0 || priority > 65535 {
		return fmt.Errorf("%w: priority must be 0-65535", ErrInvalidRecordValue)
	}
	return nil
}

// ValidateWeight validates weight value (0-65535)
func ValidateWeight(weight int) error {
	if weight < 0 || weight > 65535 {
		return fmt.Errorf("%w: weight must be 0-65535", ErrInvalidRecordValue)
	}
	return nil
}

// ValidatePort validates port value (0-65535)
func ValidatePort(port int) error {
	if port < 0 || port > 65535 {
		return fmt.Errorf("%w: port must be 0-65535", ErrInvalidRecordValue)
	}
	return nil
}

// ValidateTrustScore validates trust score (0.0-1.0)
func ValidateTrustScore(score float64) error {
	if score < 0.0 || score > 1.0 {
		return fmt.Errorf("trust score must be between 0.0 and 1.0, got %f", score)
	}
	return nil
}
