package reducer

import "testing"

func TestHybridReducerKeepsRecent(t *testing.T) {
	msgs := make([]map[string]interface{}, 0, 20)
	for i := 0; i < 20; i++ {
		msgs = append(msgs, map[string]interface{}{
			"id":       string(rune('a'+i%26)) + string(rune('0'+i/10)),
			"role":     "user",
			"content":  "message content number " + string(rune('0'+i%10)) + " padding padding padding",
			"priority": 1,
		})
	}
	// unique ids
	for i := range msgs {
		msgs[i]["id"] = "m-" + itoa(i)
		msgs[i]["content"] = "msg-" + itoa(i) + " " + longText(50)
	}

	r := NewHybridReducer()
	out := r.Reduce(msgs, 200)
	if len(out) < 2 {
		t.Fatalf("expected at least 2 messages, got %d", len(out))
	}
	// last message must be kept
	last := out[len(out)-1]
	if last["id"] != "m-19" {
		t.Fatalf("expected last id m-19, got %v", last["id"])
	}
}

func TestSlidingWindow(t *testing.T) {
	msgs := []map[string]interface{}{
		{"id": "1", "role": "user", "content": longText(100)},
		{"id": "2", "role": "assistant", "content": longText(100)},
		{"id": "3", "role": "user", "content": "short"},
	}
	r := NewSlidingWindowReducer()
	out := r.Reduce(msgs, 50)
	if len(out) == 0 {
		t.Fatal("empty result")
	}
}

func longText(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'x'
	}
	return string(b)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
