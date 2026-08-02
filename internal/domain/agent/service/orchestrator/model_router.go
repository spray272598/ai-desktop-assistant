package orchestrator

import (
	"strings"
	"sync"
)

// ModelProfile 模型配置档
type ModelProfile struct {
	Name     string  `json:"name"`
	APIBase  string  `json:"apiBase,omitempty"`
	APIKey   string  `json:"apiKey,omitempty"`
	Weight   int     `json:"weight"` // A/B 权重
	Scenarios []string `json:"scenarios"`
}

// ModelRouter 场景 → 模型 A/B 切换
type ModelRouter struct {
	mu       sync.RWMutex
	defaultM string
	profiles map[string]ModelProfile // name -> profile
	// scenario -> model names
	routes map[string][]string
	// 简单轮询计数做 A/B
	counter map[string]int
}

func NewModelRouter(defaultModel string) *ModelRouter {
	r := &ModelRouter{
		defaultM: defaultModel,
		profiles: make(map[string]ModelProfile),
		routes:   make(map[string][]string),
		counter:  make(map[string]int),
	}
	// 内置场景路由
	r.routes["chat"] = []string{"fast", "default"}
	r.routes["complex"] = []string{"strong", "default"}
	r.routes["code"] = []string{"strong", "default"}
	r.routes["browser"] = []string{"fast", "default"}
	r.routes["file"] = []string{"fast"}
	r.routes["command"] = []string{"fast"}
	return r
}

func (r *ModelRouter) Register(p ModelProfile) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p.Name == "" {
		return
	}
	if p.Weight <= 0 {
		p.Weight = 1
	}
	r.profiles[p.Name] = p
	for _, sc := range p.Scenarios {
		sc = strings.ToLower(sc)
		r.routes[sc] = appendUnique(r.routes[sc], p.Name)
	}
}

func (r *ModelRouter) SetDefault(name string) {
	r.mu.Lock()
	r.defaultM = name
	r.mu.Unlock()
}

// Select 按场景选择模型（加权轮询 A/B）
func (r *ModelRouter) Select(scenario string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	scenario = strings.ToLower(scenario)
	cands := r.routes[scenario]
	if len(cands) == 0 {
		if r.defaultM != "" {
			return r.defaultM
		}
		return "default"
	}
	// 展开权重
	var pool []string
	for _, name := range cands {
		w := 1
		if p, ok := r.profiles[name]; ok {
			w = p.Weight
		}
		for i := 0; i < w; i++ {
			pool = append(pool, name)
		}
	}
	if len(pool) == 0 {
		return r.defaultM
	}
	idx := r.counter[scenario] % len(pool)
	r.counter[scenario]++
	return pool[idx]
}

func (r *ModelRouter) List() []ModelProfile {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ModelProfile, 0, len(r.profiles))
	for _, p := range r.profiles {
		out = append(out, p)
	}
	return out
}

func (r *ModelRouter) Routes() map[string][]string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cp := make(map[string][]string, len(r.routes))
	for k, v := range r.routes {
		cp[k] = append([]string(nil), v...)
	}
	return cp
}

func appendUnique(ss []string, v string) []string {
	for _, s := range ss {
		if s == v {
			return ss
		}
	}
	return append(ss, v)
}
