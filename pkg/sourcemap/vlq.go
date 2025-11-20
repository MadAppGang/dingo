package sourcemap

// VLQ (Variable Length Quantity) encoding for source maps
// Based on the Source Map v3 specification

const (
	vlqBaseShift = 5
	vlqBase      = 1 << vlqBaseShift // 32
	vlqBaseMask  = vlqBase - 1       // 31
	vlqContinuationBit = vlqBase     // 32
)

var base64Chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

// encodeVLQ encodes a single integer as VLQ
func encodeVLQ(value int) string {
	var encoded string

	// Convert to unsigned with sign bit as LSB
	var vlq int
	if value < 0 {
		vlq = ((-value) << 1) | 1
	} else {
		vlq = value << 1
	}

	// Encode as base64 VLQ
	for {
		digit := vlq & vlqBaseMask
		vlq >>= vlqBaseShift

		if vlq > 0 {
			// More digits to come, set continuation bit
			digit |= vlqContinuationBit
		}

		encoded += string(base64Chars[digit])

		if vlq == 0 {
			break
		}
	}

	return encoded
}

// encodeVLQSegment encodes a mapping segment (up to 5 values)
func encodeVLQSegment(values []int) string {
	var result string
	for _, val := range values {
		result += encodeVLQ(val)
	}
	return result
}

// generateVLQMappings generates VLQ-encoded mappings string
func generateVLQMappings(mappings []Mapping) string {
	if len(mappings) == 0 {
		return ""
	}

	var result string
	var prevGenLine = 0
	var prevGenColumn = 0
	var prevSourceIndex = 0
	var prevSourceLine = 0
	var prevSourceColumn = 0

	for _, m := range mappings {
		// Each new generated line is separated by ';'
		for prevGenLine < m.GenLine-1 {
			result += ";"
			prevGenLine++
			prevGenColumn = 0 // Column resets for each line
		}

		// Same line, add comma separator if not first on line
		if prevGenLine == m.GenLine-1 && len(result) > 0 && result[len(result)-1] != ';' {
			result += ","
		}

		// Build segment values (relative to previous)
		// For single source file, source index is always 0
		sourceIndex := 0

		segment := []int{
			m.GenColumn - 1 - prevGenColumn,      // Generated column (delta)
			sourceIndex - prevSourceIndex,         // Source file index (delta)
			m.SourceLine - 1 - prevSourceLine,     // Original line (delta)
			m.SourceColumn - 1 - prevSourceColumn, // Original column (delta)
		}

		// TODO: Add name index if m.Name is set (5th field)
		// For now, we only encode the 4-field version

		result += encodeVLQSegment(segment)

		// Update previous values
		prevGenLine = m.GenLine - 1
		prevGenColumn = m.GenColumn - 1
		prevSourceIndex = sourceIndex
		prevSourceLine = m.SourceLine - 1
		prevSourceColumn = m.SourceColumn - 1
	}

	return result
}
