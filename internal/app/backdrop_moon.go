package app

// moonSource paints still water at night, looked across at a slant: a
// low moon of the cover's color in thin cloud, its light going out into
// the air about it; far hills in two ranges, the farther half lost in
// the haze, their feet in the mist that lies on the water; and the water
// under them, which holds it all upside down. Small waves cross it,
// finer the farther off, so that the moon's image comes apart toward us
// into a road of glints. Drops fall on it with the beat (Drops: where on
// the water, when, and how hard), each sending out rings that widen
// and die away, flattened by the slant, lit on the side that faces the
// moon and dark on the other.
const moonSource = sceneHead + sceneNoise + `
var Drops [8]vec4

const horizon = 0.32
const life = 4.5

// Where the moon is: across, as a share of the width, how far above the
// water, as a share of the height, and how large.
const moonAt = 0.6
const moonUp = 0.105
const moonR = 0.028

// plane is where on the water the point of the picture at height y (a
// share of it) and x across from its middle (in heights) lies: across,
// and away.
func plane(x float, y float) vec2 {
	z := 1.0 / (y - horizon + 0.2)
	return vec2(x*z*0.45, z)
}

// peak is a row of peaks along x, from 0 in their saddles to 1.
func peak(x float) float {
	return 1.0 - abs(sin(x))
}

// sky is the sky at x across (in heights) and up above the water (a
// share of the height), with its hills: the moon at mx, its face as it
// is in the sky, or, with soft of 1, as the water holds it, all light
// and no edge.
func sky(x float, up float, mx float, t float, soft float) vec3 {
	// The night, lighter toward the water.
	c := Deep.rgb * (0.42 + 1.0*exp(-max(up, 0.0)*3.4))
	rel := vec2(x-mx, up-moonUp)
	d := length(rel)
	// Thin cloud, lying in streaks, drifting.
	veil := smoothstep(0.4, 0.82, fbm(vec2(x*1.9+t*0.012, up*9.0)+vec2(3.1, 0.7)))
	// The moon's light in the air: far out, nearer, about its face, and
	// in the cloud.
	wide := 1.0 / (1.0 + d*d*10.0)
	near := exp(-d * 7.5)
	c += A.rgb * wide * (0.24 + 0.1*Bass)
	c += mix(A.rgb, B.rgb, 0.6) * near * (0.26 + 0.12*Bass)
	c += mix(B.rgb, C.rgb, 0.5) * 0.2 * exp(-d*24.0)
	c += mix(A.rgb, B.rgb, 0.5) * veil * (0.025 + 0.2*near + 0.09*wide)
	// The moon itself: soft at its edge, dimmer toward it, behind the
	// cloud.
	face := mix(B.rgb, C.rgb, 0.75)
	disc := (1.0 - smoothstep(moonR-0.008, moonR+0.005, d)) * (1.0 - soft)
	if disc > 0.0 {
		// Its seas, faintly.
		seas := noise(rel*42.0+vec2(4.3, 1.9)) + 0.5*noise(rel*95.0+vec2(1.1, 7.7)) - 0.75
		// Never darker than the light about it, whatever the cover.
		c = mix(c, max(face*(0.93-0.16*d*d/(moonR*moonR)+0.13*seas), c*(1.12+0.13*seas)), disc*(0.97-0.3*veil))
	}
	c += face * 0.8 * exp(-d*d*330.0) * soft
	// The hills: the far range half lost in the air, lighter under the
	// moon, where they part; the near one dark.
	under := 1.0 / (1.0 + (x-mx)*(x-mx)*5.0)
	// Their outlines are drawn, not found in the noise, which differs
	// from one machine to the next.
	far := (0.02 + 0.1*(0.5*peak(x*1.3+0.4)+0.32*peak(x*2.9+2.0)+0.18*(0.5+0.5*sin(x*7.1+1.0)))) * (1.0 - 0.5*exp(-(x-mx)*(x-mx)*7.0))
	c = mix(c, Deep.rgb*0.52+A.rgb*(0.02+0.07*under), 0.85*smoothstep(0.0, 0.009, far-up))
	nigh := 0.006 + 0.055*(0.6*peak(x*2.3+5.0)+0.4*(0.5+0.5*sin(x*5.9+1.3)))*(0.3+0.7*(0.5+0.5*sin(x*1.1+3.6)))
	c = mix(c, Deep.rgb*0.3+A.rgb*0.02*under, 0.94*smoothstep(0.0, 0.007, nigh-up))
	// The mist on the water, which hides their feet.
	mist := exp(-abs(up)*27.0) * (0.55 + 0.45*noise(vec2(x*2.2+t*0.02, 1.7)))
	c = mix(c, Deep.rgb*0.75+A.rgb*0.05+B.rgb*(0.13+0.27*under), 0.78*mist)
	return c
}

// turn is which way the water slopes at a corner of the noise.
func turn(p vec2) vec2 {
	a := hash(p) * 6.2831853
	return vec2(cos(a), sin(a))
}

// wave is a noise of slopes, about nothing, without the squares of the
// other: the water's.
func wave(p vec2) float {
	i := floor(p)
	f := fract(p)
	u := f * f * f * (f*(f*6.0-15.0) + 10.0)
	a := dot(turn(i), f)
	b := dot(turn(i+vec2(1, 0)), f-vec2(1, 0))
	c := dot(turn(i+vec2(0, 1)), f-vec2(0, 1))
	d := dot(turn(i+vec2(1, 1)), f-vec2(1, 1))
	return mix(mix(a, b, u.x), mix(c, d, u.x), u.y)
}

// swell is the slope of the small waves at pl on the water, across and
// away, the picture px pixels high: long low crests of three sizes, in
// two trains that cross one another and go opposite ways, so that the
// water glitters rather than slides; in gusts; each size gone where it
// is too fine to paint.
func swell(pl vec2, t float, px float) vec2 {
	z2 := pl.y * pl.y
	f1 := smoothstep(1.5, 4.5, 0.085*px/z2)
	f2 := smoothstep(1.5, 4.5, 0.039*px/z2)
	f3 := smoothstep(1.5, 4.5, 0.019*px/z2)
	pa := vec2(0.906*pl.x+0.423*pl.y, -0.423*pl.x+0.906*pl.y)
	pb := vec2(0.94*pl.x-0.342*pl.y, 0.342*pl.x+0.94*pl.y)
	away := 1.3 * f1 * (wave(pa*vec2(6.2, 20.0)+vec2(t*0.1, t*0.3)) + wave(pb*vec2(5.4, 23.0)+vec2(40.0-t*0.08, -t*0.26)))
	away += 0.8 * f2 * (wave(pb*vec2(14.0, 44.0)+vec2(t*0.16, t*0.5)) + wave(pa*vec2(12.0, 51.0)+vec2(17.0-t*0.13, -t*0.44)))
	away += 0.5 * f3 * (wave(pa*vec2(30.0, 90.0)+vec2(t*0.2, t*0.8)) + wave(pb*vec2(27.0, 101.0)+vec2(9.0-t*0.2, -t*0.7)))
	across := 1.5*f1*wave(pb*vec2(7.0, 17.0)+vec2(71.0+t*0.09, t*0.2)) + 1.0*f2*wave(pa*vec2(15.0, 40.0)+vec2(53.0-t*0.12, -t*0.4)) + 0.6*f3*wave(pb*vec2(31.0, 83.0)+vec2(23.0+t*0.2, t*0.6))
	gust := 0.55 + 0.9*noise(pl*vec2(2.6, 1.1)+vec2(t*0.05, 3.0))
	return vec2(across, away) * gust
}

func Fragment(dst vec4, pos vec2) vec4 {
	size := rectSize()
	uv := pos / size
	aspect := size.x / size.y
	x := uv.x * aspect
	mx := moonAt * aspect
	t := Time
	c := vec3(0)
	if uv.y < horizon {
		c = sky(x, horizon-uv.y, mx, t, 0.0)
	} else {
		below := uv.y - horizon
		depth := below / (1.0 - horizon)
		pl := plane(x-0.5*aspect, uv.y)
		// The rings of the drops: a few crests behind a front that
		// widens, lower as they spread, gone where too fine to paint.
		ring := vec2(0)
		splash := 0.0
		for i := 0; i < 8; i++ {
			dr := Drops[i]
			age := t - dr.z
			if dr.w > 0.0 && age > 0.0 && age < life {
				off := pl - plane((dr.x-0.5)*aspect, 0.5+0.46*dr.y)
				d := length(off)
				r := 0.012 + 0.058*age
				u := d - r
				k := (u + 0.036) / 0.052
				left := 1.0 - age/life
				amp := dr.w * smoothstep(0.0, 0.15, age) * pow(left, 1.3) / (1.0 + 3.0*r)
				ring += off / max(d, 0.0001) * (cos(u*150.0) * exp(-k*k) * amp)
				splash += exp(-d*d*7000.0) * dr.w * (1.0 - smoothstep(0.0, 0.32, age))
			}
		}
		ring *= smoothstep(1.5, 4.0, 0.042*size.y/(pl.y*pl.y))
		tilt := swell(pl, t, size.y)*(0.78+0.34*Bass) + ring*2.0
		// The water holds the sky upside down, each wave turning it a
		// little: more the nearer, so that the moon's image comes apart
		// into glints toward us.
		up := max(below*(1.0+tilt.y), 0.002)
		c = sky(x+tilt.x*(0.012+0.24*below), up, mx, t, 1.0)
		// Darker the more steeply we look into it; its glints as bright.
		hot := smoothstep(0.38, 0.72, max(c.r, max(c.g, c.b)))
		c *= mix(0.88-0.46*depth, 0.96, hot)
		c += Deep.rgb * 0.04
		// The mist lies out over the far water, drifting.
		c += mix(A.rgb, B.rgb, 0.6) * 0.07 * exp(-below*9.0) * (0.4 + 0.9*fbm(vec2(x*1.6+t*0.016, below*14.0)+vec2(1.3, 6.1)))
		// The rings catch the moon on the side that faces it.
		lit := dot(ring, normalize(vec2((mx-x)*0.7, 1.0)))
		path := 1.0 / (1.0 + (x-mx)*(x-mx)*7.0)
		c += mix(A.rgb, C.rgb, 0.55) * max(lit, 0.0) * (0.5 + 0.9*path)
		c *= 1.0 - 0.45*clamp(-lit, 0.0, 1.0)
		c += C.rgb * 0.45 * min(splash, 1.0)
	}
	// Darker at the corners.
	g := length((uv - vec2(0.56, 0.36)) * vec2(1.0, 1.15))
	c *= 1.05 - 0.4*smoothstep(0.3, 1.0, g)
	c += grain(pos)
	return vec4(clamp(c, 0.0, 1.0), 1)
}
`
