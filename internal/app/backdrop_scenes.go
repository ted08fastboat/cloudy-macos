package app

import (
	"math/rand/v2"
	"sync"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"
)

// The living backdrop of Now Playing (settings.Fog) has more scenes than
// the fog (settings.FogStyle): lights out of focus, an aurora over a
// lake, a night sky, the deep sea, a pool in the sun, a lake under the
// moon. They keep to what makes the fog good: one thing to
// look at, in the cover's own hue, soft enough to paint at half the
// window's size, billowing rather than sliding, and the bass making it
// thicker and brighter, not larger.
//
// Each is one shader given the same things, so that they change places
// freely: Time, the drift in seconds, Bass, the low end from 0 to 1,
// Deep, the shade behind it all, and A, B and C, its lights: the cover's
// hue, a lighter one beside it, and its highlight, nearly white.
//
// Stars are the one thing too fine for half the size: they are painted
// over the canvas, at the window's own, by a shader of their own
// (starsOverSource), for the scenes that have them.

// sceneNoise is what the scenes share: noise, and how light adds.
const sceneNoise = `
func hash(p vec2) float {
	return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453)
}

func hash1(n float) float {
	return hash(vec2(n*1.37, 17.0-n*0.61))
}

func noise(p vec2) float {
	i := floor(p)
	f := fract(p)
	u := f * f * (3.0 - 2.0*f)
	a := hash(i)
	b := hash(i + vec2(1, 0))
	c := hash(i + vec2(0, 1))
	d := hash(i + vec2(1, 1))
	return mix(mix(a, b, u.x), mix(c, d, u.x), u.y)
}

// fbm is the fog's noise, of five octaves.
func fbm(p vec2) float {
	v := 0.0
	amp := 0.5
	for i := 0; i < 5; i++ {
		v += amp * noise(p)
		p = p*2.03 + vec2(1.7, 9.2)
		amp *= 0.5
	}
	return v
}

// screen adds light to c as light adds on film: never past white, and
// gently where it is already bright.
func screen(c vec3, light vec3) vec3 {
	return 1.0 - (1.0-c)*(1.0-min(light, vec3(1)))
}

// grain is a grain too fine to see, which keeps the dark gradients from
// showing in bands.
func grain(pos vec2) float {
	return (hash(pos*1.31+vec2(0.7, 3.3)) - 0.5) * 0.008
}
`

const sceneHead = `package main

var Time float
var Bass float
var Deep vec4
var A vec4
var B vec4
var C vec4
`

