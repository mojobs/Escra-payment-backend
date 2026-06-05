package money

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Amount stores fiat money in the smallest currency unit.
// For NGN this means kobo. Keeping this as an integer avoids float rounding.
type Amount int64

const (
	Zero Amount = 0
)

var ErrInvalidAmount = errors.New("invalid money amount")

func FromMinorUnits(value int64) Amount {
	return Amount(value)
}

func (a Amount) MinorUnits() int64 {
	return int64(a)
}

func (a Amount) IsPositive() bool {
	return a > 0
}

func (a Amount) String() string {
	major := int64(a) / 100
	minor := int64(a) % 100
	if minor < 0 {
		minor = -minor
	}
	return fmt.Sprintf("%d.%02d", major, minor)
}

func ParseDecimal(value string) (Amount, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, ErrInvalidAmount
	}

	sign := int64(1)
	if strings.HasPrefix(value, "-") {
		sign = -1
		value = strings.TrimPrefix(value, "-")
	}

	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" {
		return 0, ErrInvalidAmount
	}

	major, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, ErrInvalidAmount
	}

	minor := int64(0)
	if len(parts) == 2 {
		fraction := parts[1]
		if len(fraction) > 2 {
			return 0, ErrInvalidAmount
		}
		if len(fraction) == 1 {
			fraction += "0"
		}
		if fraction != "" {
			minor, err = strconv.ParseInt(fraction, 10, 64)
			if err != nil {
				return 0, ErrInvalidAmount
			}
		}
	}

	return Amount(sign * ((major * 100) + minor)), nil
}

func (a Amount) MarshalJSON() ([]byte, error) {
	return []byte(strconv.FormatInt(int64(a), 10)), nil
}

func (a *Amount) UnmarshalJSON(data []byte) error {
	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	switch value := raw.(type) {
	case string:
		amount, err := ParseDecimal(value)
		if err != nil {
			return err
		}
		*a = amount
		return nil
	case float64:
		if value != float64(int64(value)) {
			return errors.New("amount numbers must be integer minor units; use a quoted decimal such as \"1000.00\" for naira")
		}
		*a = Amount(int64(value))
		return nil
	default:
		return ErrInvalidAmount
	}
}

func (a Amount) Value() (driver.Value, error) {
	return int64(a), nil
}

func (a *Amount) Scan(value interface{}) error {
	switch v := value.(type) {
	case int64:
		*a = Amount(v)
	case int:
		*a = Amount(v)
	case float64:
		if v != float64(int64(v)) {
			return ErrInvalidAmount
		}
		*a = Amount(int64(v))
	case []byte:
		raw := string(v)
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			// Fallback: DB may return decimal-formatted string (e.g. "0.00")
			amount, err2 := ParseDecimal(raw)
			if err2 != nil {
				if legacyScaleDecimal(raw) {
					return fmt.Errorf("legacy decimal money value %q detected; migrate money columns from decimal major units to bigint minor units", raw)
				}
				return err
			}
			*a = amount
		} else {
			*a = Amount(parsed)
		}
	case string:
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			// Fallback: DB may return decimal-formatted string (e.g. "0.00")
			amount, err2 := ParseDecimal(v)
			if err2 != nil {
				if legacyScaleDecimal(v) {
					return fmt.Errorf("legacy decimal money value %q detected; migrate money columns from decimal major units to bigint minor units", v)
				}
				return err
			}
			*a = amount
		} else {
			*a = Amount(parsed)
		}
	case nil:
		*a = Zero
	default:
		return fmt.Errorf("unsupported money amount type %T", value)
	}
	return nil
}

func legacyScaleDecimal(value string) bool {
	value = strings.TrimSpace(strings.TrimPrefix(value, "-"))
	parts := strings.Split(value, ".")
	return len(parts) == 2 && len(parts[1]) > 2
}
