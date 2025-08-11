package hermes_test

import (
	"testing"
	"time"

	"lucas/internal/hermes"
)

func TestGenerateMessageID(t *testing.T) {
	t.Run("UniqueIDs", func(t *testing.T) {
		id1 := hermes.GenerateMessageID()
		time.Sleep(1 * time.Millisecond) // Ensure different timestamps
		id2 := hermes.GenerateMessageID()

		if id1 == id2 {
			t.Error("Expected unique message IDs")
		}

		if id1 == "" || id2 == "" {
			t.Error("Expected non-empty message IDs")
		}
	})

	t.Run("MessageIDFormat", func(t *testing.T) {
		id := hermes.GenerateMessageID()

		if len(id) == 0 {
			t.Error("Expected message ID to have content")
		}

		// Test that multiple calls generate different IDs
		ids := make(map[string]bool)
		for i := 0; i < 10; i++ {
			msgID := hermes.GenerateMessageID()
			if ids[msgID] {
				t.Errorf("Generated duplicate message ID: %s", msgID)
				break
			}
			ids[msgID] = true
			time.Sleep(1 * time.Millisecond) // Add delay to ensure different timestamps
		}
	})
}

func BenchmarkGenerateMessageID(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hermes.GenerateMessageID()
	}
}