// bokehSource paints lights out of focus in hazy air: a lamp out of the
// picture, up and to the left, its glow breathing with the bass; a thin
// haze of the fog's own kind, lit by it; and in the haze the lights, as
// a lens draws those far behind what it is focused on: a few very large
// and very faint, close to the lens, a far layer of small soft ones and
// a near one of discs with the bright rim and the shading of a real
// aperture, brighter toward the lamp, drifting up and swaying, each
// breathing in its own time.
const bokehSource = sceneHead + sceneNoise + `
func Fragment(dst vec4, pos vec2) vec4 {
	size := rectSize()
	uv := pos / size
	aspect := size.x / size.y
	p := vec2(uv.x*aspect, uv.y)
	t := Time
	// The lamp, and how near it each place is.
	lamp := vec2(0.14*aspect, 0.04)
	glow := exp(-dot(p-lamp, p-lamp) * 1.5)
	c := Deep.rgb * (1.15 - 0.45*uv.y)
	c += A.rgb * glow * (0.11 + 0.08*Bass)
	c += C.rgb * glow * glow * (0.035 + 0.03*Bass)
	// The haze, billowing as the fog does, brightest in the lamp's light.
	fp := p * 1.7
	rise := vec2(t*0.012, t*0.03)
	q := fbm(fp + rise + vec2(0, t*0.02))
	haze := smoothstep(0.32, 0.8, fbm(fp+rise+vec2(5.2, 1.3)+1.1*q))
	c += mix(A.rgb, B.rgb, 0.4) * haze * (0.05 + 0.11*glow) * (0.85 + 0.4*Bass)
	for i := 0; i < 28; i++ {
		n := float(i)
		h1 := hash1(n*4.0 + 0.3)
		h2 := hash1(n*4.0 + 1.1)
		h3 := hash1(n*4.0 + 2.7)
		h4 := hash1(n*4.0 + 3.9)
		// Three close to the lens, fifteen far, ten between.
		huge := 1.0 - step(2.5, n)
		near := step(17.5, n)
		rad := mix(0.016+0.036*h3, 0.05+0.11*h3*h3, near)
		rad = mix(rad, 0.26+0.14*h3, huge)
		soft := mix(mix(0.62, 0.16, near), 0.5, huge)
		speed := mix(0.4, 1.0, near) * (0.5 + 0.5*h4)
		// Spread evenly across, each then swaying about its place.
		at := vec2(fract(n*0.618034+0.13*h1)*(aspect+0.4)-0.2, fract(n*0.754878+0.2*h2-t*0.008*speed)*1.5-0.25)
		at += vec2(0.04*sin(t*(0.1+0.12*h4)+h2*6.28), 0.025*sin(t*(0.15+0.1*h1)+h4*6.28))
		rel := (p - at) / rad
		d := length(rel)
		if d < 1.0 {
			disc := 1.0 - smoothstep(1.0-soft, 1.0, d)
			rim := smoothstep(1.0-soft*2.4, 1.0-soft*0.7, d) * disc * near
			// Brighter on the lamp's side, as a lens shades its discs.
			shade := 0.86 - 0.14*rel.y - 0.1*rel.x
			col := mix(A.rgb, B.rgb, smoothstep(0.25, 0.75, h4))
			col = mix(col, C.rgb, smoothstep(0.7, 0.95, h1)*0.7)
			breath := 0.72 + 0.28*sin(t*(0.25+0.4*h2)+h3*30.0)
			lit := 0.55 + 1.1*exp(-dot(at-lamp, at-lamp)*1.2)
			amt := mix(0.06+0.06*h3, 0.1+0.1*h3, near)
			amt = mix(amt, 0.035, huge) * breath * lit * (0.85 + 0.45*Bass)
			light := col*(disc*shade*amt) + mix(col, vec3(1), 0.45)*(rim*amt*0.85)
			c = screen(c, light)
		}
	}
	// Darker away from the lamp, and under the controls.
	g := length((uv - vec2(0.42, 0.36)) * vec2(1.0, 1.2))
	c *= 1.06 - 0.5*smoothstep(0.2, 0.95, g)
	c += grain(pos)
	return vec4(clamp(c, 0.0, 1.0), 1)
}
`

