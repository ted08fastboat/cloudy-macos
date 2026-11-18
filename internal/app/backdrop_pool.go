package app

import (
	"math"
	"math/rand/v2"

	"github.com/egoist/mygo/ui"
)

// The ripple scene is a pool on a summer's day, looked straight down
// into, drawn flat: water of one clear color, lighter where the sun
// lies on it, in two steps with a drawn edge; the net of light the sun
// casts through the surface, white lines that never hold still; a leaf
// or two afloat, and their shadows on the floor; glints. Drops fall on
// it with the beat (Drops: where on the picture, when, and how hard),
// each sending out a few thin white rings, round, seen from above, which
// bend the net as they pass.
//
// The water is soft enough for the canvas; the net, the rings, the
// glints and the leaves are lines and edges, painted over it at the
// window's own size (poolOverSource). Where the leaves are is the app's
// to say (leafState): they go where the water takes them.

// poolShared is what both of its shaders know: where a drop fell, how
// far its ring has gone, and the leaves (Leaves: where each is on the
// picture, as shares of it, how it is turned, and half its length, as a
// share of the height, or 0 for none).
const poolShared = `
var Drops [8]vec4
var Leaves [6]vec4

const life = 3.4

// reach is how far the ring of a drop has gone, age seconds after it
// fell, in heights of the picture: fast at first, then slowing.
func reach(age float) float {
	return 0.34 * (1.0 - exp(-0.7*age))
}

// dropAt is where a drop fell, across and down, in heights.
func dropAt(dr vec4, aspect float) vec2 {
	return vec2(dr.x*aspect, 0.08+0.84*dr.y)
}

// leaf is the leaf lf at p: how much of it covers p, soft over soft
// heights at its edge, and where across it p is, from one edge at -1 to
// the other at 1.
func leaf(p vec2, lf vec4, aspect float, soft float) vec2 {
	if lf.w <= 0.0 {
		return vec2(0)
	}
	rel := p - vec2(lf.x*aspect, lf.y)
	l := vec2(rel.x*cos(lf.z)+rel.y*sin(lf.z), rel.y*cos(lf.z)-rel.x*sin(lf.z)) / lf.w
	wide := 0.34 * (1.0 - l.x*l.x)
	return vec2(smoothstep(0.0, soft/lf.w, wide-abs(l.y)), l.y/0.34)
}
`

// poolSource paints the water: its color, the sunlit parts of it, and
// the shadows of the leaves.
const poolSource = sceneHead + sceneNoise + poolShared + `
func Fragment(dst vec4, pos vec2) vec4 {
	size := rectSize()
	uv := pos / size
	aspect := size.x / size.y
	p := vec2(uv.x*aspect, uv.y)
	t := Time
	// The rings bend what lies under them, and lighten the water they
	// have just crossed.
	bend := vec2(0)
	wash := 0.0
	for i := 0; i < 8; i++ {
		dr := Drops[i]
		age := t - dr.z
		if dr.w > 0.0 && age > 0.0 && age < life {
			off := p - dropAt(dr, aspect)
			d := length(off)
			u := d - reach(age)
			env := exp(-u*u/0.0012) * dr.w * (1.0 - age/life)
			bend += off / max(d, 0.0001) * (sin(u*110.0) * env * 0.012)
			wash += env * smoothstep(0.0, 0.3, age)
		}
	}
	// Where the sun lies on the water: more of it toward the corner it
	// shines from, in two steps, each with an edge; a little more of it
	// with the bass.
	v := 0.5 + 0.25*sin(p.x*3.1+t*0.07+1.3*sin(p.y*2.3-t*0.05)) + 0.25*sin(p.y*3.7-t*0.06+1.1*sin(p.x*2.9+t*0.04))
	v += 0.3*(0.5-0.5*(uv.x+uv.y)) + 0.05*Bass
	e := 2.2 / size.y
	c := Deep.rgb
	c = mix(c, A.rgb, smoothstep(-e, e, v-0.56))
	c = mix(c, B.rgb, smoothstep(-e, e, v-0.8))
	c = mix(c, B.rgb, min(0.3*wash, 0.4))
	// The shadows of the leaves, on the floor.
	for n := 0; n < 6; n++ {
		lf := leaf(p+bend-vec2(0.012, 0.018), Leaves[n], aspect, 5.0/size.y)
		c = mix(c, Deep.rgb*0.76, 0.5*lf.x)
	}
	c += grain(pos)
	return vec4(clamp(c, 0.0, 1.0), 1)
}
`

