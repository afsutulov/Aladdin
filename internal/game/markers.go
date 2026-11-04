package game

import (
	"aladdin/internal/choreography"
	"aladdin/internal/data"
	"fmt"
)

type markerRecipe struct {
	template, pool, kind, animation, motion, tiles int
	offsetX, offsetY                               int
	stationary                                     bool
	flipX, keepMarker                              bool
	paletteBank2                                   int
	copies                                         [][2]int
	flags                                          byte
	overrideFlags                                  bool
	palette                                        int
	spawnFlag                                      string
}

// markerRecipe ports the indicated native stage constructors. Unsupported
// markers stay in the scene data for subsequent conversion.
func (w *NativeWorld) markerRecipe(kind int) (markerRecipe, bool) {
	r := markerRecipe{pool: 3, tiles: -1}
	switch kind {
	case 0x4d:
		r.template, r.paletteBank2 = 0x1e357a, 0x151cf4
		r.spawnFlag = "flag_f646"
	case 0x57:
		r.template, r.pool = 0x1e317e, 1
	case 0x3d:
		r.template, r.pool = 0x1e2e72, 0
	case 0x47:
		r.template, r.pool, r.offsetY = 0x1e3052, 0, -32
	case 0x65:
		if !w.flag("flag_f628") {
			return r, false
		}
		return w.markerRecipe(0x12)
	case 0x82:
		if w.flag("flag_f63f") {
			return r, false
		}
		r.template, r.pool, r.offsetX, r.offsetY = 0x1e2e4a, 1, 13, -12
	case 0xbb, 0xa9:
		r.template, r.pool = 0x1e367e, 1 // 1E2076 / 1E2088
		r.flipX = kind == 0xbb
	case 0xa6:
		r.template, r.pool = 0x1e3692, 1
	case 0x5b, 0x5d, 0xcb, 0xcc, 0xcd, 0xce:
		r.template, r.pool = 0x1e3836, 1
		r.animation = map[int]int{0x5b: 0x14d618, 0x5d: 0x14d6d2, 0xcb: 0x14d5e6, 0xcc: 0x14d5f0, 0xcd: 0x14d5fa, 0xce: 0x14d604}[kind]
		if (kind == 0x5b || kind == 0x5d) && w.get("difficulty") == 0 {
			r.animation = 0x14d5e6
		}
	case 0x17:
		r.template, r.offsetX, r.offsetY = 0x1e385e, 24, -14
	case 0xfd, 0xfe:
		r.template, r.pool = 0x1e3b42, 1
		r.offsetX, r.offsetY = 20, -1
		if kind == 0xfe {
			r.offsetX, r.offsetY = 11, 6
		}
	case 0x43:
		if w.get("difficulty") != 0 {
			return r, false
		}
		return w.markerRecipe(0x40)
	case 0xa4:
		if w.get("difficulty") == 0 {
			return r, false
		}
		r.template, r.pool, r.offsetY = 0x1e35ca, 1, 8
	case 0x20, 0x21, 0x38:
		r.template, r.pool, r.offsetY = 0x1e33ae, 2, 10
		if kind == 0x21 {
			r.offsetX = 24
		} else if kind == 0x38 {
			r.offsetY = 18
		}
	case 0x35, 0x36, 0x37:
		r.template, r.pool, r.offsetY = 0x1e371e, 1, 16
		r.kind = map[int]int{0x35: 0x1e, 0x36: 0x1f, 0x37: 0x20}[kind]
		if kind == 0x35 {
			r.offsetX = 16
		} else if kind == 0x36 {
			r.flipX = true
		}
	case 0xef, 0xf4:
		if kind == 0xf4 && !w.flag("flag_f630") {
			return r, false
		}
		r.template, r.pool, r.offsetY, r.keepMarker = 0x1e36f6, 3, -8, true
		if w.Stage.Index == 5 {
			r.template = 0x1e370a
		}
	case 0xee:
		r.template, r.pool, r.offsetX = 0x1e36e2, 1, 16
	case 0xd6:
		r.template, r.pool = 0x1e348a, 3
	case 0xd8:
		r.template, r.pool = 0x1e353e, 1
	case 0x2f:
		r.template = 0x1e38ea
		if !w.flag("flag_f6a5") {
			r.paletteBank2 = 0x1519d4
		}
	case 0x30:
		r.template, r.offsetY = 0x1e3b7e, 12
	case 0x3c:
		r.template = 0x1e3976
		if !w.flag("flag_f6a5") {
			if w.Stage.Index == 5 {
				r.palette = 0x6000
			} else {
				r.paletteBank2 = 0x151c14
			}
		}
	case 0x3e:
		if w.get("difficulty") == 0 {
			return r, false
		}
		r.template = 0x1e39c6
		if !w.flag("flag_f6a5") {
			r.paletteBank2 = 0x151c54
		}
	case 0x6b:
		r.template, r.kind, r.animation, r.tiles = 0x1e2f9e, 0x3d, 0x14bc8e, 2
		r.flags, r.overrideFlags = 0x21, true
	case 0xf5, 0xf6:
		if w.flag("flag_f6a5") {
			return r, false
		}
		r.template, r.pool, r.paletteBank2 = 0x1e366a, 2, 0x151a14
		r.animation = 0x14c8b8
		if kind == 0xf6 {
			r.animation = 0x14c8f6
		}
	case 6, 7, 8:
		r.template, r.pool = 0x1e3a2a, 2
		r.animation = map[int]int{6: 0x14e062, 7: 0x14e082, 8: 0x14e0a2}[kind]
	case 0x19:
		r.template, r.kind, r.animation, r.stationary = 0x1e3246, 0x26, 0x14b4d2, true
	case 0x1c:
		r.template = 0x1e3746
	case 0x23, 0x24, 0x25:
		r.template = 0x1e3066
		if kind == 0x25 {
			r.template = 0x1e30a2
		}
		if !w.flag("flag_f6a5") {
			r.paletteBank2 = 0x1519d4
		}
	case 0x75, 0x76:
		r.template, r.pool = 0x1e37e6, 2
		if kind == 0x76 {
			r.animation = 0x14d382
		}
	case 0x8f:
		if w.Stage.Index == 11 {
			if w.flag("flag_f646") {
				return r, false
			}
			r.template, r.pool = 0x1e34c6, 2
			if !w.flag("flag_f6a5") {
				w.PaletteBank2 = 0x151d34
			}
			break
		}
		r.template, r.pool = 0x1e34b2, 2
	case 0x18:
		w.Sound(0x32, false)
		r.template, r.pool = 0x1e3192, 1
	case 0x16:
		r.template, r.pool, r.flipX = 0x1e3282, 0, true
	case 0x1e:
		r.template, r.animation = 0x1e3822, 0x14d4c8
	case 0x28:
		_, y := w.PlayerPosition()
		if w.flag("flag_f6ab") || uint16(y) < 0x25d || uint16(y) >= 0x308 {
			return r, false
		}
		r.template, r.paletteBank2 = 0x1e361a, 0x151874
	case 0x64:
		if !w.flag("flag_f629") {
			return r, false
		}
		return w.markerRecipe(0x11)
	case 0x6c:
		r.template, r.pool = 0x1e311a, 2
	case 0x70:
		if w.flag("flag_f622") {
			return r, false
		}
		r.template, r.pool, r.kind, r.animation = 0x1e344e, 2, 0x73, 0x14bdb2
		r.offsetX, r.offsetY = 15, -1
	case 0xd0:
		r.template, r.pool = 0x1e30f2, 1
	case 0xc9:
		r.template, r.kind, r.animation, r.tiles = 0x1e2f9e, 0x8c, 0x14bb06, 2
		r.flags = 0x21
		r.overrideFlags = true
		r.pool = 2
	case 0xca:
		r.template, r.pool, r.animation, r.offsetY = 0x1e2e86, 1, 0x14c0c6, -8
	case 0xb6, 0xb7, 0xb8, 0xb9, 0xba:
		visible := false
		switch kind {
		case 0xb6:
			visible = w.flag("flag_f649") && !w.flag("flag_f64a") && !w.flag("flag_f64b")
		case 0xb7:
			visible = w.flag("flag_f64a") && !w.flag("flag_f64b")
		case 0xb8:
			visible = w.flag("flag_f64b")
		case 0xb9:
			visible = w.flag("flag_f64c")
		case 0xba:
			visible = w.flag("flag_f64d")
		}
		if !visible {
			return r, false
		}
		r.template, r.pool, r.offsetY = 0x1e2e86, 1, -8
		r.animation = map[int]int{0xb6: 0x14c044, 0xb7: 0x14c05e, 0xb8: 0x14c078, 0xb9: 0x14c092, 0xba: 0x14c0ac}[kind]
	case 0xb1, 0xb2, 0xb3, 0xb4, 9, 0xb5:
		if kind == 0xb4 && !w.flag("flag_f6ab") {
			return r, false
		}
		r.template, r.kind, r.animation, r.stationary, r.tiles = 0x1e2f9e, 0x4d, 0x14bac2, true, 1
		r.kind = map[int]int{0xb1: 0x4d, 0xb2: 0x4e, 0xb3: 0x4f, 0xb4: 0x50, 9: 0x51, 0xb5: 0x52}[kind]
		if kind == 0xb1 {
			r.pool = 1
		}
	case 0x61:
		if w.get("apples_ascii") != 0x3030 {
			return r, false
		}
		r.template, r.kind, r.animation, r.tiles = 0x1e2f9e, 0x46, 0x14a56c, 0
		r.keepMarker = true
		r.copies = [][2]int{{12, 0}, {-12, 0}, {0, -10}, {0, 10}}
	case 0x12:
		r.template = 0x1e321e // 1E1842
	case 0x11:
		r.template = 0x1e3232 // 1E1858
	case 0x10, 0x67:
		r.template = 0x1e3246 // 1E186E / 1E18D4
	case 0x1b:
		r.template = 0x1e321e
		r.kind = 0x24
		r.animation = 0x14af04
		r.stationary = true
	case 0x1a:
		r.template = 0x1e3232
		r.kind = 0x25
		r.animation = 0x14b136
		r.stationary = true
	case 0x62:
		if !w.flag("flag_f654") {
			return r, false
		}
		r.template = 0x1e3246
	case 0x6a:
		if !w.flag("flag_f654") {
			return r, false
		}
		r.template = 0x1e3232
	case 0x60:
		if w.get("apples_ascii") == 0x3939 {
			return r, false
		}
		r.template = 0x1e2f9e
		r.kind = 0x46
		r.animation = 0x14a56c
		r.tiles = 0
	case 0x51:
		if w.get("gems_ascii") == 0x3939 {
			return r, false
		}
		r.template = 0x1e2f9e
		r.kind = 0x40
		r.animation = 0x14a532
		r.tiles = 1
	case 0x58:
		if !w.flag("flag_f654") {
			return r, false
		}
		return w.markerRecipe(0x51)
	case 0x53:
		r.template = 0x1e2f9e
		r.kind = 0x3a
		r.animation = 0x14a578
		r.motion = 0x148d50
		r.tiles = 6
	case 0x50:
		r.template = 0x1e2f9e
		r.kind = 0x4a
		r.animation = 0x14a59a
		r.tiles = 1
	case 0x40:
		r.template = 0x1e2fc6
		r.pool = 2 // 1E1C6C
	case 0x74:
		r.template = 0x1e352a
		r.pool = 1
		r.offsetX = -8
		r.offsetY = 4 // 1E10B2
	case 0x14:
		r.template = 0x1e3822 // 1E1998
	case 0x5c:
		r.template = 0x1e3142
		r.pool = 1
		r.offsetX = 9
		r.offsetY = 7 // 1E121C
	case 0x0d, 0x2b:
		r.template = 0x1e3462
		if kind == 0x2b {
			r.template = 0x1e344e
		}
		r.pool = 2
		r.offsetX = 8
		r.offsetY = -1 // 1E11AE / 1E11C8
	case 0x87:
		r.template, r.pool = 0x1e3016, 1 // 1E206A: springboard
	case 0x46:
		r.template, r.pool = 0x1e3052, 0 // 1E15FA
	case 0x45:
		r.template, r.pool = 0x1e37be, 1 // 1E10CC
	case 0x0a:
		r.template, r.pool, r.animation = 0x1e39ee, 1, 0x14e046
	case 0xaa:
		r.template, r.pool = 0x1e32aa, 2
	case 0x68:
		r.template = 0x1e3232
	case 0x4b:
		r.template, r.pool = 0x1e3aa2, 1
	case 0x7e:
		r.template, r.pool, r.offsetX, r.offsetY = 0x1e3002, 1, 8, 12
	case 0xad:
		if !w.flag("flag_f6a1") {
			return r, false
		}
		return w.markerRecipe(0x60)
	case 0xaf:
		if !w.flag("flag_f6a2") {
			return r, false
		}
		return w.markerRecipe(0x55)
	case 0x5f, 0x41, 0x42:
		first, second := w.flag("scarab_first"), w.flag("scarab_second")
		if kind == 0x5f && first || kind == 0x41 && (!first || second) || kind == 0x42 && (!first || !second) {
			return r, false
		}
		r.template, r.pool, r.kind = 0x1e2f9e, 0, 0x44
		if kind == 0x41 {
			r.kind = 0x45
		} else if kind == 0x42 {
			r.kind = 0x42
		}
		r.animation, r.motion, r.tiles = 0x14c112, 0x148870, 1
	case 0xea:
		r.template, r.pool = 0x1e3872, 1 // 1E1A46: lamp projectile pickup
	case 0x55:
		if w.get("lives") == 9 {
			return r, false
		}
		r.template, r.pool = 0x1e2fb2, 0 // 1E1FBC
	case 0xc8:
		if w.flag("flag_f6a8") {
			return r, false
		}
		r.template, r.kind, r.animation, r.tiles = 0x1e2f9e, 0x47, 0x14e462, 2
	case 0x80:
		w.set("flag_f626", 0)
		r.template, r.pool = 0x1e325a, 1
	case 0x13:
		r.template = 0x1e362e
	case 0x6d:
		if !w.flag("flag_f654") || w.get("difficulty") == 0 {
			return r, false
		}
		r.template = 0x1e3822
	case 0xab:
		if !w.flag("flag_f69f") {
			return r, false // The alternate constructor is an intentional RTS.
		}
		r.template, r.kind, r.animation, r.tiles = 0x1e2f9e, 0x4a, 0x14a59a, 1
	default:
		return r, false
	}
	return r, true
}

