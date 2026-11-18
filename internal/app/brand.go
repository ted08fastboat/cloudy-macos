package app

import (
	"bytes"
	_ "embed"
	"image"
	"log"
	"math"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// Where the app's source is, as the AGPL asks of a modified version, and what
// it was made from.
const (
	appSource = "https://github.com/nya-owo/cloudy"
	appOrigin = "https://github.com/egoist/mygo-cloudmusic"
)

//go:embed brand/mascot.png
var mascotPNG []byte

//go:embed brand/icon.png
var brandPNG []byte

func embeddedBitmap(b []byte) func() *ui.Bitmap {
	return sync.OnceValue(func() *ui.Bitmap {
		img, _, err := image.Decode(bytes.NewReader(b))
		if err != nil {
			log.Printf("brand: %v", err)
			return nil
		}
		return ui.NewBitmap(img)
	})
}

// The little dinosaurs of the record on the app's icon, and the icon.
var (
	mascot    = embeddedBitmap(mascotPNG)
	brandIcon = embeddedBitmap(brandPNG)
)

// mascotView is who shows where a page has nothing to show.
func mascotView(c *ui.Context, size float32) ui.Element {
	return ui.Image(c, mascot()).Size(size, size).Fit(ui.Cover)
}

// aboutCard says what the app is, whose it is and where its source is.
func (a *app) aboutCard(k *settingsKit) {
	c, ink := k.c, k.ink
	ui.Row(c).Padding(16, 20).Gap(14).AlignItems(ui.Center).Children(func() {
		ui.Image(c, brandIcon()).Size(52, 52).Fit(ui.Cover).Shrink(0)
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(3).Children(func() {
			ui.Text(c, mygo.App.Name()+" "+mygo.App.Version()).FontSize(14).FontWeight(700)
			ui.Text(c, "由 nya 制作，基于 egoist/mygo-cloudmusic 修改，以 AGPL-3.0 开源。与网易云音乐官方无关").
				FontSize(12).TextColor(ink.muted).LineHeight(1.4)
		})
		if k.button("source", "源码", iconExternal, false, false).Clicked() {
			openLink(appSource)
		}
		if k.button("origin", "原项目", iconExternal, false, false).Clicked() {
			openLink(appOrigin)
		}
	})
}

// openLink opens a page in the browser, off the frame.
func openLink(url string) {
	go func() {
		if err := mygo.Shell.OpenExternal(url); err != nil {
			log.Printf("open %s: %v", url, err)
		}
	}()
}

// How the app comes in as it starts (1.3.2, the user's 方案 A): the app's
// icon, its arm lifting and a glint crossing it; the record rising off its
// base and spinning up, the base fading; the record flying into the
// player bar's cover as the veil over the app lifts, landing with a ring.
// Times in ms from the start; all of it is painted by one element, by the
// time, so that the frames are painted only, however busy the first page
// is as it loads under it.
const (
	splashEnd   = 1180 * time.Millisecond
	splashTile  = 196 // the icon's side
	splashLiftR = 0.5 // the record's radius risen, of the tile
)

// splash is the start of the app (above), over the window. It takes no
// clicks, for nothing to wait on it.
func (a *app) splash(c *ui.Context) {
	if a.splashed {
		return
	}
	if a.motion != motionFull {
		a.splashed = true
		return
	}
	now := c.Now()
	if a.splashAt.IsZero() {
		a.splashAt = now
	}
	if now.Sub(a.splashAt) >= splashEnd {
		a.splashed = true
		return
	}
	// A build once it is over, to take it away; the frames until then only paint.
	c.After(splashEnd - now.Sub(a.splashAt))
	bg, dark := a.pal.bg, a.pal.dark
	ui.Box(c.Key("splash")).Absolute().Top(0).Left(0).Right(0).Bottom(0).PassThrough().Draw(func(p *ui.Painter, r ui.Rect) {
		t := float32(p.Now().Sub(a.splashAt).Milliseconds())
		if t < float32(splashEnd.Milliseconds()) {
			p.AnimationFrame()
		}
		a.drawSplash(p, r, t, bg, dark)
	})
}

// drawSplash paints the start at t ms.
func (a *app) drawSplash(p *ui.Painter, r ui.Rect, t float32, bg ui.Color, dark bool) {
	S := float32(splashTile)
	side := a.sideWidth(r.W)
	cx, cy := r.X+side+(r.W-side)/2, r.Y+r.H/2-22
	span := func(from, to float32) float32 { return clamp01((t - from) / (to - from)) }
	out3 := func(x float32) float32 { return 1 - (1-x)*(1-x)*(1-x) }
	inOut3 := func(x float32) float32 {
		if x < .5 {
			return 4 * x * x * x
		}
		y := -2*x + 2
		return 1 - y*y*y/2
	}
	back := func(x float32) float32 { const k = 1.35; y := x - 1; return 1 + (k+1)*y*y*y + k*y*y }

	// The veil over the app, lifting as the record flies, with a red glow
	// in it about the icon, which goes as the base does.
	veil := 1 - out3(span(640, 1000))
	if veil > 0 {
		p.Fill(r, bg.Alpha(veil), 0)
		glow := out3(span(0, 260)) * (1 - inOut3(span(380, 760))) * veil
		if glow > 0 {
			g := float32(.16)
			if dark {
				g = .26
			}
			core := ui.Rect{X: cx - 60, Y: cy - 60, W: 120, H: 120}
			p.Shadow(core, 60, 0, 0, 260, 60, ui.RGBA(241, 43, 55, g*glow))
		}
	}

	// The icon: its base, the record on it, the arm on the record.
	iconIn := span(0, 260)
	baseOut := span(380, 660)
	baseAlpha := 1 - out3(baseOut)
	scale := lerp(.6, 1, back(iconIn)) * lerp(1, .9, out3(baseOut))
	if iconIn <= 0 {
		return
	}
	bx, by := cx-S*.1*inOut3(baseOut), cy+S*.04*inOut3(baseOut)
	w := S * scale
	tile := ui.Rect{X: bx - w/2, Y: by - w/2, W: w, H: w}
	lift := inOut3(span(320, 640))
	if baseAlpha > 0 {
		p.Shadow(tile, w*.23, 0, 18, 46, 0, ui.RGBA(150, 12, 24, .38*baseAlpha))
		p.FillGradient(tile, ui.LinearGradient{Angle: 160, Stops: []ui.GradientStop{
			{Color: ui.Hex("#ff5058").Alpha(baseAlpha), At: 0},
			{Color: ui.Hex("#f12b37").Alpha(baseAlpha), At: .55},
			{Color: ui.Hex("#d41b29").Alpha(baseAlpha), At: 1},
		}}, w*.23)
		p.FillGradient(tile, ui.LinearGradient{Angle: 180, Stops: []ui.GradientStop{
			{Color: white.Alpha(.26 * baseAlpha), At: 0}, {Color: white.Alpha(0), At: .5},
		}}, w*.23)
		// The platter's well, as the record leaves it.
		if lift > 0 {
			wr := .38 * w
			well := ui.Rect{X: bx - .06*w - wr, Y: by + .06*w - wr, W: 2 * wr, H: 2 * wr}
			p.Fill(well, ui.RGBA(90, 0, 10, .22*lift*baseAlpha), wr)
			p.Stroke(well, white.Alpha(.18*lift*baseAlpha), wr, 1)
			hub := w * .02
			p.Fill(ui.Rect{X: well.X + wr - hub, Y: well.Y + wr - hub, W: 2 * hub, H: 2 * hub}, white.Alpha(.5*lift*baseAlpha), hub)
		}
		// A glint crossing it, corner to corner.
		if g := span(180, 460); g > 0 && g < 1 {
			at := lerp(-.3, 1.3, g)
			p.FillGradient(tile, ui.LinearGradient{Angle: 135, Stops: []ui.GradientStop{
				{Color: white.Alpha(0), At: at - .14}, {Color: white.Alpha(.32 * baseAlpha), At: at}, {Color: white.Alpha(0), At: at + .14},
			}}, w*.23)
		}
		p.Stroke(tile, white.Alpha(.22*baseAlpha), w*.23, 1)
	}

	// The record: on the icon, risen over it, then into the bar.
	ox, oy, oR := bx-.06*w, by+.06*w, .38*w
	ux, uy, uR := cx+S*.06, cy-S*.04, S*splashLiftR
	rx, ry, R := lerp(ox, ux, lift), lerp(oy, uy, lift), lerp(oR, uR, lift)
	shade := lift
	to := a.barSpot.cover
	if to.W <= 0 {
		to = ui.Rect{X: r.X + side + 24, Y: r.Y + r.H - 70, W: 48, H: 48}
	}
	tx, ty, tR := to.X+to.W/2, to.Y+to.H/2, to.W/2
	if fly := span(620, 1000); fly > 0 {
		k := inOut3(fly)
		// An arc: up a little first, then down to the bar.
		c1x, c1y := lerp(rx, tx, .38), min(ry, ty)-80
		ax, ay, bx2, by2 := lerp(rx, c1x, k), lerp(ry, c1y, k), lerp(c1x, tx, k), lerp(c1y, ty, k)
		rx, ry, R = lerp(ax, bx2, k), lerp(ay, by2, k), lerp(R, tR, k)
		shade = lerp(shade, 0, k)
	}
	// Gone into the bar's own cover as it lands.
	if land := span(1000, 1080); land < 1 {
		disc := ui.Rect{X: rx - R, Y: ry - R, W: 2 * R, H: 2 * R}
		p.Shadow(disc, R, 0, R*(.06+.08*shade), R*(.25+.25*shade), 0, ui.RGBA(0, 0, 0, .45*(1-land)))
		if land == 0 {
			drawRecord(p, disc, mascot(), float32(math.Mod(float64(splashTurn(t)), 360)))
		}
	}

	// The arm: lifts off the record and goes.
	if armAlpha := 1 - out3(span(340, 480)); armAlpha > 0 {
		armLift := inOut3(span(200, 440))
		px, py := bx+.30*w, by-.30*w
		rest := float32(math.Atan2(.38, -.13))
		ang := rest - armLift*.62
		hx, hy := px+float32(math.Cos(float64(ang)))*.42*w, py+float32(math.Sin(float64(ang)))*.42*w
		arm := ui.Hex("#f4f4f6").Alpha(armAlpha)
		p.Line(px, py, hx, hy, w*.035, arm)
		ha := ang + .55
		p.Line(hx, hy, hx+float32(math.Cos(float64(ha)))*.1*w, hy+float32(math.Sin(float64(ha)))*.1*w, w*.06, arm)
		pr, hr := w*.07, w*.032
		p.Fill(ui.Rect{X: px - pr, Y: py - pr, W: 2 * pr, H: 2 * pr}, arm, pr)
		p.Fill(ui.Rect{X: px - hr, Y: py - hr, W: 2 * hr, H: 2 * hr}, ui.Hex("#c9c9cf").Alpha(armAlpha), hr)
	}

	// The name under the icon while it shows.
	if name := out3(span(90, 300)) * (1 - out3(span(380, 560))) * veil; name > 0 {
		ink := ui.RGBA(29, 29, 31, .82*name)
		if dark {
			ink = ui.RGBA(242, 242, 244, .86*name)
		}
		const size = 17
		p.Text(cx-30, cy+S*.5+28+8*(1-out3(span(90, 320))), "Cloudy", size, ink)
	}

	// Landing: a soft ring out of the bar's cover.
	if land := span(1000, 1180); land > 0 && land < 1 {
		g := 18 * out3(land)
		ring := ui.Rect{X: tx - tR - g, Y: ty - tR - g, W: 2 * (tR + g), H: 2 * (tR + g)}
		p.Stroke(ring, ui.RGBA(229, 56, 59, .45*(1-land)), tR+g, 2)
	}
}

// splashTurn is how far, in degrees, the record has turned by t ms: still
// on the icon, spinning up as it rises, easing down to the bar's turn.
func splashTurn(t float32) float32 {
	speed := func(s float32) float32 { // turns a second
		up := 1 - (1-clamp01((s-300)/320))*(1-clamp01((s-300)/320))*(1-clamp01((s-300)/320))
		down := clamp01((s - 760) / 390)
		down = down * down * (3 - 2*down)
		return lerp(up*1.8, .35, down)
	}
	var a float32
	for s := float32(0); s < t; s += 4 {
		a += speed(s) * 4 / 1000 * 360
	}
	return a
}
