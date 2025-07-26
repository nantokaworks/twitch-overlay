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

	"github.com/joeyak/go-twitch-eventsub/v3"
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

func MessageToImage(userName string, msg []twitch.ChatMessageFragment) (image.Image, error) {
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

	// 動的な高さ計算：ユーザー名行(アセント+デセント) + 各行
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
		// Emote-only 行（空白テキスト無 & emote数 ≤8）の場合、高さ cellW を追加
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

		// single-character text-only line: 高さも幅いっぱいに拡大
		if len(lines) == 1 && len(line) == 1 &&
			line[0].Emote == nil &&
			!urlRe.MatchString(line[0].Text) &&
			len([]rune(strings.TrimSpace(line[0].Text))) == 1 {
			text := strings.TrimSpace(line[0].Text)
			origW := int((&font.Drawer{Face: face}).MeasureString(text) >> 6)
			if origW > 0 {
				scale := float64(PaperWidth) / float64(origW)
				newSize := float64(fontSize) * scale
				// 新フェイスで行高さを取得
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

	// 画像生成
	img := image.NewRGBA(image.Rect(0, 0, PaperWidth, imgHeight))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)

	// Drawer準備
	d := &font.Drawer{Dst: img, Src: image.NewUniform(color.Black), Face: face}

	// 1行目: userName
	d.Dot = fixed.Point26_6{X: fixed.I(0), Y: fixed.I(ascent)}
	d.DrawString(userName)

	// 2行目以降: 折返し後の行を描画
	for i, line := range lines {
		y := (i+1)*lineHeight + ascent

		// 全て Emote または 空白テキストのみ かつ エモート数 ≤ 8 の場合
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
				draw.Draw(img,
					image.Rect(j*cellW, y-ascent, j*cellW+cellW, y-ascent+cellW),
					dst, image.Point{}, draw.Over)
			}
			continue
		}

		// single-character text-only line: 横幅いっぱいに中央揃え＋フォントサイズ拡大
		if len(line) == 1 &&
			line[0].Emote == nil &&
			!urlRe.MatchString(line[0].Text) &&
			len([]rune(strings.TrimSpace(line[0].Text))) == 1 {
			text := strings.TrimSpace(line[0].Text)
			// 現行フェイスでの幅を取得
			origW := int(d.MeasureString(text) >> 6)
			if origW > 0 {
				// 幅いっぱいに拡大するスケール
				scale := float64(PaperWidth) / float64(origW)
				newSize := float64(fontSize) * scale
				// 新フェイス生成
				face2, err := opentype.NewFace(f, &opentype.FaceOptions{
					Size:    newSize,
					DPI:     72,
					Hinting: font.HintingFull,
				})
				if err == nil {
					// 新フェイスの ascent を取得
					ascent2 := int(face2.Metrics().Ascent >> 6)
					d2 := &font.Drawer{Dst: img, Src: image.NewUniform(color.Black), Face: face2}
					w2 := int(d2.MeasureString(text) >> 6)
					x2 := (PaperWidth - w2) / 2
					// 元の y 位置から元フェイスの ascent を引き、新フェイスの ascent を足す
					d2.Dot = fixed.Point26_6{
						X: fixed.I(x2),
						Y: fixed.I(y - ascent + ascent2),
					}
					d2.DrawString(text)
				} else {
					// フォールバック
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
					// 画像を描画
					draw.Draw(img,
						image.Rect(0, y-ascent, PaperWidth, y-ascent+img0.Bounds().Dy()),
						img0, image.Point{}, draw.Over)
					// 画像の下に QR を描画
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
				draw.Draw(img,
					image.Rect(x, y-ascent, x+eimg.Bounds().Dx(), y-ascent+eimg.Bounds().Dy()),
					eimg, image.Point{}, draw.Over)
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
			draw.Draw(img,
				image.Rect(x0, underlineY, end, underlineY+UnderlineHeight),
				&image.Uniform{color.Black}, image.Point{}, draw.Src)
		}
	} else {
		draw.Draw(img,
			image.Rect(0, underlineY, PaperWidth, underlineY+UnderlineHeight),
			&image.Uniform{color.Black}, image.Point{}, draw.Src)
	}

	return img, nil
}

// generateTimeImage creates an image with the given time string
func generateTimeImage(timeStr string) (image.Image, error) {
	// Load font
	fontBytes, err := os.ReadFile("/Users/toka/Library/Fonts/HackGen-Bold.ttf")
	if err != nil {
		return nil, fmt.Errorf("failed to load font: %w", err)
	}
	
	f, err := opentype.Parse(fontBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse font: %w", err)
	}
	
	face, err := opentype.NewFace(f, &opentype.FaceOptions{
		Size:    48,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create font face: %w", err)
	}
	defer face.Close()
	
	// Measure text
	d := &font.Drawer{
		Face: face,
	}
	bounds, _ := d.BoundString(timeStr)
	textWidth := bounds.Max.X.Round() - bounds.Min.X.Round()
	textHeight := face.Metrics().Height.Round()
	
	// Create image with padding
	padding := 20
	imgHeight := textHeight + padding*2
	img := image.NewRGBA(image.Rect(0, 0, PaperWidth, imgHeight))
	
	// Fill white background
	draw.Draw(img, img.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)
	
	// Draw time centered
	d.Dst = img
	d.Src = image.Black
	d.Dot = fixed.Point26_6{
		X: fixed.I((PaperWidth - textWidth) / 2),
		Y: fixed.I(padding) + face.Metrics().Ascent,
	}
	d.DrawString(timeStr)
	
	// Draw decorative lines
	lineY := imgHeight - 10
	for x := 10; x < PaperWidth-10; x += 2 {
		img.Set(x, lineY, color.Black)
	}
	
	return img, nil
}

// generateTimeImageWithStats creates an image with time and Twitch channel statistics
func generateTimeImageWithStats(timeStr string) (image.Image, error) {
	// Import twitchapi
	viewers, followers, isLive, err := getTwitchStats()
	if err != nil {
		// If API fails, just print time
		return generateTimeImage(timeStr)
	}
	
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
	
	// Prepare text
	statusText := "オフライン"
	if isLive {
		statusText = "配信中"
	}
	statsLine1 := fmt.Sprintf("視聴者: %d人", viewers)
	statsLine2 := fmt.Sprintf("フォロワー: %d人", followers)
	
	// Create image
	padding := 20
	lineSpacing := 10
	imgHeight := padding*2 + 48 + lineSpacing*2 + 36*3 + 20
	img := image.NewRGBA(image.Rect(0, 0, PaperWidth, imgHeight))
	
	// Fill white background
	draw.Draw(img, img.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)
	
	// Draw time centered
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
	
	// Draw status
	d.Face = statsFace
	yPos := padding + 48 + lineSpacing
	
	bounds, _ = d.BoundString(statusText)
	statusWidth := bounds.Max.X.Round() - bounds.Min.X.Round()
	d.Dot = fixed.Point26_6{
		X: fixed.I((PaperWidth - statusWidth) / 2),
		Y: fixed.I(yPos) + statsFace.Metrics().Ascent,
	}
	d.DrawString(statusText)
	
	// Draw stats line 1
	yPos += 36 + lineSpacing
	bounds, _ = d.BoundString(statsLine1)
	stats1Width := bounds.Max.X.Round() - bounds.Min.X.Round()
	d.Dot = fixed.Point26_6{
		X: fixed.I((PaperWidth - stats1Width) / 2),
		Y: fixed.I(yPos) + statsFace.Metrics().Ascent,
	}
	d.DrawString(statsLine1)
	
	// Draw stats line 2
	yPos += 36 + lineSpacing
	bounds, _ = d.BoundString(statsLine2)
	stats2Width := bounds.Max.X.Round() - bounds.Min.X.Round()
	d.Dot = fixed.Point26_6{
		X: fixed.I((PaperWidth - stats2Width) / 2),
		Y: fixed.I(yPos) + statsFace.Metrics().Ascent,
	}
	d.DrawString(statsLine2)
	
	// Draw decorative line
	lineY := imgHeight - 10
	for x := 10; x < PaperWidth-10; x += 4 {
		for y := 0; y < 2; y++ {
			img.Set(x, lineY+y, color.Black)
		}
	}
	
	return img, nil
}

// getTwitchStats is a helper function to get Twitch statistics
func getTwitchStats() (viewers int, followers int, isLive bool, err error) {
	return twitchapi.GetChannelStats()
}