// auroraSource paints an aurora over a lake: one curtain hanging in
// folds, brighter where it turns toward us, with a bright hem and rays
// of uneven height above it, the cover's color at its hem and a lighter
// one at its top; a fainter band far behind it; light surging along
// them, more of it with the bass, and stretches of them fading away and
// coming back; hills against the sky's foot; and the lake under them,
// which holds it all upside down, drawn out and rippling.
const auroraSource = sceneHead + sceneNoise + `
const horizon = 0.8

// curtain is the light of a curtain at (x, y), y down from the top of
// the sky: the one near us for n of 0, the far one for 1.
func curtain(x float, y float, t float, n float) vec3 {
	xs := x*(1.0+0.6*n) + n*3.7
	// The folds it hangs in: where it turns toward us there is more of
	// it to look through.
	w1 := xs*1.7 + t*0.15 + n*2.1
	w2 := xs*3.9 - t*0.23 + n*4.7
	u := xs + 0.2*sin(w1) + 0.08*sin(w2)
	press := 1.0 / (0.35 + abs(1.0+0.34*cos(w1)+0.312*cos(w2)))
	hem := 0.52 - 0.25*n + 0.085*sin(xs*0.8+t*0.1+n) + 0.05*sin(u*2.3-t*0.18)
	d := hem - y
	if d < -0.08 {
		return vec3(0)
	}
	fine := 0.6*noise(vec2(u*13.0, t*0.3+n*7.0)) + 0.4*noise(vec2(u*31.0+4.0, t*0.45))
	broad := noise(vec2(u*4.0+n*11.0, t*0.16))
	// Light surges along it, and with the bass; stretches of it fade
	// away and come back.
	surge := smoothstep(0.3, 0.8, noise(vec2(u*0.8-t*0.3, t*0.06+n*3.0)))
	beat := smoothstep(0.35, 0.75, noise(vec2(u*1.6-t*1.1, n*5.0)))
	life := 0.3 + 0.7*smoothstep(0.28, 0.62, noise(vec2(xs*0.45+3.0+n*9.0, t*0.045)))
	power := life * (0.55 + 0.7*surge + 0.7*Bass*beat)
	// Some rays reach far higher than others.
	reach := (0.3 + 1.3*broad*broad) * (0.7 + 0.5*surge + 0.4*Bass)
	up := max(d, 0.0)
	body := exp(-up * (2.7 + 2.6*n) / reach)
	hot := exp(-up * 30.0)
	ray := 0.3 + 0.7*smoothstep(0.25, 0.75, 0.5*broad+0.5*fine)
	v := smoothstep(-0.08, 0.012, d) * (body*ray + 0.3*hot*(0.55+0.45*fine)) * press * power * (0.66 - 0.28*n)
	col := mix(A.rgb, B.rgb, smoothstep(0.02*reach, 0.5*reach, up))
	col = mix(col, C.rgb, 0.35*hot)
	return col * v
}

// hill is how high the hills stand at x, as a height of the picture.
func hill(x float) float {
	return horizon - 0.003 - 0.02*noise(vec2(x*1.4+3.0, 1.0)) - 0.007*noise(vec2(x*5.0, 7.0))
}

// sky is the sky at (x, y), with its hills.
func sky(x float, y float, t float) vec3 {
	// The night, lighter toward the horizon, where the aurora lights it.
	c := Deep.rgb * (0.6 + 0.75*y*y)
	c += A.rgb * 0.07 * smoothstep(0.4, horizon, y) * (0.7 + 0.6*Bass)
	c = screen(c, curtain(x, y, t, 1.0))
	c = screen(c, curtain(x, y, t, 0.0))
	return mix(c, Deep.rgb*0.4, smoothstep(0.0, 0.005, y-hill(x)))
}

func Fragment(dst vec4, pos vec2) vec4 {
	size := rectSize()
	uv := pos / size
	x := uv.x * size.x / size.y
	t := Time
	c := vec3(0)
	if uv.y < horizon {
		c = sky(x, uv.y, t)
	} else {
		// The lake: the sky upside down, drawn out toward us, broken by
		// its ripples, more the nearer they are.
		depth := (uv.y - horizon) / (1.0 - horizon)
		wob := noise(vec2(x*16.0, uv.y*90.0-t*0.5)) - 0.5
		c = sky(x+0.01*wob*depth, horizon-(uv.y-horizon)*2.3-0.035*wob*(0.3+depth), t)
		c = c*(0.85-0.35*depth) + Deep.rgb*0.06
		// Light caught on the ripples.
		c += A.rgb * 0.05 * smoothstep(0.7, 0.9, noise(vec2(x*9.0, uv.y*160.0+t*0.4))) * (1.0 - depth)
	}
	c += grain(pos)
	return vec4(clamp(c, 0.0, 1.0), 1)
}
`

// starsSource paints the night sky behind the stars: its own depth,
// lighter low down, and a thin gas of the cover's color lying across
// it, the fog itself, thinner and darker, billowing slowly, thicker
// with the bass. The stars are painted over it.
const starsSource = sceneHead + sceneNoise + `
func Fragment(dst vec4, pos vec2) vec4 {
	size := rectSize()
	uv := pos / size
	p := pos / size.y * 2.0
	t := Time
	rise := vec2(t*0.018, -t*0.012)
	q := vec2(fbm(p+rise+vec2(0, t*0.01)), fbm(p+rise+vec2(5.2, 1.3)-vec2(t*0.012, 0)))
	f := fbm(p + rise + 0.9*q)
	// Broad banks of it, most of it in a band across the sky.
	bank := fbm(p*0.45 + rise*0.6 + vec2(3.7, 0))
	across := (uv.y - 0.46 + (uv.x-0.5)*0.36) * 2.0
	m := 0.3 + 0.7*exp(-across*across)
	cloud := smoothstep(0.34-0.05*Bass, 0.86, f) * (0.3 + 0.8*smoothstep(0.3, 0.65, bank)) * m
	c := Deep.rgb * (0.8 + 0.45*uv.y)
	c = mix(c, A.rgb, min(cloud*(0.5+0.3*Bass), 1.0))
	c += B.rgb * cloud * cloud * 0.12
	// Darker at the corners.
	g := length((uv - vec2(0.5, 0.45)) * vec2(1.0, 1.2))
	c *= 1.06 - 0.4*smoothstep(0.25, 0.95, g)
	c += grain(pos)
	return vec4(clamp(c, 0.0, 1.0), 1)
}
`

