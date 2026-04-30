package presidio

const (
	SentinelBinaryDataRemoved        = "<binary data removed>"
	SentinelAccessDenied             = "<access denied>"
	SentinelRedactedKnown            = "<redacted: %s>"
	SentinelRedacted                 = "<redacted>"
	SentinelContentTruncated         = "<content truncated at %d>"
	SentinelContentTruncatedRedacted = "<content truncated and redacted>"
	SentinelContentNotScanned        = "<content not scanned: %s>"
)

func SentinelRedactedFor(entity EntityType) string {
	return "<redacted: " + string(entity) + ">"
}

func SentinelContentTruncatedAt(length int) string {
	return "<content truncated at " + itoa(length) + ">"
}

func SentinelContentNotScannedFor(binaryType string) string {
	return "<content not scanned: " + binaryType + ">"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
