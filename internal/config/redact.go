package config

const redactedSentinel = "[REDACTED]"

func redactIfSensitive(sensitive bool, value string) string {
	if sensitive {
		return redactedSentinel
	}
	return value
}