// deepSource paints the deep sea, looked up through from below: water
// of the cover's color where the light comes in, going black with depth;
// shafts of light slanting down from far above the picture, soft and uneven,
// swaying, wider and brighter with the bass; a shimmer under the
// surface; and what drifts in the water, sinking slowly, far specks and
// near ones out of focus, lit where a shaft passes through them. All of
// it sways, a little, with the swell.
const deepSource = sceneHead + sceneNoise + `
// snow is the light at pos of what drifts in a grid of cell, a share
// dense of the cells holding a speck rad across, each wandering about
// its place.
func snow(pos vec2, cell float, seed float, dense float, rad float, t float) float {
	sp := pos / cell
	id := floor(sp) + vec2(seed, seed*1.7)
	h := hash(id)
	if h > dense {
		return 0.0
	}
	at := vec2(hash(id+vec2(1.3, 2.9)), hash(id+vec2(8.1, 4.3)))*0.6 + 0.2
	at += 0.1 * vec2(sin(t*(0.3+h)+h*40.0), cos(t*(0.25+0.5*h)+h*17.0))
	d := length((fract(sp)-at)*cell) / max(rad, 0.7)
	return exp(-d*d) * (0.4 + 0.6*hash(id+vec2(5.7, 0.9)))
}

func Fragment(dst vec4, pos vec2) vec4 {
	size := rectSize()
	uv := pos / size
	aspect := size.x / size.y
	t := Time
	// The swell.
	q := uv + vec2(0.012*sin(t*0.21+uv.y*3.0), 0.008*sin(t*0.17+uv.x*2.6))
	p := vec2(q.x*aspect, q.y)
	// The water: the cover's color under the surface, black far down.
	c := mix(A.rgb*0.4, Deep.rgb*0.7, smoothstep(0.0, 0.6, q.y))
	c = mix(c, Deep.rgb*0.2, smoothstep(0.45, 1.05, q.y))
	// The shafts come from where the light comes in, far above the
	// picture, so nearly side by side: uneven across, in broad ones and fine, and thinning along
	// their length.
	rel := p - vec2(0.3*aspect, -1.2)
	dist := length(rel)
	ang := rel.x / rel.y
	n1 := noise(vec2(ang*13.0+t*0.05, t*0.07))
	n2 := noise(vec2(ang*34.0-t*0.08, t*0.11+4.0))
	n3 := noise(vec2(ang*5.5+9.0, t*0.04))
	shaft := smoothstep(0.42-0.08*Bass, 0.85, 0.5*n1+0.25*n2+0.35*n3)
	reach := exp(-q.y * (2.1 - 0.5*Bass))
	wisp := 0.65 + 0.35*fbm(vec2(ang*10.0, dist*2.0-t*0.05))
	light := shaft * reach * wisp
	c += mix(B.rgb, C.rgb, 0.35) * light * (0.3 + 0.3*Bass)
	c += A.rgb * 0.16 * exp(-q.y*3.2) * (0.8 + 0.4*Bass)
	// The shimmer just under the surface.
	ca := noise(p*vec2(9.0, 5.0)+vec2(t*0.3, t*0.12)) * noise(p*vec2(7.0, 6.0)-vec2(t*0.22, -t*0.1))
	c += C.rgb * smoothstep(0.3, 0.6, ca) * 0.1 * (1.0 - smoothstep(0.0, 0.35, q.y))
	// What drifts in the water, sinking: the far, the nearer, and a few
	// close to the eye, large and faint; brightest in a shaft.
	drift := size.y * 0.02 * sin(t*0.13)
	at := q * size
	s1 := snow(at+vec2(drift, -t*size.y*0.006), size.y/34, 3.0, 0.5, size.y*0.0016, t)
	s2 := snow(at+vec2(drift*1.6, -t*size.y*0.011), size.y/20, 7.0, 0.4, size.y*0.0028, t)
	s3 := snow(at+vec2(drift*2.4, -t*size.y*0.018), size.y/9, 11.0, 0.35, size.y*0.009, t)
	lit := 0.25 + 1.3*light + 0.5*reach
	c += C.rgb * (s1*0.22 + s2*0.36 + s3*0.1) * lit
	// Darker at the corners, and under the controls.
	g := length((uv - vec2(0.45, 0.3)) * vec2(1.0, 1.1))
	c *= 1.05 - 0.4*smoothstep(0.3, 1.0, g)
	c += grain(pos)
	return vec4(clamp(c, 0.0, 1.0), 1)
}
`

