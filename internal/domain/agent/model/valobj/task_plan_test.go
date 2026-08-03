package valobj

import "testing"

func TestAdvanceWithTool(t *testing.T) {
	p := &TaskPlan{
		Summary: "test",
		SubTasks: []SubTask{
			{Index: 1, Title: "list", ExpectedTools: "list_files", Status: "pending"},
			{Index: 2, Title: "read", ExpectedTools: "read_file", Status: "pending"},
		},
	}
	if !p.AdvanceWithTool("list_files", "ok") {
		t.Fatal("should advance")
	}
	if p.SubTasks[0].Status != "done" {
		t.Fatalf("got %s", p.SubTasks[0].Status)
	}
	_ = p.StartNext()
	if p.SubTasks[1].Status != "running" {
		t.Fatalf("got %s", p.SubTasks[1].Status)
	}
	if !p.AdvanceWithTool("read_file", "content") {
		t.Fatal("should advance second")
	}
	if !p.AllDone() {
		t.Fatal("should all done")
	}
}
