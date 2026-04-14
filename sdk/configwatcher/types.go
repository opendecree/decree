package configwatcher

import (
	"fmt"
	"strconv"
	"time"

	pb "github.com/zeevdr/central-config-service/api/centralconfig/v1"
)

func parseString(s string) (string, error) {
	return s, nil
}

func parseInt(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

func parseFloat(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

func parseBool(s string) (bool, error) {
	return strconv.ParseBool(s)
}

func parseDuration(s string) (time.Duration, error) {
	return time.ParseDuration(s)
}

// typedValueToString extracts a string representation from a TypedValue.
func typedValueToString(tv *pb.TypedValue) string {
	if tv == nil {
		return ""
	}
	switch v := tv.Kind.(type) {
	case *pb.TypedValue_StringValue:
		return v.StringValue
	case *pb.TypedValue_IntegerValue:
		return fmt.Sprintf("%d", v.IntegerValue)
	case *pb.TypedValue_NumberValue:
		return strconv.FormatFloat(v.NumberValue, 'f', -1, 64)
	case *pb.TypedValue_BoolValue:
		return strconv.FormatBool(v.BoolValue)
	case *pb.TypedValue_UrlValue:
		return v.UrlValue
	case *pb.TypedValue_JsonValue:
		return v.JsonValue
	case *pb.TypedValue_TimeValue:
		if v.TimeValue != nil {
			return v.TimeValue.AsTime().Format(time.RFC3339Nano)
		}
		return ""
	case *pb.TypedValue_DurationValue:
		if v.DurationValue != nil {
			return v.DurationValue.AsDuration().String()
		}
		return ""
	default:
		return ""
	}
}