// starsOverSource paints stars over a scene, at the window's own size,
// so that each is a point: few, of three sizes, blue-white to amber,
// the brightest with a halo and the spikes a telescope gives them,
// turning slowly, twinkling; as many as Dense says, none below Horizon,
// a height of the picture, nor over the moon, if the scene has one
// (Moon: where on the picture, as shares of it, and how large, as a
// share of its height); and, with Shoot, a shooting star now and then.
// Opacity is the scene's.
const starsOverSource = `package main

var Time float
var Bass float
var C vec4
var Opacity float
var Dense float
var Horizon float
var Shoot float
var Moon vec4

func hash(p vec2) float {
	return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453)
}

func hash1(n float) float {
	return hash(vec2(n*1.37, 17.0-n*0.61))
}

// starColor is the color of a star as warm as warm: blue-white to amber.
func starColor(warm float) vec3 {
	return mix(vec3(0.76, 0.85, 1.0), vec3(1.0, 0.9, 0.76), smoothstep(0.35, 0.95, warm))
}

// layer is the light at pos of the stars of a grid of cell DIPs, one in
// a cell at most, a share dense of the cells, rad DIPs across; and how
// warm the star is.
func layer(pos vec2, cell float, seed float, dense float, rad float, t float) vec2 {
	sp := pos / cell
	id := floor(sp) + vec2(seed, seed*1.7)
	h := hash(id)
	if h > dense {
		return vec2(0)
	}
	at := vec2(hash(id+vec2(1.3, 2.9)), hash(id+vec2(8.1, 4.3)))*0.7 + 0.15
	bright := hash(id + vec2(5.7, 0.9))
	d := length((fract(sp)-at)*cell) / (rad * (0.75 + 0.5*bright))
	glint := 0.72 + 0.28*sin(t*(0.5+1.6*bright)+h*90.0)
	return vec2(exp(-d*d*1.4)*(0.25+0.75*bright*bright)*glint, hash(id+vec2(2.2, 6.1)))
}

// flare is a bright star: its core, its halo, and for the brightest
// their spikes.
func flare(pos vec2, cell float, seed float, dense float, t float) vec2 {
	sp := pos / cell
	id := floor(sp) + vec2(seed, seed*1.7)
	h := hash(id)
	if h > dense {
		return vec2(0)
	}
	at := vec2(hash(id+vec2(1.3, 2.9)), hash(id+vec2(8.1, 4.3)))*0.6 + 0.2
	bright := hash(id + vec2(5.7, 0.9))
	rel := (fract(sp) - at) * cell
	d := length(rel)
	glint := 0.8 + 0.2*sin(t*(0.4+1.1*bright)+h*90.0)
	v := exp(-d*d*0.55) + (0.1+0.08*Bass)*exp(-d*0.22)
	spike := exp(-abs(rel.x)*0.16)*exp(-rel.y*rel.y*1.1) + exp(-abs(rel.y)*0.16)*exp(-rel.x*rel.x*1.1)
	v += 0.28 * spike * smoothstep(0.45, 0.9, bright) * glint
	return vec2(v*(0.6+0.4*bright)*glint, hash(id+vec2(2.2, 6.1)))
}

func Fragment(dst vec4, pos vec2) vec4 {
	size := rectSize()
	uv := pos / size
	t := Time
	// The sky turns, a little; the near stars faster.
	turn := vec2(t*0.35, t*0.09)
	a := layer(pos+turn, 44.0, 1.0, 0.3*Dense, 0.8, t)
	b := layer(pos+turn*1.3, 96.0, 5.0, 0.3*Dense, 1.1, t)
	f := flare(pos+turn*1.6, 230.0, 9.0, 0.42*Dense, t)
	light := starColor(a.y)*a.x*0.75 + starColor(b.y)*b.x + mix(starColor(f.y), C.rgb, 0.15)*f.x
	// Fewer low in the sky, and none below the horizon.
	light *= (1.0 - smoothstep(Horizon-min(0.25, Horizon*0.4), Horizon-0.02, uv.y)) * (1.0 - 0.3*uv.y)
	if Moon.z > 0.0 {
		// None over the moon, and few in its light.
		light *= smoothstep(Moon.z, Moon.z*5.0, length((uv-Moon.xy)*vec2(size.x/size.y, 1)))
	}
	if Shoot > 0.5 {
		// A shooting star, every so often.
		every := floor(t / 19.0)
		ph := t - every*19.0
		if ph < 1.0 {
			side := 1.0
			if hash1(every+11.0) > 0.5 {
				side = -1.0
			}
			dir := normalize(vec2(side*(0.8+0.4*hash1(every+7.0)), 0.3+0.3*hash1(every+9.0)))
			from := vec2((0.5-side*(0.1+0.25*hash1(every+0.5)))*size.x, (0.06+0.3*hash1(every+3.5))*size.y)
			rel := pos - (from + dir*ph*0.5*size.y)
			behind := -dot(rel, dir)
			off := rel.x*dir.y - rel.y*dir.x
			trail := 0.0
			if behind > 0.0 {
				trail = exp(-behind/(0.09*size.y)) * exp(-off*off*0.9/(1.0+behind*0.03))
			}
			show := smoothstep(0.0, 0.12, ph) * (1.0 - smoothstep(0.55, 1.0, ph))
			light += vec3(0.88, 0.94, 1.0) * (0.6*trail + exp(-dot(rel, rel)*0.35)) * show
		}
	}
	light = min(light, vec3(1))
	return vec4(light, max(light.r, max(light.g, light.b))) * Opacity
}
`

