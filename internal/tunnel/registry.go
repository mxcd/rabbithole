package tunnel

import (
	"sync"
)

type Registry struct {
	tunnels sync.Map // subdomain → *Tunnel
}

func NewRegistry() *Registry {
	return &Registry{}
}

func (r *Registry) Register(subdomain string, t *Tunnel) {
	r.tunnels.Store(subdomain, t)
}

func (r *Registry) Deregister(subdomain string) {
	r.tunnels.Delete(subdomain)
}

func (r *Registry) Get(subdomain string) (*Tunnel, bool) {
	val, ok := r.tunnels.Load(subdomain)
	if !ok {
		return nil, false
	}
	return val.(*Tunnel), true
}

func (r *Registry) Count() int {
	count := 0
	r.tunnels.Range(func(_, _ any) bool {
		count++
		return true
	})
	return count
}

type TunnelInfo struct {
	Subdomain    string `json:"subdomain"`
	APIKeyLabel  string `json:"apiKeyLabel"`
	CreatedAt    string `json:"createdAt"`
	RequestCount int64  `json:"requestCount"`
	LastRequest  string `json:"lastRequest"`
}

func (r *Registry) List() []TunnelInfo {
	var tunnels []TunnelInfo
	r.tunnels.Range(func(key, value any) bool {
		t := value.(*Tunnel)
		info := TunnelInfo{
			Subdomain:    t.Subdomain,
			APIKeyLabel:  t.APIKeyLabel,
			CreatedAt:    t.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			RequestCount: t.RequestCount.Load(),
		}
		if lastReq := t.LastRequest.Load(); lastReq != nil {
			info.LastRequest = lastReq.(string)
		}
		tunnels = append(tunnels, info)
		return true
	})
	return tunnels
}
