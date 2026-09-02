//go:build ignore

package main

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

func main() {
	const size = 256
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	bg := color.RGBA{0x10, 0x11, 0x14, 255}
	fg := color.RGBA{0xc8, 0xf4, 0x64, 255}
	fillRoundRect(img, 0, 0, size-1, size-1, 56, bg)
	strokeEllipse(img, 128, 78, 70, 26, 6, fg)
	strokeLine(img, 58, 78, 58, 162, 6, fg)
	strokeLine(img, 198, 78, 198, 162, 6, fg)
	strokeEllipse(img, 128, 120, 70, 26, 6, fg)
	strokeEllipse(img, 128, 162, 70, 26, 6, fg)

	out, err := os.Create("desktop/packaging/appicon.png")
	if err != nil {
		panic(err)
	}
	defer out.Close()
	if err := png.Encode(out, img); err != nil {
		panic(err)
	}
}

func set(img *image.RGBA, x, y int, c color.RGBA) {
	if x < 0 || y < 0 || x >= img.Bounds().Dx() || y >= img.Bounds().Dy() {
		return
	}
	img.SetRGBA(x, y, c)
}

func fillRoundRect(img *image.RGBA, x0, y0, x1, y1, r int, c color.RGBA) {
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if insideRoundRect(x, y, x0, y0, x1, y1, r) {
				img.SetRGBA(x, y, c)
			}
		}
	}
}

func insideRoundRect(x, y, x0, y0, x1, y1, r int) bool {
	cx, cy := x, y
	if x < x0+r {
		cx = x0 + r
	} else if x > x1-r {
		cx = x1 - r
	}
	if y < y0+r {
		cy = y0 + r
	} else if y > y1-r {
		cy = y1 - r
	}
	if cx == x || cy == y {
		return true
	}
	dx := float64(x - cx)
	dy := float64(y - cy)
	return dx*dx+dy*dy <= float64(r*r)
}

func strokeEllipse(img *image.RGBA, cx, cy, rx, ry, width int, c color.RGBA) {
	hw := float64(width) / 2
	for y := cy - ry - width; y <= cy+ry+width; y++ {
		for x := cx - rx - width; x <= cx+rx+width; x++ {
			nx := float64(x-cx) / float64(rx)
			ny := float64(y-cy) / float64(ry)
			d := math.Hypot(nx, ny)
			outer := 1 + hw/float64(min(rx, ry))
			inner := 1 - hw/float64(min(rx, ry))
			if d <= outer && d >= inner {
				set(img, x, y, c)
			}
		}
	}
}

func strokeLine(img *image.RGBA, x0, y0, x1, y1, width int, c color.RGBA) {
	dx := float64(x1 - x0)
	dy := float64(y1 - y0)
	length := math.Hypot(dx, dy)
	if length == 0 {
		return
	}
	steps := int(length) + 1
	hw := float64(width) / 2
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		px := float64(x0) + dx*t
		py := float64(y0) + dy*t
		for oy := -width; oy <= width; oy++ {
			for ox := -width; ox <= width; ox++ {
				if math.Hypot(float64(ox), float64(oy)) <= hw {
					set(img, int(px)+ox, int(py)+oy, c)
				}
			}
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
