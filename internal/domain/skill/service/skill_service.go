package service

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ai-desktop/assistant/internal/domain/skill/model"
)

// SkillService 扫描 skills/、安装/卸载、匹配与 Prompt 片段
type SkillService struct {
	mu      sync.RWMutex
	rootDir string // 项目 skills 根目录（内置+已安装）
	skills  map[string]*model.Skill
}

func NewSkillService(rootDir string) *SkillService {
	if rootDir == "" {
		rootDir = "./skills"
	}
	s := &SkillService{
		rootDir: rootDir,
		skills:  make(map[string]*model.Skill),
	}
	_ = s.Reload()
	return s
}

func (s *SkillService) RootDir() string { return s.rootDir }

// Reload 扫描目录加载全部 SKILL.md
func (s *SkillService) Reload() error {
	if err := os.MkdirAll(s.rootDir, 0o755); err != nil {
		return err
	}
	loaded := make(map[string]*model.Skill)
	entries, err := os.ReadDir(s.rootDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(s.rootDir, e.Name())
		sk, err := parseSkillDir(dir)
		if err != nil || sk == nil {
			continue
		}
		sk.Source = "installed"
		// 内置示例可标 builtin（目录名或 frontmatter）
		if strings.HasPrefix(sk.ID, "builtin-") || fileExists(filepath.Join(dir, ".builtin")) {
			sk.Source = "builtin"
		}
		sk.Enabled = true
		loaded[sk.ID] = sk
	}
	s.mu.Lock()
	s.skills = loaded
	s.mu.Unlock()
	return nil
}

func (s *SkillService) List() []*model.Skill {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*model.Skill, 0, len(s.skills))
	for _, sk := range s.skills {
		cp := *sk
		out = append(out, &cp)
	}
	return out
}

func (s *SkillService) Get(id string) *model.Skill {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sk := s.skills[id]
	if sk == nil {
		return nil
	}
	cp := *sk
	return &cp
}

// Match 按用户输入选最高分 Skill（阈值以下视为无匹配）
func (s *SkillService) Match(userInput string) *model.Skill {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var best *model.Skill
	bestScore := 0
	// 显式 /skill-id 或 使用 xxx skill
	lower := strings.ToLower(userInput)
	for id, sk := range s.skills {
		if strings.Contains(lower, "/"+strings.ToLower(id)) ||
			strings.Contains(lower, "使用 "+strings.ToLower(sk.Name)) ||
			strings.Contains(lower, "use skill "+strings.ToLower(id)) {
			cp := *sk
			return &cp
		}
		sc := sk.MatchScore(userInput)
		if sc > bestScore {
			bestScore = sc
			cp := *sk
			best = &cp
		}
	}
	if bestScore < 15 {
		return nil
	}
	return best
}

// InstallFromPath 从外部目录复制到 skills/<id>
func (s *SkillService) InstallFromPath(srcPath string, id string) (*model.Skill, error) {
	srcPath = filepath.Clean(srcPath)
	st, err := os.Stat(srcPath)
	if err != nil {
		return nil, fmt.Errorf("source not found: %w", err)
	}
	var srcDir string
	if st.IsDir() {
		srcDir = srcPath
	} else if strings.EqualFold(filepath.Base(srcPath), "SKILL.md") {
		srcDir = filepath.Dir(srcPath)
	} else {
		return nil, fmt.Errorf("source must be a skill directory or SKILL.md")
	}
	sk, err := parseSkillDir(srcDir)
	if err != nil {
		return nil, err
	}
	if id == "" {
		id = sk.ID
	}
	id = sanitizeID(id)
	dst := filepath.Join(s.rootDir, id)
	if err := copyDir(srcDir, dst); err != nil {
		return nil, err
	}
	// 确保 ID 与目录一致
	if sk.ID != id {
		// rewrite not strictly needed; reload will parse
	}
	if err := s.Reload(); err != nil {
		return nil, err
	}
	installed := s.Get(id)
	if installed == nil {
		// try parsed id
		installed = s.Get(sk.ID)
	}
	if installed == nil {
		return nil, fmt.Errorf("install ok but skill not loaded")
	}
	return installed, nil
}

// Uninstall 删除 skills/<id>（builtin 也可删，便于学习演示）
func (s *SkillService) Uninstall(id string) error {
	id = sanitizeID(id)
	if id == "" {
		return fmt.Errorf("id required")
	}
	dst := filepath.Join(s.rootDir, id)
	if _, err := os.Stat(dst); err != nil {
		return fmt.Errorf("skill not found: %s", id)
	}
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	return s.Reload()
}

