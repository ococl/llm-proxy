package logging

import (
	"sync"
)

var ColorPalette = []string{
	"\033[96m", "\033[93m", "\033[92m", "\033[95m",
	"\033[94m", "\033[91m", "\033[36m", "\033[33m",
	"\033[32m", "\033[35m", "\033[34m",
}

type RequestColorManager struct {
	mu              sync.RWMutex
	colorIndex      uint32
	recentReqColors map[string]uint32
	maxRecent       int
}

var globalColorManager = &RequestColorManager{
	recentReqColors: make(map[string]uint32),
	maxRecent:       20,
}

func (m *RequestColorManager) GetRequestColor(reqID string) string {
	if reqID == "" {
		return ""
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if colorIdx, exists := m.recentReqColors[reqID]; exists {
		return ColorPalette[colorIdx]
	}

	colorIdx := m.colorIndex % uint32(len(ColorPalette))
	m.colorIndex++

	m.recentReqColors[reqID] = colorIdx

	if len(m.recentReqColors) > m.maxRecent {
		for k := range m.recentReqColors {
			delete(m.recentReqColors, k)
			break
		}
	}

	return ColorPalette[colorIdx]
}

func GetGlobalColorManager() *RequestColorManager {
	return globalColorManager
}