// EnableStageActors starts the converted portion of the original marker
// constructors. The test harness can leave activation off to isolate actors.
func (w *NativeWorld) EnableStageActors() {
	w.StageActors = true
	w.Activated = map[int]bool{}
	// Final Cut's stage initializer (1E2D60) removes this map marker.
	if w.Stage.Index == 1 && len(w.Stage.Room) > 0x54ba/2 {
		w.Activated[int(w.Stage.Room[0x54ba/2])/2] = true
	}
	w.LastActivationX = -1
	w.LastActivationY = -1
}

func (w *NativeWorld) activateMarkers() error {
	if !w.StageActors || w.Stage.Index == 8 {
		return nil
	}
	cx, cy := w.get("camera_tile_x"), w.get("camera_tile_y")
	if cx == w.LastActivationX && cy == w.LastActivationY {
		return nil
	}
	ox, oy := w.LastActivationX, w.LastActivationY
	w.LastActivationX, w.LastActivationY = cx, cy
	if ox < 0 || oy < 0 {
		return w.activateMarkerRegion(cx, cy, cx+352, cy+240)
	}
	// 1D509A streams the horizontal edge before the vertical edge. The
	// column therefore uses the previous coarse Y; the row uses the new X.
	// Non-consuming markers may legitimately be visited by both passes.
	if cx != ox {
		x := cx
		if cx > ox {
			x += 352
		}
		if err := w.activateMarkerRegion(x, oy, x, oy+240); err != nil {
			return err
		}
	}
	if cy != oy {
		y := cy
		if cy > oy {
			y += 240
		}
		return w.activateMarkerRegion(cx, y, cx+352, y)
	}
	return nil
}