// backdropScene is a scene of the living backdrop.
type backdropScene struct {
	id, label string
	shader    func() *shader.Shader
	// colors returns the shade behind the scene and its three lights for
	// a cover of hue h and saturation s.
	colors func(h, s float64) (deep, a, b, c ui.Color)
	// stars is how many stars are painted over it, 1 a sky full, 0
	// none; horizon the height of the picture they stop at; and shoot
	// whether one shoots now and then.
	stars, horizon float32
	shoot          bool
	// drops says its water takes the drops the beat lets fall.
	drops bool
	// moon is where its moon is, if it has one, as the stars' shader
	// takes it.
	moon [4]float32
	// over paints over it, at the window's own size, what of it has
	// edges, given what the scene is and Opacity.
	over func() *shader.Shader
	// own says its colors are for the cover as it is, gray or not,
	// rather than for one made rich.
	own bool
}

var starsOverShader = sync.OnceValue(func() *shader.Shader { return shader.MustCompile(starsOverSource) })

// backdropScenes are the scenes, the fog first: the one a listener has
// who chose none.
var backdropScenes = []backdropScene{
	{id: "", label: "烟雾"},
	{id: "bokeh", label: "光斑", shader: sync.OnceValue(func() *shader.Shader { return shader.MustCompile(bokehSource) }),
		colors: func(h, s float64) (deep, a, b, c ui.Color) {
			return hsvColor(h, s, 0.19), hsvColor(h, s*0.85, 0.95), hsvColor(turnHue(h, 14), s*0.6, 0.95), hsvColor(turnHue(h, -10), s*0.25, 1)
		}},
	{id: "aurora", label: "极光", shader: sync.OnceValue(func() *shader.Shader { return shader.MustCompile(auroraSource) }),
		colors: func(h, s float64) (deep, a, b, c ui.Color) {
			return hsvColor(h, s*0.9, 0.12), hsvColor(h, s, 0.95), hsvColor(turnHue(h, 28), s*0.6, 0.85), hsvColor(h, s*0.25, 1)
		}, stars: 0.75, horizon: 0.8},
	{id: "stars", label: "星空", shader: sync.OnceValue(func() *shader.Shader { return shader.MustCompile(starsSource) }),
		colors: func(h, s float64) (deep, a, b, c ui.Color) {
			return hsvColor(h, s*0.85, 0.09), hsvColor(h, s*0.75, 0.46), hsvColor(h, s*0.45, 0.8), hsvColor(h, s*0.4, 1)
		}, stars: 1, horizon: 1.3, shoot: true},
	{id: "deep", label: "深海", shader: sync.OnceValue(func() *shader.Shader { return shader.MustCompile(deepSource) }),
		colors: func(h, s float64) (deep, a, b, c ui.Color) {
			return hsvColor(h, s, 0.16), hsvColor(h, s, 0.75), hsvColor(turnHue(h, -12), s*0.55, 0.95), hsvColor(h, s*0.22, 1)
		}},
	{id: "ripple", label: "波纹", shader: sync.OnceValue(func() *shader.Shader { return shader.MustCompile(poolSource) }),
		over:   sync.OnceValue(func() *shader.Shader { return shader.MustCompile(poolOverSource) }),
		colors: poolColors, own: true, drops: true},
	{id: "moon", label: "月夜", shader: sync.OnceValue(func() *shader.Shader { return shader.MustCompile(moonSource) }),
		colors: func(h, s float64) (deep, a, b, c ui.Color) {
			return hsvColor(h, s*0.9, 0.11), hsvColor(h, s, 0.8), hsvColor(turnHue(h, 18), s*0.6, 0.9), hsvColor(h, s*0.2, 1)
		}, stars: 1, horizon: 0.27, drops: true, moon: [4]float32{0.6, 0.215, 0.028}},
}