// poolOverSource paints over the water, at the window's own size, what
// has edges: the net of light the sun casts through the surface, bent
// by the rings; the rings of the drops, three to a drop, thinner as
// they widen; glints of the sun, four-pointed, coming and going; and
// the leaves. Opacity is the scene's.
const poolOverSource = `package main

var Time float
var Bass float
var C vec4
var Opacity float
` + poolShared + `
func hash(p vec2) float {
	return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453)
}

// spot is where in its cell the corner of the net of cell g is, moving.
func spot(g vec2, t float) vec2 {
	h := vec2(hash(g), hash(g+vec2(17.3, 5.1)))
	return 0.5 + 0.36*sin(t*0.45+6.2831853*h)
}

// net is how far x is from the nearest line of the net, in cells: the
// lines run midway between the spots of neighboring cells.
func net(x vec2, t float) float {
	n := floor(x)
	f := fract(x)
	mg := vec2(0)
	mr := vec2(0)
	md := 8.0
	for j := 0; j < 3; j++ {
		for i := 0; i < 3; i++ {
			g := vec2(float(i)-1.0, float(j)-1.0)
			r := g + spot(n+g, t) - f
			d := dot(r, r)
			if d < md {
				md = d
				mr = r
				mg = g
			}
		}
	}
	// The cells about the nearest spot's: its neighbors are among them,
	// the spots keeping well inside their cells.
	md = 8.0
	for j := 0; j < 3; j++ {
		for i := 0; i < 3; i++ {
			g := mg + vec2(float(i)-1.0, float(j)-1.0)
			r := g + spot(n+g, t) - f
			if dot(mr-r, mr-r) > 0.00001 {
				md = min(md, dot(0.5*(mr+r), normalize(r-mr)))
			}
		}
	}
	return md
}

func Fragment(dst vec4, pos vec2) vec4 {
	size := rectSize()
	uv := pos / size
	aspect := size.x / size.y
	p := vec2(uv.x*aspect, uv.y)
	t := Time
	// A line is as thick on a large window as on a small one looks.
	thick := max(1.0, size.y/820.0)
	col := vec3(0)
	al := 0.0
	// The rings, which bend the net under them.
	ring := 0.0
	bend := vec2(0)
	for i := 0; i < 8; i++ {
		dr := Drops[i]
		age := t - dr.z
		if dr.w > 0.0 && age > 0.0 && age < life {
			off := p - dropAt(dr, aspect)
			d := length(off)
			r := reach(age)
			u := d - r
			bend += off / max(d, 0.0001) * (sin(u*110.0) * exp(-u*u/0.0012) * dr.w * (1.0 - age/life) * 0.012)
			left := 1.0 - age/life
			fade := min(dr.w*1.2, 1.0) * pow(left, 1.5)
			w := thick * (1.9 - 1.1*age/life)
			for k := 0; k < 3; k++ {
				n := float(k)
				// The inner ones follow the first, fainter and finer.
				rr := r * (1.0 - 0.28*n)
				far := abs(d-rr) * size.y
				wk := w * (1.0 - 0.22*n)
				show := smoothstep(0.012, 0.03, rr) * (1.0 - 0.33*n)
				ring = max(ring, (1.0-smoothstep(wk-0.7, wk+0.7, far))*fade*show)
			}
		}
	}
	// The net of light, swaying: its lines waver, as water draws them,
	// and are thicker in places.
	sway := vec2(sin(p.y*6.3+t*0.31)+0.45*sin(p.y*15.0+p.x*4.0-t*0.42), cos(p.x*5.7-t*0.27)+0.45*cos(p.x*14.0+p.y*5.0+t*0.37))
	q := (p + bend + 0.04*sway) * 3.6
	far := net(q, t) / 3.6 * size.y
	w := 0.0022 * size.y * (0.75 + 0.6*smoothstep(-0.6, 0.8, sin(q.x*1.3+q.y*0.9+t*0.2)))
	web := (1.0 - smoothstep(w-0.7, w+0.7, far)) * (0.36 + 0.3*Bass)
	col = C.rgb * web
	al = web
	col = col*(1.0-ring) + C.rgb*ring
	al = al*(1.0-ring) + ring
	// The glints, in a grid, one in some of its cells, each in its own
	// time.
	cell := size.y / 3.3
	sp := pos / cell
	id := floor(sp)
	h := hash(id + vec2(3.7, 9.1))
	if h < 0.62 {
		h2 := hash(id + vec2(8.3, 1.9))
		at := vec2(hash(id+vec2(1.3, 2.9)), hash(id+vec2(6.1, 4.3)))*0.6 + 0.2
		tw := pow(max(sin(t*(0.5+0.7*h2)+h*60.0), 0.0), 5.0)
		rad := (0.015 + 0.016*h2) * size.y * (0.4 + 0.6*tw) * (0.9 + 0.3*Bass)
		rel := abs((fract(sp)-at)*cell) / rad
		star := pow(rel.x, 0.55) + pow(rel.y, 0.55)
		m := (1.0 - smoothstep(0.9, 1.0, star)) * smoothstep(0.0, 0.25, tw)
		col = col*(1.0-m) + vec3(1)*m
		al = al*(1.0-m) + m
	}
	// The leaves: two greens either side of the rib.
	for k := 0; k < 6; k++ {
		lf := leaf(p, Leaves[k], aspect, 1.4/size.y)
		if lf.x > 0.0 {
			green := mix(vec3(0.62, 0.82, 0.36), vec3(0.8, 0.92, 0.46), step(0.0, lf.y))
			green = mix(vec3(0.5, 0.72, 0.3), green, smoothstep(0.03, 0.09, abs(lf.y)))
			col = col*(1.0-lf.x) + green*lf.x
			al = al*(1.0-lf.x) + lf.x
		}
	}
	return vec4(col, al) * Opacity
}
`

