package game

import "aladdin/internal/choreography"

func (w *NativeWorld) retainArtwork(o *choreography.Object) {
	if o == nil || !o.ArtworkValid {
		return
	}
	for i := o.ArtworkStart; i < o.ArtworkStart+o.ArtworkCount && i < len(w.ArtworkLeaks); i++ {
		if i >= 0 {
			w.ArtworkLeaks[i] = true
		}
	}
}

// ReserveArtwork preserves the game's 116-block first-fit artwork budget
// (1D7B16), without console memory or rendering hardware. Rendering uses the
// decoded PNG atlas; this limit matters because exhaustion deletes actors.
func (w *NativeWorld) ReserveArtwork(o *choreography.Object) bool {
	if o == nil || !o.Alive() {
		return false
	}
	if o.ArtworkValid {
		return true
	}
	used := w.ArtworkLeaks
	mark := func(v *choreography.Object, pool *[116]bool) {
		if v == nil || !v.ArtworkValid {
			return
		}
		for i := v.ArtworkStart; i < v.ArtworkStart+v.ArtworkCount && i < len(pool); i++ {
			if i >= 0 {
				pool[i] = true
			}
		}
	}
	for _, v := range w.Objects {
		if v != nil && v.Alive() {
			mark(v, &used)
		}
	}
	count := int(o.Record[41]) + 1
	for start := 0; start+count <= len(used); start++ {
		free := true
		for _, occupied := range used[start : start+count] {
			if occupied {
				free = false
				break
			}
		}
		if free {
			o.ArtworkStart, o.ArtworkCount, o.ArtworkValid = start, count, true
			return true
		}
	}
	// This failure path restores markers and clears both linked actors. The
	// original does not release an already allocated partner's blocks here.
	restore := func(v *choreography.Object) {
		if v.Record[52] != 0 {
			w.Activated[int(uint16(v.Word(50)))] = false
		}
		v.Record[0] = 0
	}
	restore(o)
	if o.Partner != nil {
		w.retainArtwork(o.Partner)
		restore(o.Partner)
	}
	return false
}
