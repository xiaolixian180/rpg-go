package scheduler

import (
	"time"

	"hero-quest/internal/service"
)

// RegisterAutoSaveTask registers a periodic save task (every 60s)
// that persists all dirty online players to the database.
func RegisterAutoSaveTask(s *Scheduler, gm *service.GameManager) {
	s.Add("auto_save", 60*time.Second, func() {
		gm.SaveAllDirty()
	})
}
