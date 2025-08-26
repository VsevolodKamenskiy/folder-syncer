package syncer

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// Создаем временные директории и файлы для тестирования
func setupTestDirectories(srcDir, dstDir string) error {
	// Создаем исходную директорию
	if err := os.MkdirAll(srcDir, os.ModePerm); err != nil {
		return err
	}

	// Создаем целевую директорию
	if err := os.MkdirAll(dstDir, os.ModePerm); err != nil {
		return err
	}

	// Создаем тестовые файлы в исходной директории
	for i := 0; i < 10; i++ {
		content := make([]byte, 1024*1024) // 1 МБ
		if err := os.WriteFile(filepath.Join(srcDir, "file"+strconv.Itoa(i)+".txt"), content, 0644); err != nil {
			return err
		}
	}

	return nil
}

// Бенчмарк для syncDirectories
func BenchmarkSyncDirectories(b *testing.B) {
	srcDir := "./test_src"
	dstDir := "./test_dst"

	// Подготовка тестовых директорий
	if err := setupTestDirectories(srcDir, dstDir); err != nil {
		b.Fatalf("Failed to set up test directories: %v", err)
	}
	defer os.RemoveAll(srcDir)
	defer os.RemoveAll(dstDir)
	defer os.Remove("log.txt")

	// Создаем экземпляр Syncer
	syncer := NewSyncer(srcDir, dstDir)

	// Запускаем бенчмарк
	for i := 0; i < b.N; i++ {
		syncer.syncDirectories()
	}

}

// Бенчмарк для syncFile
func BenchmarkSyncFile(b *testing.B) {
	srcFile := "file0.txt"
	dstFile := "file1.txt"

	// Подготовка тестовых файлов
	content := make([]byte, 10*1024*1024) // 10 МБ

	if err := os.WriteFile(srcFile, content, 0644); err != nil {
		b.Fatalf("Failed to create source file: %v", err)
	}
	defer os.Remove(srcFile)
	defer os.Remove(dstFile)
	defer os.Remove("log.txt")

	// Создаем экземпляр Syncer
	syncer := NewSyncer("./test_src", "./test_dst")

	// Запускаем бенчмарк
	for i := 0; i < b.N; i++ {
		// Вызываем syncFile прямо
		if err := syncer.syncFile(srcFile, dstFile); err != nil {
			b.Fatalf("Failed to sync file: %v", err)
		}
	}
}