// sceneTints are the colors the scenes paint in while they keep to
// their own (not in settings.SceneCover), as if they were a cover's: each
// what its scene is in the world. The fog has none: it is the cover's.
var sceneTints = map[string]ui.Color{
	"bokeh":  ui.RGB(240, 150, 60), // lamps at dusk
	"aurora": ui.RGB(50, 210, 140), // the green of an aurora
	"stars":  ui.RGB(96, 92, 226),  // a night sky's violet blue
	"deep":   ui.RGB(24, 120, 210), // sea water
	"ripple": ui.RGB(40, 172, 199), // a pool
	"moon":   ui.RGB(64, 112, 230), // moonlight
}

// tintOf returns the color the scene of id paints in for a cover of
// tint: its own, or if it is among SceneCover, or has none, the
// cover's.
func (s *settings) tintOf(id string, tint ui.Color) ui.Color {
	if own, ok := sceneTints[id]; ok && !s.SceneCover[id] {
		return own
	}
	return tint
}

// backdropSceneOf returns the scene of id, the fog for one unknown.
func backdropSceneOf(id string) *backdropScene {
	for i := range backdropScenes {
		if backdropScenes[i].id == id {
			return &backdropScenes[i]
		}
	}
	return &backdropScenes[0]
}

// turnHue turns hue h by d degrees.
func turnHue(h, d float64) float64 {
	h += d
	for h < 0 {
		h += 360
	}
	for h >= 360 {
		h -= 360
	}
	return h
}

// sceneColors returns the colors of sc for a cover of tint: the cover's
// hue, rich, at the same depths whatever the cover, so that white reads
// over it. A gray cover stays gray, its lights dimmer, being white.
func sceneColors(sc *backdropScene, tint ui.Color) (deep, a, b, c ui.Color) {
	h, s, _ := hsv(float64(tint.R)/255, float64(tint.G)/255, float64(tint.B)/255)
	if sc.own {
		return sc.colors(h, s)
	}
	if s > 0.12 {
		s = min(max(s*1.3, 0.6), 0.85)
	}
	deep, a, b, c = sc.colors(h, s)
	if s <= 0.12 {
		dark := ui.RGB(0, 0, 0)
		a, b, c = a.Mix(dark, 0.4), b.Mix(dark, 0.4), c.Mix(dark, 0.3)
	}
	return deep, a, b, c
}

// drawScene paints the scene of style for a cover of tint in r at drift
// seconds, lit by bass: what of it is soft enough for a canvas smaller
// than the window. Its water, if it has any, takes drops as music
// would let them fall.
func drawScene(p *ui.Painter, r ui.Rect, style string, tint ui.Color, drift, bass float32) {
	drawSceneDrops(p, r, style, tint, drift, bass, nil, nil)
}

// drawSceneDrops is drawScene with the drops the beat let fall, as
// beatState.uniform gives them, and the leaves afloat on it, as
// leafState.uniform does.
func drawSceneDrops(p *ui.Painter, r ui.Rect, style string, tint ui.Color, drift, bass float32, drops, leaves []float32) {
	sc := backdropSceneOf(style)
	if sc.shader == nil {
		drawFog(p, r, tint, drift, bass)
		return
	}
	deep, a, b, c := sceneColors(sc, tint)
	uniforms := map[string]any{
		"Time": drift, "Bass": min(bass, 1), "Deep": deep, "A": a, "B": b, "C": c,
	}
	if sc.drops {
		if drops == nil {
			drops = demoDrops(drift)
		}
		uniforms["Drops"] = drops
	}
	if sc.over != nil {
		// What is afloat on it, whose shadows it has.
		if leaves == nil {
			leaves = demoLeaves(drift)
		}
		uniforms["Leaves"] = leaves
	}
	sc.shader().Draw(p, r, shader.Options{Uniforms: uniforms})
}

