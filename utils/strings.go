package utils

func StringOrDefault(str, defaultValue string) string {
	if str != "" {
		return str
	}

	return defaultValue
}
