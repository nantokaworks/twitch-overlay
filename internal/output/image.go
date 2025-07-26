package output

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/joeyak/go-twitch-eventsub/v3"
	"github.com/nantokaworks/twitch-fax/internal/env"
	"github.com/nantokaworks/twitch-fax/internal/twitchapi"
	"github.com/skip2/go-qrcode"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const PaperWidth = 384

// 下端の線の太さ（px）とテキスト下からのマージン（px）
const UnderlineHeight = 4
const UnderlineMargin = 10

// 破線設定
const UnderlineDashed = true  // true=破線, false=実線
const UnderlineDashLength = 8 // 線分の長さ(px)
const UnderlineDashGap = 4    // 線分間の間隔(px)

const fontSize = 32
const avatarSize = 100

// Common drawing functions

// drawHorizontalLine draws a horizontal line with optional margins
func drawHorizontalLine(img *image.RGBA, y, leftMargin, rightMargin, thickness int, c color.Color) {
	for lineY := 0; lineY < thickness; lineY++ {
		for x := leftMargin; x < PaperWidth-rightMargin; x++ {
			img.Set(x, y+lineY, c)
		}
	}
}

// drawCenteredText draws text centered horizontally
func drawCenteredText(d *font.Drawer, text string, yPos int) {
	bounds, _ := d.BoundString(text)
	textWidth := bounds.Max.X.Round() - bounds.Min.X.Round()
	d.Dot = fixed.Point26_6{
		X: fixed.I((PaperWidth - textWidth) / 2),
		Y: fixed.I(yPos) + d.Face.Metrics().Ascent,
	}
	d.DrawString(text)
}

// wrapFragments はテキスト/Emote/URL混合フラグメントを maxWidth で折り返し、行単位で返す
func wrapFragments(frags []twitch.ChatMessageFragment, face font.Face, maxWidth, lineHeight int) [][]twitch.ChatMessageFragment {
	var lines [][]twitch.ChatMessageFragment
	var curr []twitch.ChatMessageFragment
	currW := 0
	urlRe := regexp.MustCompile(`https?://\S+`)

	// 1文字 or Emote or URL 単位に展開
	var list []twitch.ChatMessageFragment
	for _, f := range frags {
		f.Text = strings.ReplaceAll(f.Text, "\n", "")
		if f.Emote != nil {
			list = append(list, f)
		} else if urlRe.MatchString(f.Text) {
			list = append(list, f)
		} else {
			for _, r := range f.Text {
				list = append(list, twitch.ChatMessageFragment{Text: string(r)})
			}
		}
	}

	// 折り返しロジック（URL は独立行扱い）
	for _, f := range list {
		if f.Emote == nil && urlRe.MatchString(f.Text) {
			if len(curr) > 0 {
				lines = append(lines, curr)
				curr = nil
				currW = 0
			}
			lines = append(lines, []twitch.ChatMessageFragment{f})
			continue
		}
		w := 0
		if f.Emote != nil {
			w = lineHeight
		} else {
			w = int((&font.Drawer{Face: face}).MeasureString(f.Text) >> 6)
		}
		if currW+w > maxWidth && len(curr) > 0 {
			lines = append(lines, curr)
			curr = nil
			currW = 0
		}
		curr = append(curr, f)
		currW += w
	}
	if len(curr) > 0 {
		lines = append(lines, curr)
	}
	return lines
}

// generateQR はテキストを QR に変換して image.Image を返す
func generateQR(text string, size int) (image.Image, error) {
	pngBytes, err := qrcode.Encode(text, qrcode.Medium, size)
	if err != nil {
		return nil, err
	}
	return png.Decode(bytes.NewReader(pngBytes))
}

// downloadEmote は URL から emote 画像を取得し、MIME タイプで PNG/JPEG/GIF を判別してデコード
func downloadEmote(url string) (image.Image, error) {
	// キャッシュディレクトリ準備
	cacheDir := ".cache"
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, err
	}
	// URLハッシュでファイル名生成
	h := sha1.Sum([]byte(url))
	cacheFile := filepath.Join(cacheDir, hex.EncodeToString(h[:]))
	// キャッシュから読み込み
	if data, err := os.ReadFile(cacheFile); err == nil {
		img, _, err := image.Decode(bytes.NewReader(data))
		return img, err
	}

	// ネットワークから取得
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	// キャッシュに保存（失敗しても処理継続）
	_ = os.WriteFile(cacheFile, data, 0644)

	ct := resp.Header.Get("Content-Type")
	switch {
	case strings.Contains(ct, "png"):
		return png.Decode(bytes.NewReader(data))
	case strings.Contains(ct, "gif"):
		return gif.Decode(bytes.NewReader(data))
	case strings.Contains(ct, "jpeg"), strings.Contains(ct, "jpg"):
		return jpeg.Decode(bytes.NewReader(data))
	default:
		// フォールバック：PNG→GIF→JPEG
		if img, err := png.Decode(bytes.NewReader(data)); err == nil {
			return img, nil
		}
		if img, err := gif.Decode(bytes.NewReader(data)); err == nil {
			return img, nil
		}
		return jpeg.Decode(bytes.NewReader(data))
	}
}