func (w *NativeWorld) activateMarkerRegion(left, top, right, bottom int) error {
	for _, m := range w.Stage.Entities {
		if m.X < left || m.X > right || m.Y < top || m.Y > bottom {
			continue
		}
		if w.RemovedCells[(m.Y/16)*(w.Stage.Width/16)+m.X/16] {
			continue
		}
		var resolved bool
		m, resolved = w.resolveMarker(m)
		if !resolved {
			continue
		}
		i := (m.Y/16)*(w.Stage.Width/16) + m.X/16
		key := int(w.Stage.Room[i]) / 2
		if w.Activated[key] {
			continue
		}
		if handled, err := w.escapeMarker(m); handled || err != nil {
			if err != nil {
				return err
			}
			continue
		}
		if handled, err := w.caveMarker(m); handled || err != nil {
			if err != nil {
				return err
			}
			continue
		}
		if handled, err := w.palaceCarpetMarker(m); handled || err != nil {
			if err != nil {
				return err
			}
			continue
		}
		r, ok := w.markerRecipe(m.Type)
		if !ok {
			continue
		}
		if _, err := w.spawnMarker(m, r); err != nil {
			return err
		}
	}
	return nil
}

func (w *NativeWorld) spawnMarker(m data.Marker, r markerRecipe) ([]*choreography.Object, error) {
	key := int(w.Stage.Room[(m.Y/16)*(w.Stage.Width/16)+m.X/16]) / 2
	var spawned []*choreography.Object

	_, ok := w.Scripts.Templates[r.template]
	if !ok {
		return nil, fmt.Errorf("unconverted stage template %06x", r.template)
	}
	copies := r.copies
	if copies == nil {
		copies = [][2]int{{r.offsetX, r.offsetY}}
	}
	for _, offset := range copies {
		index := w.markerSlot(r.pool)
		if index < 0 {
			break
		}
		o := w.actorTemplate(r.template, w.Objects[index])
		o.SetWord(2, m.X-16+offset[0])
		o.SetWord(4, m.Y+240+offset[1])
		if r.kind != 0 {
			o.Record[0] = byte(r.kind)
		}
		if r.animation != 0 {
			o.Animation = r.animation
		}
		if r.motion != 0 {
			o.Motion = r.motion
		}
		if r.stationary {
			o.Motion = 0
		}
		if r.flipX {
			o.Record[9] = 255
		}
		if r.overrideFlags {
			o.Record[6] = r.flags
		}
		if r.tiles >= 0 {
			o.Record[41] = byte(r.tiles)
		}
		o.SetWord(50, key)
		o.Record[52] = byte(m.Type)
		if r.palette != 0 {
			o.SetWord(30, r.palette)
		}
		w.Objects[index] = o
		if r.spawnFlag != "" {
			w.set(r.spawnFlag, 255)
		}
		spawned = append(spawned, o)
		if !r.keepMarker {
			w.Activated[key] = true
		}
		if r.paletteBank2 != 0 {
			w.PaletteBank2 = r.paletteBank2
		}
	}
	return spawned, nil
}