// PromptSection 生成注入 system 的 Skill 段落
func (s *SkillService) PromptSection(sk *model.Skill) string {
	if sk == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("## 已激活 Skill: ")
	b.WriteString(sk.Name)
	b.WriteString(" (")
	b.WriteString(sk.ID)
	b.WriteString(")\n")
	if sk.Description != "" {
		b.WriteString(sk.Description)
		b.WriteString("\n")
	}
	if len(sk.Tools) > 0 {
		b.WriteString("优先使用工具: ")
		b.WriteString(strings.Join(sk.Tools, ", "))
		b.WriteString("\n")
	}
	if sk.Body != "" {
		b.WriteString("\n### Skill 执行指南\n")
		b.WriteString(sk.Body)
		b.WriteString("\n")
	}
	return b.String()
}

// FilterTools 若 Skill 声明了 tools，则只保留这些（及名称子串匹配）；空声明不限制
func (s *SkillService) FilterTools(sk *model.Skill, all []string) []string {
	if sk == nil || len(sk.Tools) == 0 {
		return all
	}
	allow := map[string]bool{}
	for _, t := range sk.Tools {
		allow[strings.ToLower(t)] = true
	}
	var out []string
	for _, name := range all {
		ln := strings.ToLower(name)
		if allow[ln] {
			out = append(out, name)
			continue
		}
		for a := range allow {
			if strings.Contains(ln, a) || strings.Contains(a, ln) {
				out = append(out, name)
				break
			}
		}
	}
	if len(out) == 0 {
		return all // 避免空工具集卡死
	}
	return out
}

// --- parsing ---

func parseSkillDir(dir string) (*model.Skill, error) {
	mdPath := filepath.Join(dir, "SKILL.md")
	data, err := os.ReadFile(mdPath)
	if err != nil {
		return nil, err
	}
	meta, body := splitFrontmatter(string(data))
	id := filepath.Base(dir)
	sk := &model.Skill{
		ID:      id,
		Name:    id,
		Body:    strings.TrimSpace(body),
		Path:    dir,
		Enabled: true,
	}
	if v := meta["name"]; v != "" {
		sk.Name = v
	}
	if v := meta["id"]; v != "" {
		sk.ID = sanitizeID(v)
	}
	if v := meta["description"]; v != "" {
		sk.Description = v
	}
	if v := meta["triggers"]; v != "" {
		sk.Triggers = splitCSV(v)
	}
	if v := meta["tools"]; v != "" {
		sk.Tools = splitCSV(v)
	}
	// YAML-like list under keys is simplified as comma/line in frontmatter values
	return sk, nil
}

func splitFrontmatter(content string) (map[string]string, string) {
	content = strings.TrimSpace(content)
	meta := map[string]string{}
	if !strings.HasPrefix(content, "---") {
		return meta, content
	}
	rest := content[3:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return meta, content
	}
	fm := rest[:end]
	body := strings.TrimSpace(rest[end+4:])
	// also allow ---\r\n
	body = strings.TrimPrefix(body, "\n")
	body = strings.TrimPrefix(body, "\r\n")

	var currentKey string
	var listVals []string
	flushList := func() {
		if currentKey != "" && len(listVals) > 0 {
			meta[currentKey] = strings.Join(listVals, ",")
			listVals = nil
		}
	}
	for _, line := range strings.Split(fm, "\n") {
		line = strings.TrimRight(line, "\r")
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		if strings.HasPrefix(trim, "- ") && currentKey != "" {
			listVals = append(listVals, strings.TrimSpace(trim[2:]))
			continue
		}
		flushList()
		if i := strings.Index(line, ":"); i > 0 {
			key := strings.ToLower(strings.TrimSpace(line[:i]))
			val := strings.TrimSpace(line[i+1:])
			val = strings.Trim(val, `"'`)
			currentKey = key
			if val == "" || val == "|" || val == ">" {
				listVals = nil
				continue
			}
			meta[key] = val
			currentKey = key
		}
	}
	flushList()
	return meta, body
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n'
	}) {
		p = strings.TrimSpace(p)
		p = strings.Trim(p, `"'`)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func sanitizeID(id string) string {
	id = strings.TrimSpace(id)
	id = strings.ReplaceAll(id, " ", "-")
	var b strings.Builder
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
