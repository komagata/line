package gui

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
)

// Deterministic fictional raster fixture: a toy cafe with an explicit DEMO
// sign. This is image content for the actual UI, not a screenshot mockup.
func demoPhotoBytes() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 1024, 680))
	rect := func(x, y, w, h int, c color.RGBA) {
		draw.Draw(img, image.Rect(x, y, x+w, y+h), &image.Uniform{C: c}, image.Point{}, draw.Src)
	}
	rect(0, 0, 1024, 680, color.RGBA{202, 220, 226, 255})
	rect(0, 480, 1024, 200, color.RGBA{163, 177, 157, 255})
	rect(80, 80, 864, 480, color.RGBA{235, 214, 181, 255})
	rect(60, 68, 904, 26, color.RGBA{80, 101, 96, 255})
	rect(115, 220, 792, 32, color.RGBA{67, 101, 96, 255})
	for i := 0; i < 9; i++ {
		rect(115+i*88, 252, 44, 28, color.RGBA{242, 236, 217, 255})
	}
	for _, x := range []int{148, 370, 700} {
		rect(x-10, 300, 190, 210, color.RGBA{90, 105, 99, 255})
		rect(x, 310, 170, 190, color.RGBA{109, 153, 163, 255})
		rect(x+82, 310, 6, 190, color.RGBA{237, 223, 196, 255})
		rect(x, 380, 170, 6, color.RGBA{237, 223, 196, 255})
	}
	rect(580, 300, 90, 260, color.RGBA{83, 108, 104, 255})
	rect(590, 312, 70, 160, color.RGBA{154, 184, 184, 255})
	rect(635, 478, 12, 8, color.RGBA{241, 219, 167, 255})
	glyphs := map[rune][]string{
		'D': {"11110", "10001", "10001", "10001", "10001", "10001", "11110"},
		'E': {"11111", "10000", "10000", "11110", "10000", "10000", "11111"},
		'M': {"10001", "11011", "10101", "10101", "10001", "10001", "10001"},
		'O': {"01110", "10001", "10001", "10001", "10001", "10001", "01110"},
		'C': {"01111", "10000", "10000", "10000", "10000", "10000", "01111"},
		'A': {"01110", "10001", "10001", "11111", "10001", "10001", "10001"},
		'F': {"11111", "10000", "10000", "11110", "10000", "10000", "10000"},
	}
	for i, r := range "DEMO CAFE" {
		for y, row := range glyphs[r] {
			for x, p := range row {
				if p == '1' {
					rect(247+i*60+x*10, 124+y*10, 10, 10, color.RGBA{52, 76, 73, 255})
				}
			}
		}
	}
	rect(85, 575, 854, 18, color.RGBA{122, 141, 129, 255})
	var out bytes.Buffer
	_ = jpeg.Encode(&out, img, &jpeg.Options{Quality: 90})
	return out.Bytes()
}