// resizeToHeight は元画像を指定高さにアスペクト比維持でリサイズ
func resizeToHeight(src image.Image, targetH int) image.Image {
	b := src.Bounds()
	w := b.Dx() * targetH / b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, targetH))
	xdraw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, b, xdraw.Over, nil)
	return dst
}

// resizeToWidth は元画像を幅 PaperWidth にアスペクト比維持でリサイズ
func resizeToWidth(src image.Image) image.Image {
	b := src.Bounds()
	h := b.Dy() * PaperWidth / b.Dx()
	dst := image.NewRGBA(image.Rect(0, 0, PaperWidth, h))
	xdraw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, b, xdraw.Over, nil)
	return dst
}

// rotate90 は画像を 90度回転
func rotate90(src image.Image) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, h, w))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(y, w-1-x, src.At(x, y))
		}
	}
	return dst
}

// MessageToImage creates an image from the message with optional color support
func MessageToImage(userName string, msg []twitch.ChatMessageFragment, useColor bool) (image.Image, error) {
	// 新しいフォントを作成（拡大文字）
	fontBytes, err := os.ReadFile("/Users/toka/Library/Fonts/HackGen-Bold.ttf")
	if err != nil {
		return nil, err
	}

	f, err := opentype.Parse(fontBytes)
	if err != nil {
		return nil, err
	}

	face, err := opentype.NewFace(f, &opentype.FaceOptions{
		Size:    fontSize,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, err
	}

	// フォントメトリクス取得
	ascent := int(face.Metrics().Ascent >> 6)
	descent := int(face.Metrics().Descent >> 6)
	lineHeight := int(face.Metrics().Height >> 6)

	// メッセージ改行削除＋URL分割
	var processed []twitch.ChatMessageFragment
	urlRe := regexp.MustCompile(`https?://\S+`)
	for _, frag := range msg {
		if frag.Emote != nil {
			processed = append(processed, frag)
			continue
		}
		text := strings.ReplaceAll(frag.Text, "\n", "")
		idxs := urlRe.FindAllStringIndex(text, -1)
		prev := 0
		for _, idx := range idxs {
			if idx[0] > prev {
				processed = append(processed, twitch.ChatMessageFragment{Text: text[prev:idx[0]]})
			}
			processed = append(processed, twitch.ChatMessageFragment{Text: text[idx[0]:idx[1]]})
			prev = idx[1]
		}
		if prev < len(text) {
			processed = append(processed, twitch.ChatMessageFragment{Text: text[prev:]})
		}
	}

	// 折り返し
	lines := wrapFragments(processed, face, PaperWidth, lineHeight)

	// 動的な高さ計算
	currH := ascent + descent
	for _, line := range lines {
		// URL-only 行
		if len(line) == 1 && urlRe.MatchString(line[0].Text) {
			img0, err := downloadEmote(line[0].Text)
			if err != nil {
				currH += PaperWidth
			} else {
				if img0.Bounds().Dx() > img0.Bounds().Dy() {
					img0 = rotate90(img0)
				}
				h := img0.Bounds().Dy() * PaperWidth / img0.Bounds().Dx()
				currH += h + PaperWidth
			}
			continue
		}
		// Emote-only 行
		var emoteFrags []twitch.ChatMessageFragment
		hasNonEmptyText := false
		for _, frag := range line {
			if frag.Emote != nil {
				emoteFrags = append(emoteFrags, frag)
			} else if strings.TrimSpace(frag.Text) != "" {
				hasNonEmptyText = true
				break
			}
		}
		if len(lines) == 1 && !hasNonEmptyText && len(emoteFrags) > 0 && len(emoteFrags) <= 8 {
			cellW := PaperWidth / len(emoteFrags)
			currH += cellW
			continue
		}

		// single-character text-only line
		if len(lines) == 1 && len(line) == 1 &&
			line[0].Emote == nil &&
			!urlRe.MatchString(line[0].Text) &&
			len([]rune(strings.TrimSpace(line[0].Text))) == 1 {
			text := strings.TrimSpace(line[0].Text)
			origW := int((&font.Drawer{Face: face}).MeasureString(text) >> 6)
			if origW > 0 {
				scale := float64(PaperWidth) / float64(origW)
				newSize := float64(fontSize) * scale
				face2, err := opentype.NewFace(f, &opentype.FaceOptions{
					Size:    newSize,
					DPI:     72,
					Hinting: font.HintingFull,
				})
				if err == nil {
					currH += int(face2.Metrics().Height >> 6)
					continue
				}
			}
		}
		currH += lineHeight
	}
	imgHeight := currH + UnderlineMargin + UnderlineHeight

	// 画像生成 - カラー版
	img := image.NewRGBA(image.Rect(0, 0, PaperWidth, imgHeight))
	// 白背景
	draw.Draw(img, img.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)

	// Drawer準備
	d := &font.Drawer{Dst: img, Src: image.Black, Face: face}

	// 1行目: userName
	d.Dot = fixed.Point26_6{X: fixed.I(0), Y: fixed.I(ascent)}
	d.DrawString(userName)

	// 2行目以降: 折返し後の行を描画
	for i, line := range lines {
		y := (i+1)*lineHeight + ascent

		// 全て Emote の場合の特別処理
		var emoteFrags []twitch.ChatMessageFragment
		hasNonEmptyText := false
		for _, frag := range line {
			if frag.Emote != nil {
				emoteFrags = append(emoteFrags, frag)
			} else if strings.TrimSpace(frag.Text) != "" {
				hasNonEmptyText = true
				break
			}
		}
		if !hasNonEmptyText && len(emoteFrags) > 0 && len(emoteFrags) <= 8 {
			cellW := PaperWidth / len(emoteFrags)
			for j, frag := range emoteFrags {
				url := fmt.Sprintf(
					"https://static-cdn.jtvnw.net/emoticons/v2/%s/static/light/3.0",
					frag.Emote.Id,
				)
				eimg, err := downloadEmote(url)
				if err != nil {
					continue
				}
				// 正方形(cellW×cellW)にリサイズ
				dst := image.NewRGBA(image.Rect(0, 0, cellW, cellW))
				xdraw.ApproxBiLinear.Scale(dst, dst.Bounds(), eimg, eimg.Bounds(), xdraw.Over, nil)
				// カラーモードでない場合はグレースケール変換
				var drawImg image.Image = dst
				if !useColor {
					drawImg = convertToGrayscaleWithDithering(dst)
				}
				draw.Draw(img,
					image.Rect(j*cellW, y-ascent, j*cellW+cellW, y-ascent+cellW),
					drawImg, image.Point{}, draw.Over)
			}
			continue
		}

		// single-character text-only line
		if len(line) == 1 &&
			line[0].Emote == nil &&
			!urlRe.MatchString(line[0].Text) &&
			len([]rune(strings.TrimSpace(line[0].Text))) == 1 {
			text := strings.TrimSpace(line[0].Text)
			origW := int(d.MeasureString(text) >> 6)
			if origW > 0 {
				scale := float64(PaperWidth) / float64(origW)
				newSize := float64(fontSize) * scale
				face2, err := opentype.NewFace(f, &opentype.FaceOptions{
					Size:    newSize,
					DPI:     72,
					Hinting: font.HintingFull,
				})
				if err == nil {
					ascent2 := int(face2.Metrics().Ascent >> 6)
					d2 := &font.Drawer{Dst: img, Src: image.Black, Face: face2}
					w2 := int(d2.MeasureString(text) >> 6)
					x2 := (PaperWidth - w2) / 2
					d2.Dot = fixed.Point26_6{
						X: fixed.I(x2),
						Y: fixed.I(y - ascent + ascent2),
					}
					d2.DrawString(text)
				} else {
					x := (PaperWidth - origW) / 2
					d.Dot = fixed.Point26_6{X: fixed.I(x), Y: fixed.I(y)}
					d.DrawString(text)
				}
			}
			continue
		}

		x := 0
		for _, frag := range line {
			// URL-only 行：画像＋QR
			if frag.Emote == nil && urlRe.MatchString(frag.Text) {
				img0, err := downloadEmote(frag.Text)
				if err == nil {
					if img0.Bounds().Dx() > img0.Bounds().Dy() {
						img0 = rotate90(img0)
					}
					img0 = resizeToWidth(img0)
					// カラーモードでない場合はグレースケール変換
					var drawImg image.Image = img0
					if !useColor {
						drawImg = convertToGrayscaleWithDithering(img0)
					}
					draw.Draw(img,
						image.Rect(0, y-ascent, PaperWidth, y-ascent+drawImg.Bounds().Dy()),
						drawImg, image.Point{}, draw.Over)
					// QR
					qrImg, err := generateQR(frag.Text, PaperWidth)
					if err == nil {
						draw.Draw(img,
							image.Rect(0, y-ascent+img0.Bounds().Dy(), PaperWidth, y-ascent+img0.Bounds().Dy()+PaperWidth),
							qrImg, image.Point{}, draw.Over)
					}
					x = PaperWidth
					continue
				}
				// 画像取得失敗→QR のみ
				qrImg, err := generateQR(frag.Text, PaperWidth)
				if err != nil {
					continue
				}
				draw.Draw(img,
					image.Rect(0, y-ascent, PaperWidth, y-ascent+PaperWidth),
					qrImg, image.Point{}, draw.Over)
				x = PaperWidth
				continue
			}

			// Emote
			if frag.Emote != nil {
				url := fmt.Sprintf(
					"https://static-cdn.jtvnw.net/emoticons/v2/%s/static/light/3.0",
					frag.Emote.Id,
				)
				eimg, err := downloadEmote(url)
				if err != nil {
					continue
				}
				eimg = resizeToHeight(eimg, lineHeight)
				// カラーモードでない場合はグレースケール変換
				var drawEmote image.Image = eimg
				if !useColor {
					drawEmote = convertToGrayscaleWithDithering(eimg)
				}
				draw.Draw(img,
					image.Rect(x, y-ascent, x+drawEmote.Bounds().Dx(), y-ascent+drawEmote.Bounds().Dy()),
					drawEmote, image.Point{}, draw.Over)
				x += eimg.Bounds().Dx()
				continue
			}

			// 通常テキスト
			d.Dot = fixed.Point26_6{X: fixed.I(x), Y: fixed.I(y)}
			d.DrawString(frag.Text)
			x += int(d.MeasureString(frag.Text) >> 6)
		}
	}

	// 下線描画
	underlineY := currH + UnderlineMargin
	if UnderlineDashed {
		for x0 := 0; x0 < PaperWidth; x0 += UnderlineDashLength + UnderlineDashGap {
			end := x0 + UnderlineDashLength
			if end > PaperWidth {
				end = PaperWidth
			}
			for y := 0; y < UnderlineHeight; y++ {
				for x := x0; x < end; x++ {
					img.Set(x, underlineY+y, color.Black)
				}
			}
		}
	} else {
		for y := 0; y < UnderlineHeight; y++ {
			for x := 0; x < PaperWidth; x++ {
				img.Set(x, underlineY+y, color.Black)
			}
		}
	}

	return img, nil
}


// convertToGrayscaleWithDithering converts a color image to grayscale with optional dithering
func convertToGrayscaleWithDithering(src image.Image) image.Image {
	bounds := src.Bounds()
	gray := image.NewGray(bounds)
	
	// First pass: Convert to grayscale with proper luminance weights
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := src.At(x, y).RGBA()
			// Use standard luminance weights
			lum := uint8((19595*r + 38470*g + 7471*b + 1<<15) >> 24)
			gray.SetGray(x, y, color.Gray{lum})
		}
	}
	
	// Use BLACK_POINT setting for threshold (0.0 to 1.0, default 0.5)
	threshold := uint8(env.Value.BlackPoint * 255)
	
	// Second pass: Apply dithering or simple threshold based on DITHER setting
	if env.Value.Dither {
		// Apply Floyd-Steinberg dithering for better print quality
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				oldPixel := gray.GrayAt(x, y).Y
				newPixel := uint8(0)
				if oldPixel > threshold {
					newPixel = 255
				}
				gray.SetGray(x, y, color.Gray{newPixel})
			
				// Calculate error
				err := int(oldPixel) - int(newPixel)
				
				// Distribute error to neighboring pixels
				if x+1 < bounds.Max.X {
					c := gray.GrayAt(x+1, y).Y
					gray.SetGray(x+1, y, color.Gray{uint8(clamp(int(c) + err*7/16))})
				}
				if y+1 < bounds.Max.Y {
					if x-1 >= bounds.Min.X {
						c := gray.GrayAt(x-1, y+1).Y
						gray.SetGray(x-1, y+1, color.Gray{uint8(clamp(int(c) + err*3/16))})
					}
					c := gray.GrayAt(x, y+1).Y
					gray.SetGray(x, y+1, color.Gray{uint8(clamp(int(c) + err*5/16))})
					if x+1 < bounds.Max.X {
						c := gray.GrayAt(x+1, y+1).Y
						gray.SetGray(x+1, y+1, color.Gray{uint8(clamp(int(c) + err*1/16))})
					}
				}
			}
		}
	} else {
		// Simple threshold without dithering
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				oldPixel := gray.GrayAt(x, y).Y
				newPixel := uint8(0)
				if oldPixel > threshold {
					newPixel = 255
				}
				gray.SetGray(x, y, color.Gray{newPixel})
			}
		}
	}
	
	return gray
}

