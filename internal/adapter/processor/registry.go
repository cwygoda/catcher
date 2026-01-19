package processor

import "github.com/cwygoda/catcher/internal/domain"

// Registry holds registered URL processors.
type Registry struct {
	processors []domain.URLProcessor
}

// NewRegistry creates a new processor registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Register adds a processor to the registry.
func (r *Registry) Register(p domain.URLProcessor) {
	r.processors = append(r.processors, p)
}

// Match returns the first processor that matches the URL, or nil.
func (r *Registry) Match(url string) domain.URLProcessor {
	for _, p := range r.processors {
		if p.Match(url) {
			return p
		}
	}
	return nil
}

// Processors returns all registered processors.
func (r *Registry) Processors() []domain.URLProcessor {
	return r.processors
}
