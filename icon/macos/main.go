// Command macos-icon renders the app's icon as macOS lays one out: on a 1024
// pixel canvas, the icon on 824 of them, inset 100 from every edge, cut to
// the squircle macOS gives its icons and casting its small shadow.
//
// The Windows icon, resources/icon.png, fills its square to the edges, as
// Windows asks; put in a Dock as it is, it stands taller than the icons
// around it. This draws the same art in the shape and at the size a Mac
// shows it.
//
//	go run ./icon/macos            # writes icon/AppIcon-1024.png
//	go run ./icon/macos -o file.png
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"
)

const (
	canvas = 1024 // the icon's own size, in pixels
	inset  = 100  // what macOS leaves around an icon on the canvas
	radius = 185  // the corner of its squircle, on the 824 it takes up
	side   = canvas - 2*inset
	// superN is how far the corner is from a circle: macOS's squircle is a
	// superellipse, |x|^n + |y|^n = 1, flatter along the sides than a circle.
	superN = 4.6
	// shadow is how far the shadow reaches past the icon, and how deep.
	shadowBlur   = 22
	shadowDrop   = 12
	shadowAlpha  = 0.30
	shadowOffset = 0
)

func main() {
	out := flag.String("o", "icon/AppIcon-1024.png", "where to write the icon")
	src := flag.String("i", "resources/icon.png", "the square icon to draw from")
	flag.Parse()

	art, err := readPNG(*src)
	if err != nil {
		log.Fatal(err)
	}
	icon := squircle(art, side)
	img := onCanvas(icon, *out)

	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		log.Fatal(err)
	}
	f, err := os.Create(*out)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s: %dx%d, the icon on %d, inset %d\n", *out, canvas, canvas, side, inset)
}

func readPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return nil, err
	}
	return img, nil
}

// squircle scales art to size by size, keeping what shows within the
// squared circle of macOS's icons: the corners the art has in a square are
// cut to the shape a Mac draws.
func squircle(art image.Image, size int) *image.RGBA {
	b := art.Bounds()
	scaled := image.NewRGBA(image.Rect(0, 0, size, size))
	// Bilinear enough for a 1024 icon: the art is drawn from its own 1024.
	for y := 0; y < size; y++ {
		sy := b.Min.Y + y*b.Dy()/size
		for x := 0; x < size; x++ {
			sx := b.Min.X + x*b.Dx()/size
			scaled.Set(x, y, art.At(sx, sy))
		}
	}
	// The squircle as a coverage mask, antialiased on its edge.
	r := float64(radius)
	cx, cy := float64(size)/2, float64(size)/2
	out := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			px, py := float64(x)+0.5, float64(y)+0.5
			cover := inside(px, py, cx, cy, float64(size)/2, r)
			if cover <= 0 {
				continue
			}
			c := scaled.RGBAAt(x, y)
			if cover < 1 {
				a := float64(c.A) * cover
				c = color.RGBA{
					R: uint8(float64(c.R) * cover),
					G: uint8(float64(c.G) * cover),
					B: uint8(float64(c.B) * cover),
					A: uint8(a),
				}
			}
			out.SetRGBA(x, y, c)
		}
	}
	return out
}

// inside says how much of the pixel at (x, y) is within the squircle of
// center (cx, cy), half-size half and corner radius r: 1 well within it, 0
// well out, and a fraction across its edge.
func inside(x, y, cx, cy, half, r float64) float64 {
	dx, dy := math.Abs(x-cx), math.Abs(y-cy)
	// The straight sides, within the corners.
	if dx <= half-r || dy <= half-r {
		if dx > half+0.5 || dy > half+0.5 {
			return 0
		}
		return edge(math.Max(dx, dy), half)
	}
	// The corner: a superellipse on the box from (half-r) to half.
	nx, ny := (dx-(half-r))/r, (dy-(half-r))/r
	v := math.Pow(nx, superN) + math.Pow(ny, superN)
	// v = 1 on the curve; measure across it by how fast v grows.
	const eps = 0.02
	nx2, ny2 := nx*(1+eps), ny*(1+eps)
	dv := math.Pow(nx2, superN) + math.Pow(ny2, superN) - v
	if dv <= 0 {
		if v <= 1 {
			return 1
		}
		return 0
	}
	// How far past the curve, in pixels, the pixel's center is.
	past := (1 - v) / dv * eps * r
	return clamp01(0.5 + past)
}

// edge antialiases a straight side at distance d from the center, the icon
// reaching half.
func edge(d, half float64) float64 { return clamp01(half + 0.5 - d) }

func clamp01(v float64) float64 { return math.Max(0, math.Min(1, v)) }

// onCanvas puts the icon on the canvas where macOS keeps it, with the shadow
// it casts.
func onCanvas(icon *image.RGBA, out string) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, canvas, canvas))
	// The shadow: the icon's own shape, blurred and darkened, under it.
	shadow := shadowOf(icon)
	at := image.Rect(inset, inset, inset+side, inset+side)
	draw.Draw(img, at.Add(image.Pt(0, shadowOffset)), shadow, shadow.Bounds().Min, draw.Over)
	draw.Draw(img, at, icon, icon.Bounds().Min, draw.Over)
	return img
}

// shadowOf draws icon's alpha, blurred and dark, about the same size.
func shadowOf(icon *image.RGBA) *image.RGBA {
	b := icon.Bounds()
	pad := shadowBlur * 3
	sh := image.NewRGBA(image.Rect(-pad, -pad, b.Dx()+pad, b.Dy()+pad))
	// Alpha of the icon as darkness.
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			a := float64(icon.RGBAAt(x, y).A) / 255
			if a == 0 {
				continue
			}
			sh.SetRGBA(x, y+shadowDrop, color.RGBA{0, 0, 0, uint8(255 * a * shadowAlpha)})
		}
	}
	return blur(sh, float64(shadowBlur))
}

// blur blurs img's alpha with a box, twice, which is near enough a Gaussian.
func blur(img *image.RGBA, radius float64) *image.RGBA {
	r := int(math.Ceil(radius))
	for pass := 0; pass < 2; pass++ {
		img = boxPass(img, r, true)
		img = boxPass(img, r, false)
	}
	return img
}

func boxPass(img *image.RGBA, r int, horizontal bool) *image.RGBA {
	b := img.Bounds()
	out := image.NewRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			var sum, n int
			for d := -r; d <= r; d++ {
				sx, sy := x, y
				if horizontal {
					sx += d
				} else {
					sy += d
				}
				if !image.Pt(sx, sy).In(b) {
					continue
				}
				sum += int(img.RGBAAt(sx, sy).A)
				n++
			}
			if n == 0 {
				continue
			}
			out.SetRGBA(x, y, color.RGBA{0, 0, 0, uint8(sum / n)})
		}
	}
	return out
}
