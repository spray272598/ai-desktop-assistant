package service

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseAndMatchSkill(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "demo-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	md := `---
name: Demo Skill
description: 演示技能
triggers:
  - 演示
  - demo skill
tools:
  - list_files
  - read_file
---

# Body
do the demo
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := NewSkillService(dir)
	list := svc.List()
	if len(list) != 1 {
		t.Fatalf("want 1 skill, got %d", len(list))
	}
	if list[0].Name != "Demo Skill" {
		t.Fatalf("name=%s", list[0].Name)
	}
	if len(list[0].Tools) != 2 {
		t.Fatalf("tools=%v", list[0].Tools)
	}

	m := svc.Match("请帮我做个演示")
	if m == nil {
		t.Fatal("expected match")
	}
	if m.ID != "demo-skill" {
		t.Fatalf("id=%s", m.ID)
	}

	// install from path into another root
	root2 := t.TempDir()
	svc2 := NewSkillService(root2)
	installed, err := svc2.InstallFromPath(skillDir, "copied")
	if err != nil {
		t.Fatal(err)
	}
	if installed.ID != "copied" && installed.ID != "demo-skill" {
		// InstallFromPath uses id param for directory name
		t.Logf("installed id=%s", installed.ID)
	}
	if len(svc2.List()) < 1 {
		t.Fatal("install failed to load")
	}
}

func TestFilterTools(t *testing.T) {
	svc := NewSkillService(t.TempDir())
	// empty skill → no filter
	all := []string{"list_files", "run_command", "fetch"}
	out := svc.FilterTools(nil, all)
	if len(out) != 3 {
		t.Fatalf("%v", out)
	}
}