// poolColors returns the water of the ripple scene for a cover of hue h
// and saturation s, as it is: its body, its two sunlit steps and its
// lights. Light and clear whatever the cover, never deep: a red cover
// gives a soft coral, not a pool of red. Each step is as bright as the
// same step of any other hue, dark enough for white to read over.
func poolColors(h, s float64) (deep, a, b, c ui.Color) {
	// The warm hues take less color than the cool, being the ones that
	// alarm.
	sat := 0.8 - 0.36*max(0, math.Cos(h*math.Pi/180))
	if s <= 0.12 {
		sat = 0
	}
	// Yellow, as dark as white needs it, is mud: it leans to green.
	h = turnHue(h, 38*math.Exp(-(h-55)*(h-55)/(28*28)))
	tone := func(turn, k, lum float64) ui.Color {
		col := hsvColor(turnHue(h, turn), sat*k, 1)
		r, g, bl := float64(col.R)/255, float64(col.G)/255, float64(col.B)/255
		scale := lum / (0.2126*r + 0.7152*g + 0.0722*bl)
		ch := func(v float64) uint8 { return uint8(min(1, v*scale)*255 + 0.5) }
		return ui.RGB(ch(r), ch(g), ch(bl))
	}
	return tone(0, 1, 0.41), tone(-3, 0.92, 0.49), tone(-6, 0.8, 0.58), hsvColor(h, 0.06, 1)
}

// poolLeaves is how many leaves the pool may hold at once.
const poolLeaves = 6

// A poolLeaf is a leaf afloat on the pool: where it is, across and
// down, in heights of the picture, and where it is going; how it is
// turned, and turning; the pace it keeps, left alone; half its length;
// and, once it has left the picture, how long till another comes.
type poolLeaf struct {
	x, y, vx, vy float32
	turn, spin   float32
	pace, half   float32
	alive        bool
	wait         float32
}

// leafState is the leaves of the pool, carried by the water: each goes
// its own way, wandering, and a ring passing under one pushes it along
// and turns it. They leave by any edge, and others come in by any: from
// two to six of them in the picture.
type leafState struct {
	leaves  [poolLeaves]poolLeaf
	started bool
}

// leafEdge is how far past an edge of the picture a leaf is gone, or
// comes from, in heights.
const leafEdge = 0.09

