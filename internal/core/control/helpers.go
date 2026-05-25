package control

import "strings"

func NormalizeRequestOptionID(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "-", "")
	normalized = strings.ReplaceAll(normalized, "_", "")
	normalized = strings.ReplaceAll(normalized, " ", "")
	switch normalized {
	case "accept", "allow", "approve", "yes":
		return "accept"
	case "acceptforsession", "allowforsession", "allowthissession", "session":
		return "acceptForSession"
	case "decline", "deny", "reject", "no":
		return "decline"
	case "revise", "suggestalternative", "tellclaudewhattodo", "tellclaudewhattododifferently":
		return "revise"
	case "capturefeedback", "feedback", "tellcodexwhattodo", "tellcodexwhattododifferently":
		return "captureFeedback"
	default:
		return strings.TrimSpace(value)
	}
}

func ShortenThreadID(threadID string) string {
	parts := strings.Split(threadID, "-")
	if len(parts) >= 2 {
		head := strings.TrimSpace(parts[1])
		tail := strings.TrimSpace(parts[len(parts)-1])
		if len(tail) > 4 {
			tail = tail[len(tail)-4:]
		}
		switch {
		case head == "":
		case tail == "":
			return head
		case head == tail:
			return head
		default:
			return head + "…" + tail
		}
	}
	if len(threadID) <= 10 {
		return threadID
	}
	return threadID[len(threadID)-8:]
}
