package fontmanager

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/nantokaworks/twitch-overlay/internal/shared/logger"
	"go.uber.org/zap"
	"golang.org/x/image/font/opentype"
)

const (
	// フォントを保存するディレクトリ
	FontDirectory = "./uploads/fonts"
	
	// 最大ファイルサイズ (50MB)
	MaxFileSize = 50 * 1024 * 1024
)

var (
	mu             sync.RWMutex
	customFontPath string
	fontCache      *opentype.Font
	
	// エラー定義
	ErrInvalidFormat = errors.New("invalid font format")
	ErrFileTooLarge  = errors.New("file too large")
	ErrNoCustomFont  = errors.New("no custom font configured")
)

// Initialize はフォントマネージャーを初期化します
func Initialize() error {
	// フォントディレクトリの作成
	if err := os.MkdirAll(FontDirectory, 0755); err != nil {
		return fmt.Errorf("failed to create font directory: %w", err)
	}
	
	// データベースから現在の設定を読み込み
	path, err := loadCustomFontPath()
	if err == nil && path != "" {
		customFontPath = path
		logger.Info("Custom font loaded from database", zap.String("path", path))
		
		// フォントファイルをキャッシュに読み込み
		if err := loadFontToCache(path); err != nil {
			logger.Error("Failed to load custom font to cache", zap.Error(err))
			// キャッシュの読み込みに失敗してもエラーにはしない
		}
	}
	
	return nil
}

// loadFontToCache はフォントファイルをキャッシュに読み込みます
func loadFontToCache(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read font file: %w", err)
	}
	
	font, err := opentype.Parse(data)
	if err != nil {
		return fmt.Errorf("failed to parse font: %w", err)
	}
	
	fontCache = font
	return nil
}

// GetFont は現在のフォントデータを返します
// カスタムフォントが設定されていない場合はnilを返します
func GetFont(defaultFontData []byte) ([]byte, error) {
	mu.RLock()
	defer mu.RUnlock()
	
	// カスタムフォントが設定されていない場合
	if customFontPath == "" {
		return nil, fmt.Errorf("no custom font configured: please upload a font file (TTF/OTF) via the settings page")
	}
	
	// カスタムフォントを読み込み
	data, err := os.ReadFile(customFontPath)
	if err != nil {
		logger.Error("Failed to read custom font", 
			zap.String("path", customFontPath),
			zap.Error(err))
		return nil, fmt.Errorf("failed to read custom font file: %w", err)
	}
	
	return data, nil
}

// GetParsedFont はパース済みのフォントを返します（キャッシュ利用）
func GetParsedFont(defaultFontData []byte) (*opentype.Font, error) {
	mu.RLock()
	defer mu.RUnlock()
	
	// カスタムフォントが設定されていない場合
	if customFontPath == "" || fontCache == nil {
		return opentype.Parse(defaultFontData)
	}
	
	return fontCache, nil
}

// SaveCustomFont はアップロードされたフォントを保存します
func SaveCustomFont(filename string, data io.Reader, size int64) error {
	// サイズチェック
	if size > MaxFileSize {
		return ErrFileTooLarge
	}
	
	// 拡張子チェック
	ext := filepath.Ext(filename)
	if ext != ".ttf" && ext != ".otf" && ext != ".TTF" && ext != ".OTF" {
		return ErrInvalidFormat
	}
	
	// 一時ファイルに書き込み
	tempFile := filepath.Join(FontDirectory, "temp_"+filename)
	file, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile) // 成功/失敗に関わらず一時ファイルは削除
	
	// データをコピー
	written, err := io.CopyN(file, data, MaxFileSize+1)
	file.Close()
	
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to write font data: %w", err)
	}
	
	if written > MaxFileSize {
		return ErrFileTooLarge
	}
	
	// フォントとして検証
	fontData, err := os.ReadFile(tempFile)
	if err != nil {
		return fmt.Errorf("failed to read temp file: %w", err)
	}
	
	font, err := opentype.Parse(fontData)
	if err != nil {
		return ErrInvalidFormat
	}
	
	// 正式なファイル名で保存
	finalPath := filepath.Join(FontDirectory, filename)
	
	mu.Lock()
	defer mu.Unlock()
	
	// 既存のカスタムフォントを削除
	if customFontPath != "" && customFontPath != finalPath {
		os.Remove(customFontPath)
	}
	
	// ファイルを移動
	if err := os.Rename(tempFile, finalPath); err != nil {
		// Renameが失敗した場合はコピー
		if err := os.WriteFile(finalPath, fontData, 0644); err != nil {
			return fmt.Errorf("failed to save font file: %w", err)
		}
	}
	
	// フォントパスを記録
	
	// 更新
	customFontPath = finalPath
	fontCache = font
	
	logger.Info("Custom font saved successfully", 
		zap.String("filename", filename),
		zap.String("path", finalPath))
	
	return nil
}

// DeleteCustomFont はカスタムフォントを削除します
func DeleteCustomFont() error {
	mu.Lock()
	defer mu.Unlock()
	
	if customFontPath == "" {
		return ErrNoCustomFont
	}
	
	// ファイルを削除
	if err := os.Remove(customFontPath); err != nil && !os.IsNotExist(err) {
		logger.Error("Failed to delete font file", zap.Error(err))
	}
	
	// フォントパスをクリア
	
	// リセット
	customFontPath = ""
	fontCache = nil
	
	logger.Info("Custom font deleted successfully")
	
	return nil
}

// GetCurrentFontInfo は現在のフォント情報を返します
func GetCurrentFontInfo() map[string]interface{} {
	mu.RLock()
	defer mu.RUnlock()
	
	info := map[string]interface{}{
		"hasCustomFont": customFontPath != "",
		"path":          customFontPath, // main.goの確認用
	}
	
	if customFontPath != "" {
		info["filename"] = filepath.Base(customFontPath)
		
		// ファイルサイズを取得
		if stat, err := os.Stat(customFontPath); err == nil {
			info["fileSize"] = stat.Size()
			info["modifiedAt"] = stat.ModTime().Format("2006-01-02 15:04:05")
		}
	}
	
	return info
}

// loadCustomFontPath はフォントディレクトリから既存のフォントを探します
func loadCustomFontPath() (string, error) {
	// uploads/fontsディレクトリから最初のフォントファイルを探す
	files, err := os.ReadDir(FontDirectory)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	
	for _, file := range files {
		if !file.IsDir() {
			ext := filepath.Ext(file.Name())
			if ext == ".ttf" || ext == ".otf" || ext == ".TTF" || ext == ".OTF" {
				return filepath.Join(FontDirectory, file.Name()), nil
			}
		}
	}
	
	return "", nil
}