// step moves the leaves on by dt seconds, at drift seconds, on a
// picture aspect times as wide as high, with the drops fallen on it.
func (ls *leafState) step(dt, aspect float32, drift float64, drops *[sceneDrops]sceneDrop) {
	if aspect <= 0 {
		return
	}
	if !ls.started {
		ls.started = true
		for i := range 4 {
			l := &ls.leaves[i]
			ls.enter(l, aspect)
			l.x, l.y = aspect*(0.12+0.76*rand.Float32()), 0.12+0.76*rand.Float32()
		}
	}
	dt = min(dt, 0.1)
	if dt <= 0 {
		return
	}
	afloat := 0
	for i := range ls.leaves {
		l := &ls.leaves[i]
		if !l.alive {
			continue
		}
		// It wanders.
		a := (rand.Float32() - 0.5) * 1.2 * dt
		sin, cos := float32(math.Sin(float64(a))), float32(math.Cos(float64(a)))
		l.vx, l.vy = l.vx*cos-l.vy*sin, l.vx*sin+l.vy*cos
		// The rings push it outward as they pass, and turn it.
		for _, d := range drops {
			age := float32(drift) - d.at
			if d.hard <= 0 || age <= 0 || age >= 3.4 {
				continue
			}
			ox, oy := l.x-d.x*aspect, l.y-(0.08+0.84*d.y)
			far := float32(math.Hypot(float64(ox), float64(oy)))
			u := far - 0.34*(1-float32(math.Exp(float64(-0.7*age))))
			push := float32(math.Exp(float64(-u*u/0.0012))) * d.hard * (1 - age/3.4)
			if far > 0.001 && push > 0.01 {
				l.vx += ox / far * 0.55 * push * dt
				l.vy += oy / far * 0.55 * push * dt
				l.turn += 1.4 * push * dt
			}
		}
		// And it settles back to its own pace, going the way it now goes.
		if sp := float32(math.Hypot(float64(l.vx), float64(l.vy))); sp > 0.0001 {
			k := 1 + (l.pace-sp)/sp*min(1, 0.7*dt)
			l.vx, l.vy = l.vx*k, l.vy*k
		}
		l.x, l.y = l.x+l.vx*dt, l.y+l.vy*dt
		l.turn += l.spin * dt
		if l.x < -leafEdge || l.x > aspect+leafEdge || l.y < -leafEdge || l.y > 1+leafEdge {
			l.alive, l.wait = false, 1+5*rand.Float32()
			continue
		}
		afloat++
	}
	for i := range ls.leaves {
		l := &ls.leaves[i]
		if l.alive {
			continue
		}
		l.wait -= dt
		// Never fewer than three afloat, of which one may be at an edge.
		if l.wait <= 0 || afloat < 3 {
			ls.enter(l, aspect)
			afloat++
		}
	}
}

// enter brings l in at some point of some edge, headed inward, some way
// or other.
func (ls *leafState) enter(l *poolLeaf, aspect float32) {
	at := rand.Float32() * 2 * (aspect + 1)
	var nx, ny float32
	switch {
	case at < aspect:
		l.x, l.y, ny = at, -leafEdge*0.8, 1
	case at < aspect+1:
		l.x, l.y, nx = aspect+leafEdge*0.8, at-aspect, -1
	case at < 2*aspect+1:
		l.x, l.y, ny = at-aspect-1, 1+leafEdge*0.8, -1
	default:
		l.x, l.y, nx = -leafEdge*0.8, at-2*aspect-1, 1
	}
	a := math.Atan2(float64(ny), float64(nx)) + float64(rand.Float32()-0.5)*1.9
	l.pace = 0.014 + 0.02*rand.Float32()
	l.vx, l.vy = l.pace*float32(math.Cos(a)), l.pace*float32(math.Sin(a))
	l.turn, l.spin = rand.Float32()*6.28, (rand.Float32()-0.5)*0.3
	l.half = 0.042 + 0.022*rand.Float32()
	l.alive = true
}

// uniform returns the leaves as the pool's shaders take them.
func (ls *leafState) uniform(aspect float32) []float32 {
	out := make([]float32, 4*poolLeaves)
	if aspect <= 0 {
		return out
	}
	for i, l := range ls.leaves {
		if l.alive {
			out[4*i], out[4*i+1], out[4*i+2], out[4*i+3] = l.x/aspect, l.y, l.turn, l.half
		}
	}
	return out
}

// demoLeaves are leaves for a picture of the pool with none of its own:
// its card, a test.
func demoLeaves(drift float32) []float32 {
	spots := [...][4]float32{{0.2, 0.26, 0.6, 0.048}, {0.83, 0.6, 2.9, 0.057}, {0.46, 0.86, 5.2, 0.066}, {0.66, 0.18, 4.1, 0.05}}
	out := make([]float32, 4*poolLeaves)
	for i, s := range spots {
		out[4*i], out[4*i+1], out[4*i+2], out[4*i+3] = s[0], s[1], s[2]+0.3*float32(math.Sin(float64(drift)*0.2+float64(i))), s[3]
	}
	return out
}
