package intent

import "testing"

func TestRuleClassifierListFiles(t *testing.T) {
	c := NewRuleClassifier()
	r := c.Classify("列出当前目录下的文件", nil)
	if r.Intent != "LIST_FILES" {
		t.Fatalf("want LIST_FILES got %s", r.Intent)
	}
	if !r.HasHighConfidence(0.8) {
		t.Fatalf("confidence too low: %v", r.Confidence)
	}
}

func TestRuleClassifierChatFallback(t *testing.T) {
	c := NewRuleClassifier()
	r := c.Classify("今天天气怎么样", nil)
	if r.Intent != "CHAT" {
		t.Fatalf("want CHAT got %s", r.Intent)
	}
}