// The proxy constructors at 1E1ECC activate another map cell without
// consuming the proxy. Its target may be outside the current viewport.
func (w *NativeWorld) resolveMarker(m data.Marker) (data.Marker, bool) {
	seen := map[int]bool{}
	for {
		switch m.Type {
		case 0x77:
			m.Y += 80
		case 0x79:
			m.X += 32
		case 0x7a:
			m.X -= 32
		default:
			return m, true
		}
		if m.X < 0 || m.Y < 0 || m.X >= w.Stage.Width || m.Y >= w.Stage.Height {
			return m, false
		}
		i := (m.Y/16)*(w.Stage.Width/16) + m.X/16
		if i >= len(w.Stage.Room) {
			return m, false
		}
		key := int(w.Stage.Room[i]) / 2
		if key+3 >= len(w.Stage.Floor) || seen[key] || w.Activated[key] {
			return m, false
		}
		seen[key] = true
		m.Type = int(w.Stage.Floor[key+3])
	}
}

func (w *NativeWorld) markerSlot(pool int) int {
	lo, hi, step := 1, 24, 1
	switch pool {
	case 1:
		lo, hi, step = 20, 1, -1
	case 2:
		lo, hi, step = 24, 1, -1
	case 3:
		lo, hi = 3, 22
	}
	for i := lo; ; i += step {
		if w.Objects[i] == nil || !w.Objects[i].Alive() {
			return i
		}
		if i == hi {
			break
		}
	}
	return -1
}

func (w *NativeWorld) replaceActor(o *choreography.Object, template int) error {
	_, ok := w.Scripts.Templates[template]
	if !ok {
		return fmt.Errorf("unconverted replacement template %06x", template)
	}
	x, y := o.Position()
	*o = *w.actorTemplate(template, o)
	o.SetWord(2, x)
	o.SetWord(4, y)
	return nil
}

func (w *NativeWorld) despawn(o *choreography.Object) {
	if o.Record[6]&32 != 0 && o.Record[52] != 0 && o.Record[52] != 255 {
		w.Activated[int(uint16(o.Word(50)))] = false
	}
	w.Delete(o, true)
}

func (w *NativeWorld) outsideActorWindow(o *choreography.Object) bool {
	if o.Record[6]&8 != 0 || o.Record[60]&2 != 0 {
		return false
	}
	x, y := o.Position()
	cx, cy := w.get("camera_x"), w.get("camera_y")
	return uint16(y-176) < uint16(cy) || uint16(cy+304) < uint16(y-256) || uint16(x+80) < uint16(cx) || uint16(cx+400) < uint16(x)
}

func (w *NativeWorld) collectActor(o *choreography.Object, template int) error {
	w.Delete(o, true)
	return w.replaceActor(o, template)
}
