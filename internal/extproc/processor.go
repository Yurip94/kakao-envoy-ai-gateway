package extproc

import (
	"github.com/Yurip94/kakao-envoy-ai-gateway/internal/memory"
)

type Processor struct {
	Config Config
	Store  memory.Store
}

func NewProcessor(cfg Config, store memory.Store) *Processor {
	return &Processor{
		Config: cfg,
		Store:  store,
	}
}
