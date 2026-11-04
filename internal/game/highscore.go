package game

// updateHighScore ports 1DB4EE. In the supplied Final Cut build both the
// palette and background remain black throughout this interlude. Its text
// call is disabled (1DCA74 is RTS); the 660-update wait still cannot be skipped.
func (w *NativeWorld) updateHighScore() error {
	switch w.Life.Phase {
	case LifeHighScoreFade:
		w.Life.Age++
		if w.Life.Age == 16 {
			return w.beginHighScore()
		}
	case LifeHighScore:
		if w.Life.Age >= 659 {
			w.Life.Phase = LifeHighScoreExit
			w.Life.Age = 0
			return nil
		}
		w.Life.Age++
		w.set("tick", (w.get("tick")+1)&255)
		return w.lifeActors()
	case LifeHighScoreExit:
		w.Life.Age++
		if w.Life.Age == 16 {
			return w.finishDeathScene()
		}
	}
	return nil
}
func (w *NativeWorld) beginHighScore() error {
	w.stopAudio()
	w.clearLifeScene()
	w.PaletteOverrides = nil
	w.PaletteBank2 = 0
	w.set("invulnerability", 0)
	w.set("new_high_score", 0)
	w.Life = LifeCycle{Phase: LifeHighScore}
	w.set("message_id", 4) // 1DCA74 is disabled in this particular ROM.
	w.set("tick", 255)
	if e := w.lifeActors(); e != nil {
		return e
	}
	w.set("tick", 0)
	return w.lifeActors()
}