// drawSceneOver paints over the scene of style, in r at the window's
// own size, what is too fine for its canvas: its stars, or what of it
// has edges, as opaque as the scene is; with the drops the beat let
// fall, or those of music playing for none.
func drawSceneOver(p *ui.Painter, r ui.Rect, style string, tint ui.Color, drift, bass, opacity float32, drops, leaves []float32) {
	sc := backdropSceneOf(style)
	if opacity <= 0 || sc.over == nil && sc.stars <= 0 {
		return
	}
	_, _, _, c := sceneColors(sc, tint)
	if sc.over != nil {
		if drops == nil {
			drops = demoDrops(drift)
		}
		if leaves == nil {
			leaves = demoLeaves(drift)
		}
		sc.over().Draw(p, r, shader.Options{Uniforms: map[string]any{
			"Time": drift, "Bass": min(bass, 1), "C": c, "Opacity": opacity, "Drops": drops, "Leaves": leaves,
		}})
	}
	if sc.stars <= 0 {
		return
	}
	shoot := float32(0)
	if sc.shoot {
		shoot = 1
	}
	starsOverShader().Draw(p, r, shader.Options{Uniforms: map[string]any{
		"Time": drift, "Bass": min(bass, 1), "C": c, "Opacity": opacity,
		"Dense": sc.stars, "Horizon": sc.horizon, "Shoot": shoot, "Moon": sc.moon[:],
	}})
}

// sceneDrops is how many drops a scene's water holds at once.
const sceneDrops = 8

// A drop fell on the water of a scene, with a beat: where on the
// water, as shares of it, when, in the drift's seconds, and how hard.
type sceneDrop struct{ x, y, at, hard float32 }

// beatState finds the beats in the low end of what plays, and keeps the
// drops they let fall.
type beatState struct {
	was, was2 float32 // the low end a moment ago, and one before
	mean, dev float32 // how far it has been leaping lately, and how unevenly
	last      float64 // when the last drop fell, in the drift's seconds
	drops     [sceneDrops]sceneDrop
	next      int
}

// beatGap is the least time between two beats, in seconds, beatLull how
// long the water lies still before a drop falls of itself, and beatMemory
// how long a song's leaps are remembered.
//
// Measured over the first 100 seconds of five songs, against the onsets
// of the low end taken finely from their samples: about a drop a second
// in songs with a beat, four in five of them on an onset, some 30 ms
// after it; a half of that in an orchestral one.
const (
	beatGap    = 0.28
	beatLull   = 2.4
	beatMemory = 1.2
)

// step takes low, the low end now, from 0 to 1, dt seconds after the
// last, at drift seconds: a beat is the low end leaping well beyond what
// its leaps have been lately, loud song or quiet, and lets a drop fall,
// as hard as the leap is beyond them. With none for a while, a small one
// falls of itself.
func (b *beatState) step(low, dt float32, drift float64, playing bool) {
	leap := max(low-min(b.was, b.was2), 0)
	b.was2, b.was = b.was, low
	need := b.mean + 2.4*b.dev + 0.02
	k := min(1, dt/beatMemory)
	b.mean += (leap - b.mean) * k
	b.dev += (max(leap-b.mean, b.mean-leap) - b.dev) * k
	if !playing {
		return
	}
	since := drift - b.last
	switch {
	case since >= beatGap && leap > need:
		b.fall(drift, min(1, 0.45+5*(leap-need)))
	case since >= beatLull:
		b.fall(drift, 0.4)
	}
}

// fall lets a drop fall at drift seconds, somewhere on the water.
func (b *beatState) fall(drift float64, hard float32) {
	b.last = drift
	b.drops[b.next] = sceneDrop{x: 0.06 + 0.88*rand.Float32(), y: rand.Float32(), at: float32(drift), hard: hard}
	b.next = (b.next + 1) % sceneDrops
}

// uniform returns the drops as the scenes' shaders take them.
func (b *beatState) uniform() []float32 {
	out := make([]float32, 0, 4*sceneDrops)
	for _, d := range b.drops {
		out = append(out, d.x, d.y, d.at, d.hard)
	}
	return out
}

// demoDrops are drops as music lets them fall, for a picture of a scene
// at drift seconds with no music playing: its card, a test.
func demoDrops(drift float32) []float32 {
	spots := [...][2]float32{{0.3, 0.26}, {0.74, 0.11}, {0.52, 0.78}, {0.16, 0.87}, {0.86, 0.57}, {0.45, 0.04}}
	out := make([]float32, 4*sceneDrops)
	for i, s := range spots {
		out[4*i], out[4*i+1], out[4*i+2], out[4*i+3] = s[0], s[1], drift-0.5-0.7*float32(i), 1-0.08*float32(i)
	}
	return out
}