func clamp(v int) int {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}

// downloadAndResizeAvatarGray downloads, resizes and converts an avatar image to grayscale
func downloadAndResizeAvatarGray(url string, size int) (image.Image, error) {
	// Download image
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	// Decode image
	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return nil, err
	}
	
	// Create resized image
	resized := image.NewRGBA(image.Rect(0, 0, size, size))
	xdraw.ApproxBiLinear.Scale(resized, resized.Bounds(), img, img.Bounds(), xdraw.Over, nil)
	
	// Convert to grayscale with dithering
	return convertToGrayscaleWithDithering(resized), nil
}

// GenerateTimeImageWithStats creates a monochrome image with time and Twitch channel statistics
func GenerateTimeImageWithStats(timeStr string) (image.Image, error) {
	// Get bits leaders
	monthLeaders, allTimeLeaders := getBitsLeaders()
	
	// Load font
	fontBytes, err := os.ReadFile("/Users/toka/Library/Fonts/HackGen-Bold.ttf")
	if err != nil {
		return nil, fmt.Errorf("failed to load font: %w", err)
	}
	
	f, err := opentype.Parse(fontBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse font: %w", err)
	}
	
	// Large font for time
	timeFace, err := opentype.NewFace(f, &opentype.FaceOptions{
		Size:    48,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create time font face: %w", err)
	}
	defer timeFace.Close()
	
	// Medium font for stats
	statsFace, err := opentype.NewFace(f, &opentype.FaceOptions{
		Size:    36,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create stats font face: %w", err)
	}
	defer statsFace.Close()
	
	// Small font for Bits count
	smallFace, err := opentype.NewFace(f, &opentype.FaceOptions{
		Size:    24,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create small font face: %w", err)
	}
	defer smallFace.Close()
	
	// Calculate image height (matching color version)
	padding := 20
	lineSpacing := 10
	baseHeight := padding*2 + 48 + 36 + 10 + 20
	
	// Add height for bits leaders if present
	extraHeight := 0
	if len(monthLeaders) > 0 || len(allTimeLeaders) > 0 {
		// Calculate height for monthly leaders
		if len(monthLeaders) > 0 {
			// Separator + title
			extraHeight += 20 + 24 + 10
			// First place with avatar
			extraHeight += 128 + 10 + 36 + 36 + lineSpacing
			// 2nd and 3rd place without avatar (smaller font)
			for i := 1; i < len(monthLeaders) && i < 3; i++ {
				extraHeight += 24 + 24 + lineSpacing
			}
		}
		// Calculate height for all-time leaders
		if len(allTimeLeaders) > 0 {
			// Separator + title
			extraHeight += 20 + 24 + 10
			// First place with avatar
			extraHeight += 128 + 10 + 36 + 36 + lineSpacing
			// 2nd and 3rd place without avatar (smaller font)
			for i := 1; i < len(allTimeLeaders) && i < 3; i++ {
				extraHeight += 24 + 24 + lineSpacing
			}
		}
	}
	
	height := baseHeight + extraHeight
	
	// Create image with white background
	img := image.NewRGBA(image.Rect(0, 0, PaperWidth, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)
	
	// Draw top separator
	drawHorizontalLine(img, 0, 0, 0, 1, color.Black)
	
	// Setup drawer
	d := &font.Drawer{
		Dst:  img,
		Src:  image.Black,
		Face: timeFace,
	}
	
	// Draw time centered
	drawCenteredText(d, timeStr, padding)
	
	// Draw date
	yPos := padding + 48 + 10
	now := time.Now()
	dateStr := now.Format("2006/01/02")
	d.Face = statsFace
	drawCenteredText(d, dateStr, yPos)
	
	// Calculate starting position for content
	yPos = baseHeight - 20
	
	// Draw monthly bits leaders
	if len(monthLeaders) > 0 {
		// Draw separator line with margins
		yPos += 10
		drawHorizontalLine(img, yPos, 20, 20, 2, color.Black)
		yPos += 15  // Space after separator
		
		// Section title
		d.Face = smallFace
		titleStr := "今月のトップCheer"
		drawCenteredText(d, titleStr, yPos)
		yPos += 24 + 10  // Title height + space
		
		// Draw leaders
		for i, leader := range monthLeaders {
			if i >= 3 {
				break
			}
			
			if i == 0 {
				// First place with avatar
				avatarLocalSize := 128
				avatarDrawn := false
				if leader.AvatarURL != "" {
					avatarImg, err := downloadAndResizeAvatarGray(leader.AvatarURL, avatarLocalSize)
					if err == nil {
						avatarX := (PaperWidth - avatarLocalSize) / 2
						draw.Draw(img, image.Rect(avatarX, yPos, avatarX+avatarLocalSize, yPos+avatarLocalSize),
							avatarImg, image.Point{}, draw.Over)
						yPos += avatarLocalSize
						avatarDrawn = true
					}
				}
				
				// Leader name
				d.Face = statsFace
				d.Src = image.Black
				if avatarDrawn {
					yPos += 10
				} else {
					yPos += 36  // Match color version spacing when no avatar
				}
				drawCenteredText(d, leader.UserName, yPos)
				
				// Bits count
				yPos += 36
				bitsStr := fmt.Sprintf("%d Bits", leader.Score)
				drawCenteredText(d, bitsStr, yPos)
				yPos += 36 + 10  // Bits height + line spacing
			} else {
				// 2nd/3rd place
				d.Face = smallFace
				d.Src = image.NewUniform(color.Gray{128})
				placeStr := fmt.Sprintf("%d位 %s", i+1, leader.UserName)
				drawCenteredText(d, placeStr, yPos)
				
				// Bits count
				yPos += 24
				bitsStr := fmt.Sprintf("%d Bits", leader.Score)
				drawCenteredText(d, bitsStr, yPos)
				yPos += 24 + 10  // Bits height + line spacing
			}
		}
		
	}
	
	// Draw all-time bits leaders
	if len(allTimeLeaders) > 0 {
		// Space between sections, or draw separator if both exist
		if len(monthLeaders) > 0 {
			// Draw separator if both exist
			yPos += 10  // Reduced from 20
			drawHorizontalLine(img, yPos, 20, 20, 2, color.Black)
			yPos += 15
		} else {
			yPos += 15
		}
		
		// Section title
		d.Face = smallFace
		d.Src = image.Black
		titleStr := "全期間トップCheer"
		drawCenteredText(d, titleStr, yPos)
		yPos += 24 + 10  // Title height + space
		
		// Draw leaders
		for i, leader := range allTimeLeaders {
			if i >= 3 {
				break
			}
			
			if i == 0 {
				// First place with avatar
				avatarLocalSize := 128
				avatarDrawn := false
				if leader.AvatarURL != "" {
					avatarImg, err := downloadAndResizeAvatarGray(leader.AvatarURL, avatarLocalSize)
					if err == nil {
						avatarX := (PaperWidth - avatarLocalSize) / 2
						draw.Draw(img, image.Rect(avatarX, yPos, avatarX+avatarLocalSize, yPos+avatarLocalSize),
							avatarImg, image.Point{}, draw.Over)
						yPos += avatarLocalSize
						avatarDrawn = true
					}
				}
				
				// Leader name
				d.Face = statsFace
				d.Src = image.Black
				if avatarDrawn {
					yPos += 10
				} else {
					yPos += 36  // Match color version spacing when no avatar
				}
				drawCenteredText(d, leader.UserName, yPos)
				
				// Bits count
				yPos += 36
				bitsStr := fmt.Sprintf("%d Bits", leader.Score)
				drawCenteredText(d, bitsStr, yPos)
				yPos += 36 + 10  // Bits height + line spacing
			} else {
				// 2nd/3rd place
				d.Face = smallFace
				d.Src = image.NewUniform(color.Gray{128})
				placeStr := fmt.Sprintf("%d位 %s", i+1, leader.UserName)
				drawCenteredText(d, placeStr, yPos)
				
				// Bits count
				yPos += 24
				bitsStr := fmt.Sprintf("%d Bits", leader.Score)
				drawCenteredText(d, bitsStr, yPos)
				yPos += 24 + 10  // Bits height + line spacing
			}
		}
	}
	
	// Draw bottom separator (dashed)
	lineY := height - 10
	for x := 10; x < PaperWidth-10; x += 4 {
		for y := 0; y < 2; y++ {
			img.Set(x, lineY+y, color.Black)
		}
	}
	
	return img, nil
}


// getBitsLeaders gets the top bits cheerers for month and all time
func getBitsLeaders() (monthLeaders, allTimeLeaders []*twitchapi.BitsLeaderboardEntry) {
	// Get monthly leaders
	monthLeaders, err := twitchapi.GetBitsLeaderboard("month")
	if err != nil {
		fmt.Printf("Failed to get monthly bits leaders: %v\n", err)
		monthLeaders = nil
	} else if len(monthLeaders) == 0 {
		fmt.Printf("No monthly bits leaders found\n")
	} else {
		fmt.Printf("Clock: Fetched %d monthly bits leaders:\n", len(monthLeaders))
		for i, leader := range monthLeaders {
			fmt.Printf("  #%d: %s with %d bits (avatar: %v)\n", i+1, leader.UserName, leader.Score, leader.AvatarURL != "")
		}
	}
	
	// Get all time leaders
	allTimeLeaders, err = twitchapi.GetBitsLeaderboard("all")
	if err != nil {
		fmt.Printf("Failed to get all-time bits leaders: %v\n", err)
		allTimeLeaders = nil
	} else if len(allTimeLeaders) == 0 {
		fmt.Printf("No all-time bits leaders found\n")
	} else {
		fmt.Printf("Clock: Fetched %d all-time bits leaders:\n", len(allTimeLeaders))
		for i, leader := range allTimeLeaders {
			fmt.Printf("  #%d: %s with %d bits (avatar: %v)\n", i+1, leader.UserName, leader.Score, leader.AvatarURL != "")
		}
	}
	
	return monthLeaders, allTimeLeaders
}

// downloadAndResizeAvatarColor downloads and resizes an avatar image in color
func downloadAndResizeAvatarColor(url string, size int) (image.Image, error) {
	// Download image
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	// Decode image
	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return nil, err
	}
	
	// Create resized image
	resized := image.NewRGBA(image.Rect(0, 0, size, size))
	xdraw.CatmullRom.Scale(resized, resized.Bounds(), img, img.Bounds(), xdraw.Over, nil)
	
	return resized, nil
}


// GenerateTimeImageWithStatsColor creates a color image with time and Twitch channel statistics
func GenerateTimeImageWithStatsColor(timeStr string) (image.Image, error) {
	// Get bits leaders
	monthLeaders, allTimeLeaders := getBitsLeaders()
	
	// Load font
	fontBytes, err := os.ReadFile("/Users/toka/Library/Fonts/HackGen-Bold.ttf")
	if err != nil {
		return nil, fmt.Errorf("failed to load font: %w", err)
	}
	
	f, err := opentype.Parse(fontBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse font: %w", err)
	}
	
	// Large font for time
	timeFace, err := opentype.NewFace(f, &opentype.FaceOptions{
		Size:    48,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create time font face: %w", err)
	}
	defer timeFace.Close()
	
	// Medium font for stats
	statsFace, err := opentype.NewFace(f, &opentype.FaceOptions{
		Size:    36,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create stats font face: %w", err)
	}
	defer statsFace.Close()
	
	// Calculate image height based on content
	padding := 20
	lineSpacing := 10
	baseHeight := padding*2 + 48 + 36 + 10 + 20
	
	// Add height for bits leaders if present
	extraHeight := 0
	if len(monthLeaders) > 0 || len(allTimeLeaders) > 0 {
		// Calculate height for monthly leaders
		if len(monthLeaders) > 0 {
			// Separator + title
			extraHeight += 20 + 24 + 10
			// First place with avatar
			extraHeight += 128 + 10 + 36 + 36 + lineSpacing
			// 2nd and 3rd place without avatar (smaller font)
			for i := 1; i < len(monthLeaders) && i < 3; i++ {
				extraHeight += 24 + 24 + lineSpacing
			}
		}
		// Calculate height for all-time leaders
		if len(allTimeLeaders) > 0 {
			// Separator + title
			extraHeight += 20 + 24 + 10
			// First place with avatar
			extraHeight += 128 + 10 + 36 + 36 + lineSpacing
			// 2nd and 3rd place without avatar (smaller font)
			for i := 1; i < len(allTimeLeaders) && i < 3; i++ {
				extraHeight += 24 + 24 + lineSpacing
			}
		}
	}
	
	imgHeight := baseHeight + extraHeight
	img := image.NewRGBA(image.Rect(0, 0, PaperWidth, imgHeight))
	
	// Fill with white background
	draw.Draw(img, img.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)
	
	// Draw time centered in black
	d := &font.Drawer{
		Face: timeFace,
		Dst:  img,
		Src:  image.Black,
	}
	
	bounds, _ := d.BoundString(timeStr)
	timeWidth := bounds.Max.X.Round() - bounds.Min.X.Round()
	d.Dot = fixed.Point26_6{
		X: fixed.I((PaperWidth - timeWidth) / 2),
		Y: fixed.I(padding) + timeFace.Metrics().Ascent,
	}
	d.DrawString(timeStr)
	
	// Draw date with smaller font in black
	d.Face = statsFace
	d.Src = image.Black
	dateStr := time.Now().Format("2006/01/02")
	bounds, _ = d.BoundString(dateStr)
	dateWidth := bounds.Max.X.Round() - bounds.Min.X.Round()
	d.Dot = fixed.Point26_6{
		X: fixed.I((PaperWidth - dateWidth) / 2),
		Y: fixed.I(padding + 48 + 10) + statsFace.Metrics().Ascent,
	}
	d.DrawString(dateStr)
	
	// Draw bits leaders if available
	yPos := padding + 48 + 10 + 36 + 10  // padding + time + space + date + space
	if len(monthLeaders) > 0 || len(allTimeLeaders) > 0 {
		// Draw separator line in black
		yPos += 10
		drawHorizontalLine(img, yPos, 20, 20, 2, color.Black)
		
		// Small font for leader sections
		smallFace, err := opentype.NewFace(f, &opentype.FaceOptions{
			Size:    24,
			DPI:     72,
			Hinting: font.HintingFull,
		})
		if err == nil {
			defer smallFace.Close()
			d.Face = smallFace
			
			// Monthly leaders
			if len(monthLeaders) > 0 {
				yPos += 15  // Space after separator
				titleText := "今月のトップCheer"
				d.Src = image.Black
				bounds, _ = d.BoundString(titleText)
				titleWidth := bounds.Max.X.Round() - bounds.Min.X.Round()
				d.Dot = fixed.Point26_6{
					X: fixed.I((PaperWidth - titleWidth) / 2),
					Y: fixed.I(yPos) + smallFace.Metrics().Ascent,
				}
				d.DrawString(titleText)
				yPos += 24 + 10  // Title height + space
				
				// Draw leaders
				for i, leader := range monthLeaders {
					if i >= 3 {
						break // Only show top 3
					}
					
					if i == 0 {
						// First place - with avatar and larger font
						// Draw avatar if available
						avatarSize := 128
						avatarDrawn := false
						if leader.AvatarURL != "" {
							avatarImg, err := downloadAndResizeAvatarColor(leader.AvatarURL, avatarSize)
							if err == nil {
								avatarX := (PaperWidth - avatarSize) / 2
								draw.Draw(img, image.Rect(avatarX, yPos, avatarX+avatarSize, yPos+avatarSize),
									avatarImg, image.Point{}, draw.Over)
								yPos += avatarSize
								avatarDrawn = true
							}
						}
						
						// Leader name
						d.Face = statsFace
						d.Src = image.Black
						if avatarDrawn {
							yPos += 10
						} else {
							yPos += 36
						}
						leaderText := leader.UserName
						bounds, _ = d.BoundString(leaderText)
						leaderWidth := bounds.Max.X.Round() - bounds.Min.X.Round()
						d.Dot = fixed.Point26_6{
							X: fixed.I((PaperWidth - leaderWidth) / 2),
							Y: fixed.I(yPos) + statsFace.Metrics().Ascent,
						}
						d.DrawString(leaderText)
						
						// Bits count
						yPos += 36
						bitsText := fmt.Sprintf("%d Bits", leader.Score)
						d.Src = image.Black
						bounds, _ = d.BoundString(bitsText)
						bitsWidth := bounds.Max.X.Round() - bounds.Min.X.Round()
						d.Dot = fixed.Point26_6{
							X: fixed.I((PaperWidth - bitsWidth) / 2),
							Y: fixed.I(yPos) + statsFace.Metrics().Ascent,
						}
						d.DrawString(bitsText)
						yPos += 36 + lineSpacing
					} else {
						// 2nd and 3rd place - smaller font, no avatar
						d.Face = smallFace
						d.Src = image.NewUniform(color.RGBA{100, 100, 100, 255})
						
						// Place and name
						placeText := fmt.Sprintf("%d位 %s", i+1, leader.UserName)
						bounds, _ = d.BoundString(placeText)
						placeWidth := bounds.Max.X.Round() - bounds.Min.X.Round()
						d.Dot = fixed.Point26_6{
							X: fixed.I((PaperWidth - placeWidth) / 2),
							Y: fixed.I(yPos) + smallFace.Metrics().Ascent,
						}
						d.DrawString(placeText)
						
						// Bits count
						yPos += 24
						bitsText := fmt.Sprintf("%d Bits", leader.Score)
						d.Src = image.NewUniform(color.RGBA{100, 100, 100, 255})
						bounds, _ = d.BoundString(bitsText)
						bitsWidth := bounds.Max.X.Round() - bounds.Min.X.Round()
						d.Dot = fixed.Point26_6{
							X: fixed.I((PaperWidth - bitsWidth) / 2),
							Y: fixed.I(yPos) + smallFace.Metrics().Ascent,
						}
						d.DrawString(bitsText)
						yPos += 24 + lineSpacing
					}
				}
			}
			
			// All time leaders
			if len(allTimeLeaders) > 0 {
				if len(monthLeaders) > 0 {
					// Draw separator if both exist
					yPos += 10  // Reduced from 20
					drawHorizontalLine(img, yPos, 20, 20, 2, color.Black)
					yPos += 15
				} else {
					yPos += 15
				}
				
				d.Face = smallFace
				titleText := "全期間トップCheer"
				d.Src = image.Black
				bounds, _ = d.BoundString(titleText)
				titleWidth := bounds.Max.X.Round() - bounds.Min.X.Round()
				d.Dot = fixed.Point26_6{
					X: fixed.I((PaperWidth - titleWidth) / 2),
					Y: fixed.I(yPos) + smallFace.Metrics().Ascent,
				}
				d.DrawString(titleText)
				yPos += 24 + 10  // Title height + space
				
				// Draw leaders
				for i, leader := range allTimeLeaders {
					if i >= 3 {
						break // Only show top 3
					}
					
					if i == 0 {
						// First place - with avatar and larger font
						// Draw avatar if available
						avatarSize := 128
						avatarDrawn := false
						if leader.AvatarURL != "" {
							avatarImg, err := downloadAndResizeAvatarColor(leader.AvatarURL, avatarSize)
							if err == nil {
								avatarX := (PaperWidth - avatarSize) / 2
								draw.Draw(img, image.Rect(avatarX, yPos, avatarX+avatarSize, yPos+avatarSize),
									avatarImg, image.Point{}, draw.Over)
								yPos += avatarSize
								avatarDrawn = true
							}
						}
						
						// Leader name
						d.Face = statsFace
						d.Src = image.Black
						if avatarDrawn {
							yPos += 10
						} else {
							yPos += 36
						}
						leaderText := leader.UserName
						bounds, _ = d.BoundString(leaderText)
						leaderWidth := bounds.Max.X.Round() - bounds.Min.X.Round()
						d.Dot = fixed.Point26_6{
							X: fixed.I((PaperWidth - leaderWidth) / 2),
							Y: fixed.I(yPos) + statsFace.Metrics().Ascent,
						}
						d.DrawString(leaderText)
						
						// Bits count
						yPos += 36
						bitsText := fmt.Sprintf("%d Bits", leader.Score)
						d.Src = image.Black
						bounds, _ = d.BoundString(bitsText)
						bitsWidth := bounds.Max.X.Round() - bounds.Min.X.Round()
						d.Dot = fixed.Point26_6{
							X: fixed.I((PaperWidth - bitsWidth) / 2),
							Y: fixed.I(yPos) + statsFace.Metrics().Ascent,
						}
						d.DrawString(bitsText)
						yPos += 36 + lineSpacing
					} else {
						// 2nd and 3rd place - smaller font, no avatar
						d.Face = smallFace
						d.Src = image.NewUniform(color.RGBA{100, 100, 100, 255})
						
						// Place and name
						placeText := fmt.Sprintf("%d位 %s", i+1, leader.UserName)
						bounds, _ = d.BoundString(placeText)
						placeWidth := bounds.Max.X.Round() - bounds.Min.X.Round()
						d.Dot = fixed.Point26_6{
							X: fixed.I((PaperWidth - placeWidth) / 2),
							Y: fixed.I(yPos) + smallFace.Metrics().Ascent,
						}
						d.DrawString(placeText)
						
						// Bits count
						yPos += 24
						bitsText := fmt.Sprintf("%d Bits", leader.Score)
						d.Src = image.NewUniform(color.RGBA{100, 100, 100, 255})
						bounds, _ = d.BoundString(bitsText)
						bitsWidth := bounds.Max.X.Round() - bounds.Min.X.Round()
						d.Dot = fixed.Point26_6{
							X: fixed.I((PaperWidth - bitsWidth) / 2),
							Y: fixed.I(yPos) + smallFace.Metrics().Ascent,
						}
						d.DrawString(bitsText)
						yPos += 24 + lineSpacing
					}
				}
			}
		}
	}
	
	// Draw decorative line
	lineY := imgHeight - 10
	for x := 10; x < PaperWidth-10; x += 4 {
		for y := 0; y < 2; y++ {
			img.Set(x, lineY+y, color.Black)
		}
	}
	
	return img, nil
}
