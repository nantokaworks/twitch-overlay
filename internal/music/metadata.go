package music

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"os"

	"github.com/dhowden/tag"
	"github.com/nantokaworks/twitch-overlay/internal/shared/logger"
	"go.uber.org/zap"
)

type Metadata struct {
	Title       string
	Artist      string
	Album       string
	Duration    int
	ArtworkData []byte
}

func ExtractMetadata(filePath string) (*Metadata, error) {
	// ファイルを開いてメタデータを読み取る
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file for metadata: %w", err)
	}
	defer file.Close()

	// tagライブラリでメタデータを読み取る
	m, err := tag.ReadFrom(file)
	if err != nil {
		logger.Warn("Failed to read metadata tags", zap.Error(err))
		return &Metadata{
			Title:  "Unknown Title",
			Artist: "Unknown Artist",
		}, nil
	}

	metadata := &Metadata{
		Title:  m.Title(),
		Artist: m.Artist(),
		Album:  m.Album(),
	}

	// デフォルト値の設定
	if metadata.Title == "" {
		metadata.Title = "Unknown Title"
	}
	if metadata.Artist == "" {
		metadata.Artist = "Unknown Artist"
	}

	// アートワークの抽出
	if pic := m.Picture(); pic != nil {
		// 画像データを取得
		artworkData := pic.Data
		
		// JPEG形式に変換（必要に応じて）
		if pic.MIMEType != "image/jpeg" {
			img, _, err := image.Decode(bytes.NewReader(pic.Data))
			if err == nil {
				var buf bytes.Buffer
				if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err == nil {
					artworkData = buf.Bytes()
				}
			}
		}
		
		metadata.ArtworkData = artworkData
		logger.Info("Artwork extracted",
			zap.String("title", metadata.Title),
			zap.Int("size", len(artworkData)))
	}

	// トラック番号や年などの追加メタデータも取得可能
	if trackNum, _ := m.Track(); trackNum > 0 {
		logger.Debug("Track number", zap.Int("track", trackNum))
	}

	return metadata, nil
}